package request

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

func readBody(t *testing.T, req *Request) (string, error) {
	t.Helper()
	b, err := io.ReadAll(req.Body)
	return string(b), err
}

func TestBody_NoFramingHeadersMeansEmptyBody(t *testing.T) {
	req := mustParse(t, "POST / HTTP/1.1\r\nHost: x\r\n\r\nleftover")

	body, err := readBody(t, req)
	if err != nil || body != "" {
		t.Fatalf("body = %q, err = %v; want empty", body, err)
	}
}

func TestBody_ContentLength(t *testing.T) {
	req := mustParse(t, "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 5\r\n\r\nhello world")

	body, err := readBody(t, req)
	if err != nil || body != "hello" {
		t.Fatalf("body = %q, err = %v; want %q", body, err, "hello")
	}
	if req.ContentLength != 5 {
		t.Fatalf("ContentLength = %d, want 5", req.ContentLength)
	}
}

func TestBody_ContentLengthTruncatedIsUnexpectedEOF(t *testing.T) {
	req := mustParse(t, "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 10\r\n\r\nshort")

	if _, err := readBody(t, req); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestBody_IdenticalDuplicateContentLengthIsAccepted(t *testing.T) {
	req := mustParse(t, "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 2\r\nContent-Length: 2\r\n\r\nok")

	if body, _ := readBody(t, req); body != "ok" {
		t.Fatalf("body = %q", body)
	}
}

func TestBody_Chunked(t *testing.T) {
	raw := "POST / HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\n" +
		"5\r\nhello\r\n" +
		"7;ext=1\r\n, world\r\n" +
		"0\r\n" +
		"X-Trailer: ignored\r\n" +
		"\r\n"
	req := mustParse(t, raw)

	body, err := readBody(t, req)
	if err != nil || body != "hello, world" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
	if req.ContentLength != -1 {
		t.Fatalf("ContentLength = %d, want -1 (unknown)", req.ContentLength)
	}
}

func TestBody_ChunkedHexSizes(t *testing.T) {
	raw := "POST / HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\n" +
		"A\r\n0123456789\r\n0\r\n\r\n"
	body, err := readBody(t, mustParse(t, raw))
	if err != nil || body != "0123456789" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

func TestBody_ChunkedErrors(t *testing.T) {
	prefix := "POST / HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\n"
	tests := []struct {
		name  string
		chunk string
		want  error
	}{
		{"invalid size", "zz\r\nhello\r\n0\r\n\r\n", ErrMalformedChunk},
		{"missing CRLF after data", "5\r\nhelloXX0\r\n\r\n", ErrMalformedChunk},
		{"truncated", "5\r\nhel", io.ErrUnexpectedEOF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mustParse(t, prefix+tt.chunk)
			if _, err := readBody(t, req); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestBody_FramingErrors(t *testing.T) {
	tests := []struct {
		name string
		hdrs string
		want error
	}{
		{"negative length", "Content-Length: -1\r\n", ErrInvalidContentLength},
		{"non numeric length", "Content-Length: abc\r\n", ErrInvalidContentLength},
		{"conflicting lengths", "Content-Length: 1\r\nContent-Length: 2\r\n", ErrInvalidContentLength},
		{"unknown encoding", "Transfer-Encoding: gzip\r\n", ErrUnsupportedTransferEncoding},
		{"both TE and CL (smuggling)", "Transfer-Encoding: chunked\r\nContent-Length: 3\r\n", ErrAmbiguousFraming},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse("POST / HTTP/1.1\r\nHost: x\r\n" + tt.hdrs + "\r\n")
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestBody_ContentLengthOverLimitIsRejectedUpfront(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxBodyBytes = 4

	_, err := NewParser(limits).Parse(bufio.NewReader(strings.NewReader(
		"POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 5\r\n\r\nhello")))
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("err = %v, want ErrBodyTooLarge", err)
	}
}

func TestBody_ChunkedOverLimitFailsWhileReading(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxBodyBytes = 4

	req, err := NewParser(limits).Parse(bufio.NewReader(strings.NewReader(
		"POST / HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n\r\n")))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if _, err := readBody(t, req); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("err = %v, want ErrBodyTooLarge", err)
	}
}

func TestBody_PipelinedRequestAfterBody(t *testing.T) {
	raw := "POST /a HTTP/1.1\r\nHost: x\r\nContent-Length: 3\r\n\r\nabc" +
		"POST /b HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\n1\r\nz\r\n0\r\n\r\n" +
		"GET /c HTTP/1.1\r\nHost: x\r\n\r\n"
	r := bufio.NewReader(strings.NewReader(raw))
	p := NewParser(DefaultLimits())

	for _, want := range []string{"/a", "/b", "/c"} {
		req, err := p.Parse(r)
		if err != nil {
			t.Fatalf("Parse(%s) error = %v", want, err)
		}
		if req.Path != want {
			t.Fatalf("Path = %q, want %q", req.Path, want)
		}
		if _, err := io.Copy(io.Discard, req.Body); err != nil {
			t.Fatalf("drain body: %v", err)
		}
	}
}
