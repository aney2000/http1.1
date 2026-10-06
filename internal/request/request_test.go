package request

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

func parse(raw string) (*Request, error) {
	return NewParser(DefaultLimits()).Parse(bufio.NewReader(strings.NewReader(raw)))
}

func mustParse(t *testing.T, raw string) *Request {
	t.Helper()
	req, err := parse(raw)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	return req
}

func TestParse_RequestLine(t *testing.T) {
	req := mustParse(t, "GET /hello/world?name=go&x=1 HTTP/1.1\r\nHost: localhost\r\n\r\n")

	if req.Method != "GET" {
		t.Errorf("Method = %q", req.Method)
	}
	if req.Target != "/hello/world?name=go&x=1" {
		t.Errorf("Target = %q", req.Target)
	}
	if req.Path != "/hello/world" {
		t.Errorf("Path = %q", req.Path)
	}
	if req.RawQuery != "name=go&x=1" {
		t.Errorf("RawQuery = %q", req.RawQuery)
	}
	if req.Version != "HTTP/1.1" {
		t.Errorf("Version = %q", req.Version)
	}
}

func TestParse_QueryIsDecoded(t *testing.T) {
	req := mustParse(t, "GET /?name=hello%20world&tag=a&tag=b HTTP/1.1\r\nHost: x\r\n\r\n")

	q := req.Query()
	if q.Get("name") != "hello world" {
		t.Errorf("name = %q", q.Get("name"))
	}
	if len(q["tag"]) != 2 {
		t.Errorf("tag = %v", q["tag"])
	}
}

func TestParse_Headers(t *testing.T) {
	req := mustParse(t, "GET / HTTP/1.1\r\n"+
		"Host: example.com\r\n"+
		"user-agent: test\r\n"+
		"Accept: a\r\n"+
		"Accept: b\r\n"+
		"\r\n")

	if req.Headers.Get("Host") != "example.com" {
		t.Errorf("Host = %q", req.Headers.Get("Host"))
	}
	if req.Headers.Get("User-Agent") != "test" {
		t.Errorf("User-Agent = %q", req.Headers.Get("User-Agent"))
	}
	if len(req.Headers.Values("Accept")) != 2 {
		t.Errorf("Accept = %v", req.Headers.Values("Accept"))
	}
}

func TestParse_AcceptsBareLF(t *testing.T) {
	req := mustParse(t, "GET / HTTP/1.1\nHost: x\n\n")
	if req.Headers.Get("Host") != "x" {
		t.Fatalf("Host = %q", req.Headers.Get("Host"))
	}
}

func TestParse_HTTP10DoesNotRequireHost(t *testing.T) {
	req := mustParse(t, "GET / HTTP/1.0\r\n\r\n")
	if req.Version != "HTTP/1.0" {
		t.Fatalf("Version = %q", req.Version)
	}
}

func TestParse_AsteriskTarget(t *testing.T) {
	req := mustParse(t, "OPTIONS * HTTP/1.1\r\nHost: x\r\n\r\n")
	if req.Path != "*" {
		t.Fatalf("Path = %q", req.Path)
	}
}

func TestParse_CleanEOFBeforeRequestReturnsEOF(t *testing.T) {
	_, err := parse("")
	if !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want io.EOF", err)
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want error
	}{
		{"missing parts", "GET /\r\nHost: x\r\n\r\n", ErrMalformedRequestLine},
		{"too many parts", "GET / x HTTP/1.1\r\nHost: x\r\n\r\n", ErrMalformedRequestLine},
		{"double space", "GET  / HTTP/1.1\r\nHost: x\r\n\r\n", ErrMalformedRequestLine},
		{"invalid method", "G(T / HTTP/1.1\r\nHost: x\r\n\r\n", ErrMalformedRequestLine},
		{"relative target", "GET hello HTTP/1.1\r\nHost: x\r\n\r\n", ErrMalformedRequestLine},
		{"garbage version", "GET / HTTX/1.1\r\nHost: x\r\n\r\n", ErrMalformedRequestLine},
		{"unsupported version", "GET / HTTP/2.0\r\nHost: x\r\n\r\n", ErrUnsupportedVersion},
		{"malformed header", "GET / HTTP/1.1\r\nHost x\r\n\r\n", ErrMalformedHeader},
		{"missing host on 1.1", "GET / HTTP/1.1\r\n\r\n", ErrMissingHost},
		{"duplicate host", "GET / HTTP/1.1\r\nHost: a\r\nHost: b\r\n\r\n", ErrMalformedHeader},
		{"truncated headers", "GET / HTTP/1.1\r\nHost: x\r\n", io.ErrUnexpectedEOF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(tt.raw)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestParse_LineTooLong(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxLineBytes = 32
	raw := "GET /" + strings.Repeat("a", 64) + " HTTP/1.1\r\nHost: x\r\n\r\n"

	_, err := NewParser(limits).Parse(bufio.NewReader(strings.NewReader(raw)))
	if !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("err = %v, want ErrLineTooLong", err)
	}
}

func TestParse_TooManyHeaders(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxHeaderCount = 2
	raw := "GET / HTTP/1.1\r\nHost: x\r\nA: 1\r\nB: 2\r\n\r\n"

	_, err := NewParser(limits).Parse(bufio.NewReader(strings.NewReader(raw)))
	if !errors.Is(err, ErrTooManyHeaders) {
		t.Fatalf("err = %v, want ErrTooManyHeaders", err)
	}
}

func TestParse_ReadsPipelinedRequestsSequentially(t *testing.T) {
	raw := "GET /a HTTP/1.1\r\nHost: x\r\n\r\nGET /b HTTP/1.1\r\nHost: x\r\n\r\n"
	r := bufio.NewReader(strings.NewReader(raw))
	p := NewParser(DefaultLimits())

	for _, want := range []string{"/a", "/b"} {
		req, err := p.Parse(r)
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if req.Path != want {
			t.Fatalf("Path = %q, want %q", req.Path, want)
		}
	}
}
