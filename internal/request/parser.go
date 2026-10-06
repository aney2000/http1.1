package request

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/uig27055/http1.1/internal/headers"
)

// Limits bounds resource usage while parsing, protecting the server from
// oversized or malicious input.
type Limits struct {
	MaxLineBytes   int
	MaxHeaderCount int
	MaxBodyBytes   int64
}

// DefaultLimits returns conservative production defaults.
func DefaultLimits() Limits {
	return Limits{
		MaxLineBytes:   8 * 1024,
		MaxHeaderCount: 100,
		MaxBodyBytes:   10 * 1024 * 1024,
	}
}

// Parser reads HTTP requests from a buffered stream.
type Parser struct {
	limits Limits
}

// NewParser creates a parser with the given limits.
func NewParser(limits Limits) *Parser {
	return &Parser{limits: limits}
}

// Parse reads exactly one request from r. It returns io.EOF when the stream
// ends cleanly before any byte of a new request, which signals a closed
// keep-alive connection rather than an error.
func (p *Parser) Parse(r *bufio.Reader) (*Request, error) {
	line, err := p.readLine(r)
	if err != nil {
		return nil, err
	}

	req, err := parseRequestLine(line)
	if err != nil {
		return nil, err
	}

	if req.Headers, err = p.readHeaders(r); err != nil {
		return nil, err
	}
	if err := validateHost(req); err != nil {
		return nil, err
	}

	req.Body = strings.NewReader("")
	return req, nil
}

func parseRequestLine(line string) (*Request, error) {
	parts := strings.Split(line, " ")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: %q", ErrMalformedRequestLine, line)
	}
	method, target, version := parts[0], parts[1], parts[2]

	if !isValidMethod(method) || !isValidTarget(target) || !strings.HasPrefix(version, "HTTP/") {
		return nil, fmt.Errorf("%w: %q", ErrMalformedRequestLine, line)
	}
	if version != "HTTP/1.1" && version != "HTTP/1.0" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedVersion, version)
	}

	path, query, _ := strings.Cut(target, "?")
	return &Request{
		Method:   method,
		Target:   target,
		Path:     path,
		RawQuery: query,
		Version:  version,
	}, nil
}

func (p *Parser) readHeaders(r *bufio.Reader) (*headers.Headers, error) {
	h := headers.New()
	for {
		line, err := p.readLine(r)
		if err != nil {
			return nil, unexpectedEOF(err)
		}
		if line == "" {
			return h, nil
		}
		if h.Len() >= p.limits.MaxHeaderCount {
			return nil, ErrTooManyHeaders
		}

		name, value, err := headers.ParseLine(line)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrMalformedHeader, err)
		}
		h.Add(name, value)
	}
}

// validateHost enforces RFC 9112 §3.2: HTTP/1.1 requires exactly one Host.
func validateHost(req *Request) error {
	hosts := req.Headers.Values("Host")
	if len(hosts) > 1 {
		return fmt.Errorf("%w: multiple Host headers", ErrMalformedHeader)
	}
	if len(hosts) == 0 && !req.Is10() {
		return ErrMissingHost
	}
	return nil
}

// readLine reads one line terminated by CRLF (or bare LF, accepted for
// robustness per RFC 9112 §2.2) and enforces the line length limit.
func (p *Parser) readLine(r *bufio.Reader) (string, error) {
	var sb strings.Builder
	for {
		chunk, err := r.ReadSlice('\n')
		if sb.Len()+len(chunk) > p.limits.MaxLineBytes {
			return "", ErrLineTooLong
		}
		sb.Write(chunk)

		switch {
		case err == nil:
			return strings.TrimSuffix(strings.TrimSuffix(sb.String(), "\n"), "\r"), nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && sb.Len() > 0:
			return "", io.ErrUnexpectedEOF
		default:
			return "", err
		}
	}
}

func unexpectedEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}

func isValidMethod(m string) bool {
	if m == "" {
		return false
	}
	for i := 0; i < len(m); i++ {
		c := m[i]
		if (c < 'A' || c > 'Z') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

func isValidTarget(t string) bool {
	return t == "*" || strings.HasPrefix(t, "/")
}
