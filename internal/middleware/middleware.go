// Package middleware provides reusable cross-cutting handler decorators.
package middleware

import (
	"log/slog"
	"time"

	"github.com/uig27055/http1.1/internal/handler"
	"github.com/uig27055/http1.1/internal/request"
	"github.com/uig27055/http1.1/internal/response"
	"github.com/uig27055/http1.1/internal/status"
)

// Recover converts a panicking handler into a 500 response so one faulty
// request cannot crash the whole server.
func Recover(logger *slog.Logger) handler.Middleware {
	return func(next handler.Handler) handler.Handler {
		return handler.Func(func(w response.Writer, r *request.Request) {
			defer func() {
				if v := recover(); v != nil {
					logger.Error("handler panic", "method", r.Method, "path", r.Path, "panic", v)
					response.Error(w, status.InternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Logger records one structured access-log line per request.
func Logger(logger *slog.Logger) handler.Middleware {
	return func(next handler.Handler) handler.Handler {
		return handler.Func(func(w response.Writer, r *request.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.Path,
				"status", int(w.Status()),
				"duration", time.Since(start),
			)
		})
	}
}
