// Package server accepts TCP connections and serves HTTP/1.1 over them.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/uig27055/http1.1/internal/handler"
	"github.com/uig27055/http1.1/internal/request"
)

// ErrServerClosed is returned by Serve after Shutdown has been called.
var ErrServerClosed = errors.New("server closed")

type connState int

const (
	stateIdle   connState = iota // waiting for the next request
	stateActive                  // reading or handling a request
)

// Server is an HTTP/1.1 server. Configure the exported fields before Serve.
type Server struct {
	Handler handler.Handler
	Logger  *slog.Logger
	Limits  request.Limits

	ReadTimeout  time.Duration // max time to read headers and body
	WriteTimeout time.Duration // max time to write a response
	IdleTimeout  time.Duration // max wait for the next keep-alive request

	mu        sync.Mutex
	closing   bool
	listeners map[net.Listener]struct{}
	conns     map[*conn]connState
	wg        sync.WaitGroup
}

// New returns a server with production-safe defaults.
func New(h handler.Handler) *Server {
	return &Server{
		Handler:      h,
		Logger:       slog.Default(),
		Limits:       request.DefaultLimits(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
		listeners:    make(map[net.Listener]struct{}),
		conns:        make(map[*conn]connState),
	}
}

// ListenAndServe listens on the TCP address and serves until Shutdown.
func (s *Server) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Serve accepts connections on ln, handling each in its own goroutine.
// It always returns a non-nil error; ErrServerClosed after Shutdown.
func (s *Server) Serve(ln net.Listener) error {
	if !s.trackListener(ln) {
		_ = ln.Close()
		return ErrServerClosed
	}
	parser := request.NewParser(s.Limits)

	for {
		netConn, err := ln.Accept()
		if err != nil {
			if s.isClosing() {
				return ErrServerClosed
			}
			return err
		}
		c := newConn(s, netConn, parser)
		if !s.trackConn(c) {
			_ = netConn.Close()
			return ErrServerClosed
		}
		go c.serve()
	}
}

// Shutdown gracefully stops the server: it stops accepting, closes idle
// connections and waits for in-flight requests to complete or ctx to end.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closing = true
	for ln := range s.listeners {
		_ = ln.Close()
		delete(s.listeners, ln)
	}
	for c, state := range s.conns {
		if state == stateIdle {
			_ = c.netConn.Close()
		}
	}
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) isClosing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closing
}

func (s *Server) trackListener(ln net.Listener) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	s.listeners[ln] = struct{}{}
	return true
}

// trackConn registers c. The closing check and wg.Add share the lock with
// Shutdown, so no connection can be added once Shutdown starts waiting.
func (s *Server) trackConn(c *conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	s.conns[c] = stateIdle
	s.wg.Add(1)
	return true
}

// setState records a connection transition; it returns false when the
// server is shutting down and the connection should be closed instead.
func (s *Server) setState(c *conn, state connState) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	s.conns[c] = state
	return true
}

func (s *Server) untrackConn(c *conn) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
	s.wg.Done()
}
