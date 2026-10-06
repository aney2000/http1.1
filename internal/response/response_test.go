package response

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/uig27055/http1.1/internal/status"
)

var fixedNow = func() time.Time {
	return time.Date(2026, time.October, 6, 12, 0, 0, 0, time.UTC)
}

const fixedDate = "Tue, 06 Oct 2026 12:00:00 GMT"

func newTestWriter(opts Options) (*Buffered, *bytes.Buffer) {
	var out bytes.Buffer
	if opts.Now == nil {
		opts.Now = fixedNow
	}
	return NewBuffered(&out, opts), &out
}

func TestBuffered_DefaultsTo200WithContentLength(t *testing.T) {
	w, out := newTestWriter(Options{})
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}

	want := "HTTP/1.1 200 OK\r\n" +
		"Content-Length: 5\r\n" +
		"Content-Type: text/plain\r\n" +
		"Date: " + fixedDate + "\r\n" +
		"\r\n" +
		"hello"
	if out.String() != want {
		t.Fatalf("got\n%q\nwant\n%q", out.String(), want)
	}
}

func TestBuffered_WriteHeaderSetsStatus(t *testing.T) {
	w, out := newTestWriter(Options{})
	w.WriteHeader(status.NotFound)
	_, _ = w.Write([]byte("nope"))
	_ = w.Finish()

	if !strings.HasPrefix(out.String(), "HTTP/1.1 404 Not Found\r\n") {
		t.Fatalf("got %q", out.String())
	}
	if w.Status() != status.NotFound {
		t.Fatalf("Status() = %d", w.Status())
	}
}

func TestBuffered_OnlyFirstWriteHeaderWins(t *testing.T) {
	w, _ := newTestWriter(Options{})
	w.WriteHeader(status.Created)
	w.WriteHeader(status.InternalServerError)

	if w.Status() != status.Created {
		t.Fatalf("Status() = %d, want 201", w.Status())
	}
}

func TestBuffered_EmptyBodyHasZeroContentLength(t *testing.T) {
	w, out := newTestWriter(Options{})
	_ = w.Finish()

	if !strings.Contains(out.String(), "Content-Length: 0\r\n") {
		t.Fatalf("got %q", out.String())
	}
}

func TestBuffered_NoContentOmitsBodyAndLength(t *testing.T) {
	w, out := newTestWriter(Options{})
	w.WriteHeader(status.NoContent)
	_, _ = w.Write([]byte("ignored"))
	_ = w.Finish()

	got := out.String()
	if strings.Contains(got, "Content-Length") || strings.HasSuffix(got, "ignored") {
		t.Fatalf("got %q", got)
	}
}

func TestBuffered_OmitBodyKeepsContentLength(t *testing.T) {
	w, out := newTestWriter(Options{OmitBody: true})
	_, _ = w.Write([]byte("hello"))
	_ = w.Finish()

	got := out.String()
	if !strings.Contains(got, "Content-Length: 5\r\n") {
		t.Fatalf("HEAD response must advertise length, got %q", got)
	}
	if !strings.HasSuffix(got, "\r\n\r\n") {
		t.Fatalf("HEAD response must have no body, got %q", got)
	}
}

func TestBuffered_HandlerSetDateIsPreserved(t *testing.T) {
	w, out := newTestWriter(Options{})
	w.Header().Set("Date", "custom")
	_ = w.Finish()

	if !strings.Contains(out.String(), "Date: custom\r\n") {
		t.Fatalf("got %q", out.String())
	}
}

func TestBuffered_FinishTwiceFails(t *testing.T) {
	w, _ := newTestWriter(Options{})
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	if err := w.Finish(); !errors.Is(err, ErrAlreadyFinished) {
		t.Fatalf("err = %v, want ErrAlreadyFinished", err)
	}
}

func TestBuffered_WriteAfterFinishFails(t *testing.T) {
	w, _ := newTestWriter(Options{})
	_ = w.Finish()
	if _, err := w.Write([]byte("x")); !errors.Is(err, ErrAlreadyFinished) {
		t.Fatalf("err = %v, want ErrAlreadyFinished", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestBuffered_FinishPropagatesWriteErrors(t *testing.T) {
	w := NewBuffered(failingWriter{}, Options{Now: fixedNow})
	if err := w.Finish(); err == nil {
		t.Fatal("Finish() error = nil, want error")
	}
}

func TestBuffered_WriterInterfaceIsSatisfied(_ *testing.T) {
	var _ Writer = (*Buffered)(nil)
}
