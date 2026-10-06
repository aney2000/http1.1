package router

import (
	"strings"
	"testing"

	"github.com/uig27055/http1.1/internal/handler"
	"github.com/uig27055/http1.1/internal/request"
	"github.com/uig27055/http1.1/internal/response"
	"github.com/uig27055/http1.1/internal/status"
)

func serve(rt *Router, method, path string) *response.Buffered {
	w := response.NewBuffered(&strings.Builder{}, response.Options{})
	rt.ServeHTTP(w, &request.Request{Method: method, Path: path})
	return w
}

func named(name string, got *string) handler.Handler {
	return handler.Func(func(_ response.Writer, r *request.Request) {
		*got = name
		if r.Params != nil {
			*got += ":" + r.Param("id")
		}
	})
}

func TestRouter_MatchesExactPath(t *testing.T) {
	var got string
	rt := New()
	rt.Handle("GET", "/", named("root", &got))
	rt.Handle("GET", "/users", named("users", &got))

	serve(rt, "GET", "/users")

	if got != "users" {
		t.Fatalf("matched %q, want users", got)
	}
}

func TestRouter_ExtractsPathParams(t *testing.T) {
	var got string
	rt := New()
	rt.Handle("GET", "/users/{id}", named("user", &got))

	serve(rt, "GET", "/users/42")

	if got != "user:42" {
		t.Fatalf("matched %q, want user:42", got)
	}
}

func TestRouter_StaticSegmentBeatsParamRegardlessOfOrder(t *testing.T) {
	var got string
	rt := New()
	rt.Handle("GET", "/users/{id}", named("param", &got))
	rt.Handle("GET", "/users/me", named("static", &got))

	serve(rt, "GET", "/users/me")

	if got != "static" {
		t.Fatalf("matched %q, want static", got)
	}
}

func TestRouter_TrailingSlashIsSignificant(t *testing.T) {
	var got string
	rt := New()
	rt.Handle("GET", "/users", named("users", &got))

	w := serve(rt, "GET", "/users/")

	if w.Status() != status.NotFound {
		t.Fatalf("Status() = %d, want 404", w.Status())
	}
}

func TestRouter_UnknownPathIs404(t *testing.T) {
	rt := New()
	rt.Handle("GET", "/a", handler.Func(func(response.Writer, *request.Request) {}))

	if w := serve(rt, "GET", "/b"); w.Status() != status.NotFound {
		t.Fatalf("Status() = %d, want 404", w.Status())
	}
}

func TestRouter_WrongMethodIs405WithAllowHeader(t *testing.T) {
	noop := handler.Func(func(response.Writer, *request.Request) {})
	rt := New()
	rt.Handle("POST", "/items", noop)
	rt.Handle("GET", "/items", noop)

	w := serve(rt, "DELETE", "/items")

	if w.Status() != status.MethodNotAllowed {
		t.Fatalf("Status() = %d, want 405", w.Status())
	}
	if got := w.Header().Get("Allow"); got != "GET, HEAD, POST" {
		t.Fatalf("Allow = %q, want %q", got, "GET, HEAD, POST")
	}
}

func TestRouter_LessSpecificRouteServesMethodMissingOnMoreSpecific(t *testing.T) {
	var got string
	rt := New()
	rt.Handle("GET", "/users/me", named("me", &got))
	rt.Handle("POST", "/users/{id}", named("update", &got))

	serve(rt, "POST", "/users/me")

	if got != "update:me" {
		t.Fatalf("matched %q, want update:me", got)
	}
}

func TestRouter_HeadFallsBackToGet(t *testing.T) {
	var got string
	rt := New()
	rt.Handle("GET", "/page", named("page", &got))

	serve(rt, "HEAD", "/page")

	if got != "page" {
		t.Fatalf("HEAD did not reach GET handler, got %q", got)
	}
}

func TestRouter_CustomNotFoundHandler(t *testing.T) {
	var got string
	rt := New()
	rt.NotFound = named("custom404", &got)

	serve(rt, "GET", "/missing")

	if got != "custom404" {
		t.Fatalf("got %q", got)
	}
}

func TestRouter_HandleFuncShortcut(t *testing.T) {
	called := false
	rt := New()
	rt.HandleFunc("GET", "/f", func(response.Writer, *request.Request) { called = true })

	serve(rt, "GET", "/f")

	if !called {
		t.Fatal("HandleFunc handler not invoked")
	}
}

func TestRouter_InvalidPatternPanics(t *testing.T) {
	for _, pattern := range []string{"", "no-slash", "/a/{}", "/a/{id"} {
		t.Run(pattern, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("Handle(%q) did not panic", pattern)
				}
			}()
			New().Handle("GET", pattern, handler.Func(func(response.Writer, *request.Request) {}))
		})
	}
}

func TestRouter_DuplicateRoutePanics(t *testing.T) {
	noop := handler.Func(func(response.Writer, *request.Request) {})
	rt := New()
	rt.Handle("GET", "/x", noop)

	defer func() {
		if recover() == nil {
			t.Fatal("duplicate registration did not panic")
		}
	}()
	rt.Handle("GET", "/x", noop)
}
