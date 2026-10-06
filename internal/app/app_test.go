package app

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/uig27055/http1.1/internal/handler"
	"github.com/uig27055/http1.1/internal/notes"
	"github.com/uig27055/http1.1/internal/request"
	"github.com/uig27055/http1.1/internal/response"
)

type result struct {
	status int
	header http.Header
	body   string
}

// do runs a raw HTTP request through the full app stack (parser, router,
// middleware, handlers, writer) without a network socket.
func do(t *testing.T, h handler.Handler, raw string) result {
	t.Helper()
	req, err := request.NewParser(request.DefaultLimits()).Parse(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var out bytes.Buffer
	w := response.NewBuffered(&out, response.Options{OmitBody: req.Method == "HEAD"})
	h.ServeHTTP(w, req)
	if err := w.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(&out), &http.Request{Method: req.Method})
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return result{resp.StatusCode, resp.Header, string(body)}
}

func newApp() *App {
	return New(notes.NewMemoryStore(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func get(path string) string {
	return "GET " + path + " HTTP/1.1\r\nHost: x\r\n\r\n"
}

func withBody(method, path, body string) string {
	return method + " " + path + " HTTP/1.1\r\nHost: x\r\nContent-Length: " +
		strconv.Itoa(len(body)) + "\r\n\r\n" + body
}

func TestApp_Index(t *testing.T) {
	r := do(t, newApp().Handler(), get("/"))
	if r.status != 200 || !strings.Contains(r.body, "http1.1") {
		t.Fatalf("got %d %q", r.status, r.body)
	}
}

func TestApp_Health(t *testing.T) {
	r := do(t, newApp().Handler(), get("/health"))
	if r.status != 200 || r.body != "{\"status\":\"ok\"}\n" {
		t.Fatalf("got %d %q", r.status, r.body)
	}
}

func TestApp_HelloUsesPathParam(t *testing.T) {
	r := do(t, newApp().Handler(), get("/hello/gopher"))
	if r.body != "Hello, gopher!" {
		t.Fatalf("body = %q", r.body)
	}
}

func TestApp_EchoReturnsBodyAndContentType(t *testing.T) {
	raw := "POST /echo HTTP/1.1\r\nHost: x\r\nContent-Type: application/xml\r\nContent-Length: 6\r\n\r\n<a/>!!"
	r := do(t, newApp().Handler(), raw)

	if r.body != "<a/>!!" || r.header.Get("Content-Type") != "application/xml" {
		t.Fatalf("got %q, type %q", r.body, r.header.Get("Content-Type"))
	}
}

func TestApp_EchoChunkedBody(t *testing.T) {
	raw := "POST /echo HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\n3\r\nabc\r\n2\r\nde\r\n0\r\n\r\n"
	if r := do(t, newApp().Handler(), raw); r.body != "abcde" {
		t.Fatalf("body = %q", r.body)
	}
}

func TestApp_NotesLifecycle(t *testing.T) {
	h := newApp().Handler()

	created := do(t, h, withBody("POST", "/notes", `{"text":"buy milk"}`))
	if created.status != 201 || created.header.Get("Location") != "/notes/1" {
		t.Fatalf("create: %d, Location %q", created.status, created.header.Get("Location"))
	}

	if r := do(t, h, get("/notes/1")); r.body != "{\"id\":1,\"text\":\"buy milk\"}\n" {
		t.Fatalf("get body = %q", r.body)
	}
	if r := do(t, h, get("/notes")); r.body != "[{\"id\":1,\"text\":\"buy milk\"}]\n" {
		t.Fatalf("list body = %q", r.body)
	}
	if r := do(t, h, "DELETE /notes/1 HTTP/1.1\r\nHost: x\r\n\r\n"); r.status != 204 {
		t.Fatalf("delete status = %d", r.status)
	}
	if r := do(t, h, get("/notes/1")); r.status != 404 {
		t.Fatalf("get after delete = %d", r.status)
	}
}

func TestApp_NotesValidation(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{"invalid json", withBody("POST", "/notes", "{oops"), 400},
		{"empty text", withBody("POST", "/notes", `{"text":"  "}`), 400},
		{"non numeric id", get("/notes/abc"), 400},
		{"missing note", get("/notes/42"), 404},
		{"delete missing", "DELETE /notes/42 HTTP/1.1\r\nHost: x\r\n\r\n", 404},
		{"wrong method", "PUT /notes HTTP/1.1\r\nHost: x\r\n\r\n", 405},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if r := do(t, newApp().Handler(), tt.raw); r.status != tt.want {
				t.Fatalf("status = %d, want %d (body %q)", r.status, tt.want, r.body)
			}
		})
	}
}

func TestApp_HeadOnHealth(t *testing.T) {
	r := do(t, newApp().Handler(), "HEAD /health HTTP/1.1\r\nHost: x\r\n\r\n")
	if r.status != 200 || r.body != "" {
		t.Fatalf("got %d %q", r.status, r.body)
	}
}

func TestApp_PanicIsRecovered(t *testing.T) {
	a := newApp()
	a.Router().HandleFunc("GET", "/boom", func(response.Writer, *request.Request) { panic("x") })

	if r := do(t, a.Handler(), get("/boom")); r.status != 500 {
		t.Fatalf("status = %d, want 500", r.status)
	}
}
