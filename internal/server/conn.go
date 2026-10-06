package server

import (
	"bufio"
	"io"
	"net"
	"strings"
	"time"

	"github.com/uig27055/http1.1/internal/request"
	"github.com/uig27055/http1.1/internal/response"
)

// conn serves the sequence of requests arriving on one TCP connection.
type conn struct {
	srv     *Server
	netConn net.Conn
	reader  *bufio.Reader
	parser  *request.Parser
}

func newConn(s *Server, netConn net.Conn, parser *request.Parser) *conn {
	return &conn{srv: s, netConn: netConn, reader: bufio.NewReader(netConn), parser: parser}
}

func (c *conn) serve() {
	defer c.close()
	defer c.recoverPanic()

	for c.awaitRequest() && c.serveRequest() {
	}
}

// awaitRequest waits up to IdleTimeout for the first byte of a request.
// While waiting the connection is idle and may be closed by Shutdown.
func (c *conn) awaitRequest() bool {
	if !c.srv.setState(c, stateIdle) {
		return false
	}
	_ = c.netConn.SetReadDeadline(time.Now().Add(c.srv.IdleTimeout))
	if _, err := c.reader.Peek(1); err != nil {
		return false
	}
	return c.srv.setState(c, stateActive)
}

// serveRequest handles one request and reports whether to keep the
// connection open for another.
func (c *conn) serveRequest() bool {
	_ = c.netConn.SetReadDeadline(time.Now().Add(c.srv.ReadTimeout))
	req, err := c.parser.Parse(c.reader)
	if err != nil {
		c.writeParseError(err)
		return false
	}

	expect := c.wrapExpectContinue(req)
	w := response.NewBuffered(c.netConn, response.Options{OmitBody: req.Method == "HEAD"})
	c.srv.Handler.ServeHTTP(w, req)

	keepAlive := wantsKeepAlive(req) && c.consumeBody(req, expect) && !c.srv.isClosing()
	setConnectionHeader(w, req, keepAlive)

	_ = c.netConn.SetWriteDeadline(time.Now().Add(c.srv.WriteTimeout))
	return w.Finish() == nil && keepAlive
}

// consumeBody discards any body the handler left unread so the next
// request starts at the right byte. It reports whether the stream is still
// in sync. If the client is waiting for "100 Continue" that was never sent,
// the body will never arrive and the connection must be closed.
func (c *conn) consumeBody(req *request.Request, expect *expectContinueReader) bool {
	if expect != nil && !expect.sent {
		return false
	}
	_, err := io.Copy(io.Discard, req.Body)
	return err == nil
}

func (c *conn) wrapExpectContinue(req *request.Request) *expectContinueReader {
	if req.Is10() || req.ContentLength == 0 ||
		!strings.EqualFold(req.Headers.Get("Expect"), "100-continue") {
		return nil
	}
	expect := &expectContinueReader{body: req.Body, out: c.netConn}
	req.Body = expect
	return expect
}

func (c *conn) writeParseError(err error) {
	code, ok := statusForParseError(err)
	if !ok {
		return
	}
	w := response.NewBuffered(c.netConn, response.Options{})
	w.Header().Set("Connection", "close")
	response.Error(w, code)
	_ = c.netConn.SetWriteDeadline(time.Now().Add(c.srv.WriteTimeout))
	_ = w.Finish()
}

// recoverPanic is a last line of defence; the Recover middleware normally
// handles handler panics with a proper 500 response.
func (c *conn) recoverPanic() {
	if v := recover(); v != nil {
		c.srv.Logger.Error("connection panic", "remote", c.netConn.RemoteAddr().String(), "panic", v)
	}
}

func (c *conn) close() {
	_ = c.netConn.Close()
	c.srv.untrackConn(c)
}

// wantsKeepAlive applies RFC 9112 §9.3: HTTP/1.1 is persistent unless
// "close" is sent; HTTP/1.0 is persistent only with "keep-alive".
func wantsKeepAlive(req *request.Request) bool {
	tokens := connectionTokens(req)
	if tokens["close"] {
		return false
	}
	return !req.Is10() || tokens["keep-alive"]
}

func setConnectionHeader(w response.Writer, req *request.Request, keepAlive bool) {
	switch {
	case !keepAlive:
		w.Header().Set("Connection", "close")
	case req.Is10():
		w.Header().Set("Connection", "keep-alive")
	}
}

func connectionTokens(req *request.Request) map[string]bool {
	tokens := map[string]bool{}
	for _, value := range req.Headers.Values("Connection") {
		for _, token := range strings.Split(value, ",") {
			tokens[strings.ToLower(strings.TrimSpace(token))] = true
		}
	}
	return tokens
}

// expectContinueReader sends the interim "100 Continue" response the first
// time the handler reads the body (RFC 9110 §10.1.1).
type expectContinueReader struct {
	body io.Reader
	out  io.Writer
	sent bool
}

func (e *expectContinueReader) Read(p []byte) (int, error) {
	if !e.sent {
		e.sent = true
		if _, err := io.WriteString(e.out, "HTTP/1.1 100 Continue\r\n\r\n"); err != nil {
			return 0, err
		}
	}
	return e.body.Read(p)
}
