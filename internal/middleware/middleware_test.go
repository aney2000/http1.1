package middleware

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/uig27055/http1.1/internal/handler"
	"github.com/uig27055/http1.1/internal/request"
	"github.com/uig27055/http1.1/internal/response"
	"github.com/uig27055/http1.1/internal/status"
)

func newWriter() *response.Buffered {
	return response.NewBuffered(&strings.Builder{}, response.Options{})
}

func testLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

func TestRecover_TurnsPanicInto500AndLogs(t *testing.T) {
	logger, logs := testLogger()
	panicking := handler.Func(func(response.Writer, *request.Request) { panic("boom") })

	w := newWriter()
	Recover(logger)(panicking).ServeHTTP(w, &request.Request{Method: "GET", Path: "/x"})

	if w.Status() != status.InternalServerError {
		t.Fatalf("Status() = %d, want 500", w.Status())
	}
	if !strings.Contains(logs.String(), "boom") {
		t.Fatalf("panic not logged: %q", logs.String())
	}
}

func TestRecover_PassesThroughWithoutPanic(t *testing.T) {
	logger, _ := testLogger()
	ok := handler.Func(func(w response.Writer, _ *request.Request) { w.WriteHeader(status.Created) })

	w := newWriter()
	Recover(logger)(ok).ServeHTTP(w, &request.Request{})

	if w.Status() != status.Created {
		t.Fatalf("Status() = %d, want 201", w.Status())
	}
}

func TestLogger_LogsMethodPathAndStatus(t *testing.T) {
	logger, logs := testLogger()
	h := handler.Func(func(w response.Writer, _ *request.Request) { w.WriteHeader(status.NotFound) })

	Logger(logger)(h).ServeHTTP(newWriter(), &request.Request{Method: "GET", Path: "/missing"})

	out := logs.String()
	for _, want := range []string{"method=GET", "path=/missing", "status=404", "duration="} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q missing %q", out, want)
		}
	}
}
