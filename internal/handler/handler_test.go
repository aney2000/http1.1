package handler

import (
	"strings"
	"testing"

	"github.com/uig27055/http1.1/internal/request"
	"github.com/uig27055/http1.1/internal/response"
)

type recorder struct {
	*response.Buffered
}

func newRecorder() recorder {
	return recorder{response.NewBuffered(&strings.Builder{}, response.Options{})}
}

func TestHandlerFunc_ImplementsHandler(t *testing.T) {
	called := false
	var h Handler = Func(func(response.Writer, *request.Request) { called = true })

	h.ServeHTTP(newRecorder(), &request.Request{})

	if !called {
		t.Fatal("HandlerFunc was not invoked")
	}
}

func TestChain_AppliesMiddlewareOutermostFirst(t *testing.T) {
	var trace []string
	mw := func(name string) Middleware {
		return func(next Handler) Handler {
			return Func(func(w response.Writer, r *request.Request) {
				trace = append(trace, name+">")
				next.ServeHTTP(w, r)
				trace = append(trace, "<"+name)
			})
		}
	}
	final := Func(func(response.Writer, *request.Request) { trace = append(trace, "handler") })

	Chain(final, mw("a"), mw("b")).ServeHTTP(newRecorder(), &request.Request{})

	want := "a> b> handler <b <a"
	if got := strings.Join(trace, " "); got != want {
		t.Fatalf("trace = %q, want %q", got, want)
	}
}

func TestChain_NoMiddlewareReturnsHandler(t *testing.T) {
	called := false
	h := Chain(Func(func(response.Writer, *request.Request) { called = true }))
	h.ServeHTTP(newRecorder(), &request.Request{})
	if !called {
		t.Fatal("handler not invoked")
	}
}
