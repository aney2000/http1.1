// Package router dispatches requests to handlers by method and path.
package router

import (
	"fmt"
	"sort"
	"strings"

	"github.com/uig27055/http1.1/internal/handler"
	"github.com/uig27055/http1.1/internal/request"
	"github.com/uig27055/http1.1/internal/response"
	"github.com/uig27055/http1.1/internal/status"
)

type route struct {
	pattern  pattern
	handlers map[string]handler.Handler // keyed by method
}

// Router is a Handler that dispatches to registered routes. Routes are
// matched by specificity, so "/users/me" wins over "/users/{id}".
type Router struct {
	routes []*route

	// NotFound handles unmatched paths; defaults to a plain 404.
	NotFound handler.Handler
}

// New creates an empty Router.
func New() *Router {
	return &Router{
		NotFound: handler.Func(func(w response.Writer, _ *request.Request) {
			response.Error(w, status.NotFound)
		}),
	}
}

// Handle registers h for method and pattern. Misconfiguration is a
// programming error, so it panics at startup rather than failing at runtime.
func (rt *Router) Handle(method, rawPattern string, h handler.Handler) {
	p, err := compilePattern(rawPattern)
	if err != nil {
		panic(err)
	}

	r := rt.findRoute(rawPattern)
	if r == nil {
		r = &route{pattern: p, handlers: make(map[string]handler.Handler)}
		rt.routes = append(rt.routes, r)
		rt.sortBySpecificity()
	}
	if _, exists := r.handlers[method]; exists {
		panic(fmt.Sprintf("router: duplicate route %s %s", method, rawPattern))
	}
	r.handlers[method] = h
}

// HandleFunc registers a plain function as a handler.
func (rt *Router) HandleFunc(method, rawPattern string, f func(response.Writer, *request.Request)) {
	rt.Handle(method, rawPattern, handler.Func(f))
}

// ServeHTTP dispatches the request to the best matching route.
// A path matched only under other methods yields 405 with an Allow header.
func (rt *Router) ServeHTTP(w response.Writer, req *request.Request) {
	allowed := map[string]bool{}
	for _, r := range rt.routes {
		params, ok := r.pattern.match(req.Path)
		if !ok {
			continue
		}
		if h := r.handlerFor(req.Method); h != nil {
			req.Params = params
			h.ServeHTTP(w, req)
			return
		}
		r.collectAllowed(allowed)
	}

	if len(allowed) > 0 {
		w.Header().Set("Allow", joinSorted(allowed))
		response.Error(w, status.MethodNotAllowed)
		return
	}
	rt.NotFound.ServeHTTP(w, req)
}

// handlerFor resolves the handler, letting HEAD reuse GET (RFC 9110 §9.3.2).
func (r *route) handlerFor(method string) handler.Handler {
	if h, ok := r.handlers[method]; ok {
		return h
	}
	if method == "HEAD" {
		return r.handlers["GET"]
	}
	return nil
}

func (r *route) collectAllowed(into map[string]bool) {
	for m := range r.handlers {
		into[m] = true
	}
	if _, hasGet := r.handlers["GET"]; hasGet {
		into["HEAD"] = true
	}
}

func joinSorted(set map[string]bool) string {
	items := make([]string, 0, len(set))
	for item := range set {
		items = append(items, item)
	}
	sort.Strings(items)
	return strings.Join(items, ", ")
}

func (rt *Router) findRoute(rawPattern string) *route {
	for _, r := range rt.routes {
		if r.pattern.raw == rawPattern {
			return r
		}
	}
	return nil
}

func (rt *Router) sortBySpecificity() {
	sort.SliceStable(rt.routes, func(i, j int) bool {
		return rt.routes[i].pattern.staticCount() > rt.routes[j].pattern.staticCount()
	})
}
