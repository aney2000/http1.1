package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/uig27055/http1.1/internal/handler"
	"github.com/uig27055/http1.1/internal/request"
	"github.com/uig27055/http1.1/internal/response"
	"github.com/uig27055/http1.1/internal/status"
)

// echo replies with method, path and the request body.
var echo = handler.Func(func(w response.Writer, r *request.Request) {
	body, _ := io.ReadAll(r.Body)
	response.Text(w, status.OK, r.Method+" "+r.Path+" "+string(body))
})

func startServer(t *testing.T, h handler.Handler, configure func(*Server)) (*Server, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := New(h)
	srv.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	if configure != nil {
		configure(srv)
	}

	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		if err := <-done; !errors.Is(err, ErrServerClosed) {
			t.Errorf("Serve() = %v, want ErrServerClosed", err)
		}
	})
	return srv, ln.Addr().String()
}

type client struct {
	t    *testing.T
	conn net.Conn
	r    *bufio.Reader
}

func dial(t *testing.T, addr string) *client {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	return &client{t: t, conn: conn, r: bufio.NewReader(conn)}
}

func (c *client) send(raw string) {
	c.t.Helper()
	if _, err := io.WriteString(c.conn, raw); err != nil {
		c.t.Fatalf("write: %v", err)
	}
}

// reply is a fully consumed response, decoded by net/http as an
// independent reference client.
type reply struct {
	StatusCode    int
	Header        http.Header
	ContentLength int64
	Close         bool // net/http moves "Connection: close" here
}

// receive reads one response; method is needed to know if a body follows.
func (c *client) receive(method string) (reply, string) {
	c.t.Helper()
	resp, err := http.ReadResponse(c.r, &http.Request{Method: method})
	if err != nil {
		c.t.Fatalf("read response: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("read body: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		c.t.Fatalf("close body: %v", err)
	}
	return reply{resp.StatusCode, resp.Header, resp.ContentLength, resp.Close}, string(body)
}

func (c *client) expectClosed() {
	c.t.Helper()
	if _, err := c.r.ReadByte(); !errors.Is(err, io.EOF) {
		c.t.Fatalf("expected connection close, got err = %v", err)
	}
}

func TestServer_ServesSimpleRequest(t *testing.T) {
	_, addr := startServer(t, echo, nil)
	c := dial(t, addr)

	c.send("GET /hello HTTP/1.1\r\nHost: x\r\n\r\n")
	resp, body := c.receive("GET")

	if resp.StatusCode != 200 || body != "GET /hello " {
		t.Fatalf("got %d %q", resp.StatusCode, body)
	}
	if resp.Header.Get("Date") == "" {
		t.Fatal("missing Date header")
	}
}

func TestServer_KeepAliveServesMultipleRequests(t *testing.T) {
	_, addr := startServer(t, echo, nil)
	c := dial(t, addr)

	for _, path := range []string{"/one", "/two", "/three"} {
		c.send("GET " + path + " HTTP/1.1\r\nHost: x\r\n\r\n")
		if _, body := c.receive("GET"); body != "GET "+path+" " {
			t.Fatalf("body = %q", body)
		}
	}
}

func TestServer_PipelinedRequests(t *testing.T) {
	_, addr := startServer(t, echo, nil)
	c := dial(t, addr)

	c.send("POST /a HTTP/1.1\r\nHost: x\r\nContent-Length: 3\r\n\r\nabc" +
		"GET /b HTTP/1.1\r\nHost: x\r\n\r\n")

	if _, body := c.receive("POST"); body != "POST /a abc" {
		t.Fatalf("first body = %q", body)
	}
	if _, body := c.receive("GET"); body != "GET /b " {
		t.Fatalf("second body = %q", body)
	}
}

func TestServer_UnreadBodyIsDrainedBeforeNextRequest(t *testing.T) {
	ignoreBody := handler.Func(func(w response.Writer, r *request.Request) {
		response.Text(w, status.OK, r.Path)
	})
	_, addr := startServer(t, ignoreBody, nil)
	c := dial(t, addr)

	c.send("POST /a HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\n3\r\nabc\r\n0\r\n\r\n")
	c.receive("POST")
	c.send("GET /b HTTP/1.1\r\nHost: x\r\n\r\n")

	if _, body := c.receive("GET"); body != "/b" {
		t.Fatalf("body = %q, want /b", body)
	}
}

func TestServer_ConnectionCloseIsHonoured(t *testing.T) {
	_, addr := startServer(t, echo, nil)
	c := dial(t, addr)

	c.send("GET / HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n")
	resp, _ := c.receive("GET")

	if !resp.Close {
		t.Fatal("response did not announce Connection: close")
	}
	c.expectClosed()
}

func TestServer_HTTP10ClosesByDefault(t *testing.T) {
	_, addr := startServer(t, echo, nil)
	c := dial(t, addr)

	c.send("GET / HTTP/1.0\r\n\r\n")
	c.receive("GET")
	c.expectClosed()
}

func TestServer_HTTP10KeepAliveOptIn(t *testing.T) {
	_, addr := startServer(t, echo, nil)
	c := dial(t, addr)

	c.send("GET /a HTTP/1.0\r\nConnection: keep-alive\r\n\r\n")
	resp, _ := c.receive("GET")
	if !strings.EqualFold(resp.Header.Get("Connection"), "keep-alive") {
		t.Fatalf("Connection = %q, want keep-alive", resp.Header.Get("Connection"))
	}

	c.send("GET /b HTTP/1.0\r\n\r\n")
	if _, body := c.receive("GET"); body != "GET /b " {
		t.Fatalf("body = %q", body)
	}
}

func TestServer_HeadResponseHasNoBody(t *testing.T) {
	_, addr := startServer(t, echo, nil)
	c := dial(t, addr)

	c.send("HEAD /x HTTP/1.1\r\nHost: x\r\n\r\n")
	resp, body := c.receive("HEAD")

	if body != "" || resp.ContentLength != int64(len("HEAD /x ")) {
		t.Fatalf("body = %q, Content-Length = %d", body, resp.ContentLength)
	}
	// The connection must still be usable: no stray body bytes on the wire.
	c.send("GET /y HTTP/1.1\r\nHost: x\r\n\r\n")
	if _, body := c.receive("GET"); body != "GET /y " {
		t.Fatalf("follow-up body = %q", body)
	}
}

func TestServer_ExpectContinue(t *testing.T) {
	_, addr := startServer(t, echo, nil)
	c := dial(t, addr)

	c.send("POST /up HTTP/1.1\r\nHost: x\r\nContent-Length: 2\r\nExpect: 100-continue\r\n\r\n")
	interim, err := c.r.ReadString('\n')
	if err != nil || interim != "HTTP/1.1 100 Continue\r\n" {
		t.Fatalf("interim = %q, err = %v", interim, err)
	}
	if blank, _ := c.r.ReadString('\n'); blank != "\r\n" {
		t.Fatalf("interim terminator = %q", blank)
	}

	c.send("hi")
	if _, body := c.receive("POST"); body != "POST /up hi" {
		t.Fatalf("body = %q", body)
	}
}

func TestServer_ParseErrorsMapToStatusAndClose(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{"malformed request line", "BROKEN\r\n\r\n", 400},
		{"missing host", "GET / HTTP/1.1\r\n\r\n", 400},
		{"unsupported version", "GET / HTTP/2.0\r\nHost: x\r\n\r\n", 505},
		{"unsupported encoding", "POST / HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: gzip\r\n\r\n", 501},
		{"body too large", "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 999\r\n\r\n", 413},
		{"header line too long", "GET / HTTP/1.1\r\nHost: x\r\nX-Big: " + strings.Repeat("a", 200) + "\r\n\r\n", 431},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, addr := startServer(t, echo, func(s *Server) {
				s.Limits.MaxBodyBytes = 100
				s.Limits.MaxLineBytes = 128
			})
			c := dial(t, addr)

			c.send(tt.raw)
			resp, _ := c.receive("GET")

			if resp.StatusCode != tt.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.want)
			}
			c.expectClosed()
		})
	}
}

