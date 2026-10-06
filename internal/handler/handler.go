// Package handler defines the contract between the server and application
// code. The server depends only on this abstraction (Dependency Inversion),
// so routing, middleware and business logic can evolve independently.
package handler

import (
	"github.com/uig27055/http1.1/internal/request"
	"github.com/uig27055/http1.1/internal/response"
)

// Handler responds to an HTTP request.
type Handler interface {
	ServeHTTP(w response.Writer, r *request.Request)
}

// Func adapts an ordinary function to the Handler interface.
type Func func(w response.Writer, r *request.Request)

// ServeHTTP calls f(w, r).
func (f Func) ServeHTTP(w response.Writer, r *request.Request) { f(w, r) }

// Middleware decorates a Handler with cross-cutting behaviour, extending
// it without modifying it (Open/Closed).
type Middleware func(next Handler) Handler

// Chain wraps h so that the first middleware is the outermost layer.
func Chain(h Handler, middlewares ...Middleware) Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
