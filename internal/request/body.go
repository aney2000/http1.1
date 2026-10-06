package request

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// attachBody selects the body reader according to RFC 9112 §6.3.
// Rejecting requests with both Transfer-Encoding and Content-Length closes
// the classic request-smuggling hole between proxies and origin servers.
func (p *Parser) attachBody(req *Request, r *bufio.Reader) error {
	te := req.Headers.Values("Transfer-Encoding")
	cl := req.Headers.Values("Content-Length")

	switch {
	case len(te) > 0 && len(cl) > 0:
		return ErrAmbiguousFraming
	case len(te) > 0:
		return p.attachChunked(req, r, te)
	case len(cl) > 0:
		return p.attachFixed(req, r, cl)
	default:
		req.ContentLength = 0
		req.Body = strings.NewReader("")
		return nil
	}
}

func (p *Parser) attachChunked(req *Request, r *bufio.Reader, te []string) error {
	if len(te) != 1 || !strings.EqualFold(te[0], "chunked") {
		return fmt.Errorf("%w: %v", ErrUnsupportedTransferEncoding, te)
	}
	req.ContentLength = -1
	req.Body = &maxBytesReader{
		r:         newChunkedReader(r, p.limits.MaxLineBytes),
		remaining: p.limits.MaxBodyBytes,
	}
	return nil
}

func (p *Parser) attachFixed(req *Request, r *bufio.Reader, cl []string) error {
	length, err := parseContentLength(cl)
	if err != nil {
		return err
	}
	if length > p.limits.MaxBodyBytes {
		return ErrBodyTooLarge
	}
	req.ContentLength = length
	req.Body = &fixedLengthReader{r: r, remaining: length}
	return nil
}

// parseContentLength accepts repeated values only if they are identical.
func parseContentLength(values []string) (int64, error) {
	for _, v := range values[1:] {
		if v != values[0] {
			return 0, fmt.Errorf("%w: conflicting values %v", ErrInvalidContentLength, values)
		}
	}
	n, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidContentLength, values[0])
	}
	return n, nil
}

// fixedLengthReader reads exactly `remaining` bytes and reports a truncated
// body as io.ErrUnexpectedEOF instead of silently succeeding.
type fixedLengthReader struct {
	r         io.Reader
	remaining int64
}

func (f *fixedLengthReader) Read(p []byte) (int, error) {
	if f.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > f.remaining {
		p = p[:f.remaining]
	}
	n, err := f.r.Read(p)
	f.remaining -= int64(n)
	if err == io.EOF && f.remaining > 0 {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}

// maxBytesReader fails with ErrBodyTooLarge once more than `remaining`
// bytes have been produced. Used for bodies of unknown size.
type maxBytesReader struct {
	r         io.Reader
	remaining int64
}

func (m *maxBytesReader) Read(p []byte) (int, error) {
	n, err := m.r.Read(p)
	m.remaining -= int64(n)
	if m.remaining < 0 {
		return n, ErrBodyTooLarge
	}
	return n, err
}