func TestServer_IdleTimeoutClosesConnection(t *testing.T) {
	_, addr := startServer(t, echo, func(s *Server) { s.IdleTimeout = 50 * time.Millisecond })
	c := dial(t, addr)

	c.send("GET / HTTP/1.1\r\nHost: x\r\n\r\n")
	c.receive("GET")
	c.expectClosed()
}

func TestServer_SlowHeadersGet408(t *testing.T) {
	_, addr := startServer(t, echo, func(s *Server) { s.ReadTimeout = 50 * time.Millisecond })
	c := dial(t, addr)

	c.send("GET / HTTP/1.1\r\n")
	resp, _ := c.receive("GET")

	if resp.StatusCode != 408 {
		t.Fatalf("status = %d, want 408", resp.StatusCode)
	}
	c.expectClosed()
}

func TestServer_ShutdownWaitsForInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	slow := handler.Func(func(w response.Writer, _ *request.Request) {
		close(started)
		<-release
		response.Text(w, status.OK, "done")
	})
	srv, addr := startServer(t, slow, nil)
	c := dial(t, addr)

	c.send("GET / HTTP/1.1\r\nHost: x\r\n\r\n")
	<-started

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- srv.Shutdown(context.Background()) }()

	select {
	case <-shutdownDone:
		t.Fatal("Shutdown returned before in-flight request finished")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	resp, body := c.receive("GET")
	if body != "done" || !resp.Close {
		t.Fatalf("body = %q, Close = %v; want done, true", body, resp.Close)
	}
	if err := <-shutdownDone; err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}
}

func TestServer_ShutdownClosesIdleConnections(t *testing.T) {
	srv, addr := startServer(t, echo, nil)
	c := dial(t, addr)

	c.send("GET / HTTP/1.1\r\nHost: x\r\n\r\n")
	c.receive("GET")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}
	c.expectClosed()
}

func TestServer_ShutdownHonoursContextDeadline(t *testing.T) {
	block := make(chan struct{})
	stuck := handler.Func(func(response.Writer, *request.Request) { <-block })
	srv, addr := startServer(t, stuck, nil)
	c := dial(t, addr)
	t.Cleanup(func() { close(block) }) // runs first (LIFO), unblocking the handler
	c.send("GET / HTTP/1.1\r\nHost: x\r\n\r\n")
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := srv.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown() = %v, want DeadlineExceeded", err)
	}
}

func TestServer_ServeAfterShutdownFails(t *testing.T) {
	srv := New(echo)
	_ = srv.Shutdown(context.Background())

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Serve(ln); !errors.Is(err, ErrServerClosed) {
		t.Fatalf("Serve() = %v, want ErrServerClosed", err)
	}
}
