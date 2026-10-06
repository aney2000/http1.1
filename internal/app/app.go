// Package app wires the demo application: routes, middleware and handlers.
// It is the composition root for HTTP concerns and keeps main.go trivial.
package app

import (
	"log/slog"

	"github.com/uig27055/http1.1/internal/handler"
	"github.com/uig27055/http1.1/internal/middleware"
	"github.com/uig27055/http1.1/internal/notes"
	"github.com/uig27055/http1.1/internal/router"
)

// App holds the application's dependencies and routes.
type App struct {
	router *router.Router
	logger *slog.Logger
}

// New builds the application with its routes registered.
func New(store notes.Store, logger *slog.Logger) *App {
	a := &App{router: router.New(), logger: logger}
	nh := &notesHandler{store: store}

	a.router.HandleFunc("GET", "/", index)
	a.router.HandleFunc("GET", "/health", health)
	a.router.HandleFunc("GET", "/hello/{name}", hello)
	a.router.HandleFunc("POST", "/echo", echo)
	a.router.HandleFunc("GET", "/notes", nh.list)
	a.router.HandleFunc("POST", "/notes", nh.create)
	a.router.HandleFunc("GET", "/notes/{id}", nh.get)
	a.router.HandleFunc("DELETE", "/notes/{id}", nh.delete)
	return a
}

// Router exposes the router so callers can register extra routes.
func (a *App) Router() *router.Router { return a.router }

// Handler returns the router wrapped in the standard middleware stack.
// Recover sits inside Logger so the logged status reflects the 500.
func (a *App) Handler() handler.Handler {
	return handler.Chain(a.router,
		middleware.Logger(a.logger),
		middleware.Recover(a.logger),
	)
}
