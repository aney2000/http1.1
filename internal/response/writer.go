// Package response builds and serializes HTTP/1.1 responses.
package response

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/uig27055/http1.1/internal/headers"
	"github.com/uig27055/http1.1/internal/status"
)

// httpDateFormat is the IMF-fixdate format mandated by RFC 9110 §5.6.7.
const httpDateFormat = "Mon, 02 Jan 2006 15:04:05 GMT"

// ErrAlreadyFinished is returned when writing to a completed response.
var ErrAlreadyFinished = errors.New("response already finished")

// Writer is the interface handlers use to build a response. It is kept
// deliberately small (Interface Segregation) so handlers can be tested
// against any fake implementation.
type Writer interface {
	Header() *headers.Headers
	WriteHeader(code status.Code)
	Write(p []byte) (int, error)
	Status() status.Code
}

// Options customizes a Buffered writer.
type Options struct {
	// OmitBody suppresses the body on the wire, used for HEAD requests.
	OmitBody bool
	// Now supplies the Date header clock; injectable for deterministic tests.
	Now func() time.Time
}

// Buffered collects the whole body in memory and emits it on Finish.
// Buffering lets the server always send an exact Content-Length, which keeps
// persistent connections simple and correct.
type Buffered struct {
	out      io.Writer
	opts     Options
	header   *headers.Headers
	body     bytes.Buffer
	code     status.Code
	finished bool
}

// NewBuffered creates a writer that serializes to out.
func NewBuffered(out io.Writer, opts Options) *Buffered {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Buffered{out: out, opts: opts, header: headers.New()}
}

// Header returns the mutable response headers.
func (b *Buffered) Header() *headers.Headers { return b.header }

// WriteHeader sets the status code. Only the first call has effect.
func (b *Buffered) WriteHeader(code status.Code) {
	if b.code == 0 {
		b.code = code
	}
}

// Status returns the effective status code (200 if never set).
func (b *Buffered) Status() status.Code {
	if b.code == 0 {
		return status.OK
	}
	return b.code
}

// Write appends to the response body.
func (b *Buffered) Write(p []byte) (int, error) {
	if b.finished {
		return 0, ErrAlreadyFinished
	}
	return b.body.Write(p)
}

// Finish writes status line, headers and body to the underlying writer.
func (b *Buffered) Finish() error {
	if b.finished {
		return ErrAlreadyFinished
	}
	b.finished = true

	code := b.Status()
	b.prepareHeaders(code)

	var msg bytes.Buffer
	msg.WriteString(code.StatusLine())
	if _, err := b.header.WriteTo(&msg); err != nil {
		return err
	}
	msg.WriteString("\r\n")
	if code.AllowsBody() && !b.opts.OmitBody {
		msg.Write(b.body.Bytes())
	}

	if _, err := b.out.Write(msg.Bytes()); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	return nil
}

func (b *Buffered) prepareHeaders(code status.Code) {
	if !b.header.Has("Date") {
		b.header.Set("Date", b.opts.Now().UTC().Format(httpDateFormat))
	}
	if code.AllowsBody() {
		b.header.Set("Content-Length", strconv.Itoa(b.body.Len()))
	} else {
		b.header.Del("Content-Length")
	}
}
