package response

import (
	"strings"
	"testing"

	"github.com/uig27055/http1.1/internal/status"
)

func TestText(t *testing.T) {
	w, out := newTestWriter(Options{})
	Text(w, status.Created, "made it")
	_ = w.Finish()

	got := out.String()
	if !strings.HasPrefix(got, "HTTP/1.1 201 Created\r\n") ||
		!strings.Contains(got, "Content-Type: text/plain; charset=utf-8\r\n") ||
		!strings.HasSuffix(got, "made it") {
		t.Fatalf("got %q", got)
	}
}

func TestJSON(t *testing.T) {
	w, out := newTestWriter(Options{})
	JSON(w, status.OK, map[string]string{"msg": "hi"})
	_ = w.Finish()

	got := out.String()
	if !strings.Contains(got, "Content-Type: application/json\r\n") ||
		!strings.HasSuffix(got, "{\"msg\":\"hi\"}\n") {
		t.Fatalf("got %q", got)
	}
}

func TestJSON_UnencodableValueYields500(t *testing.T) {
	w, _ := newTestWriter(Options{})
	JSON(w, status.OK, make(chan int))

	if w.Status() != status.InternalServerError {
		t.Fatalf("Status() = %d, want 500", w.Status())
	}
}

func TestError_UsesReasonPhrase(t *testing.T) {
	w, out := newTestWriter(Options{})
	Error(w, status.NotFound)
	_ = w.Finish()

	if !strings.HasSuffix(out.String(), "404 Not Found\n") {
		t.Fatalf("got %q", out.String())
	}
}
