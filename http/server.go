package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/exp/slog"
)

// Server represents an HTTP server. It is meant to wrap all HTTP functionality
// used by the application so that dependent packages do not need to reference the "net/http"
// package at all. This allows us to isolate all HTTP code to this "http" package.
type Server struct {
	router *httprouter.Router
	server *http.Server

	Addr            string
	Logger          *slog.Logger
	ShutdownTimeout time.Duration
}

func NewServer(options ...func(*Server)) *Server {
	srv := &Server{
		server: &http.Server{},
		router: httprouter.New(),
	}

	for _, opt := range options {
		opt(srv)
	}

	return srv
}

// Open will start the server.
func (s *Server) Open() error {
	err := s.server.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// Close will immediately close the server.
func (s *Server) Close() error {
	return s.server.Close()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.ShutdownTimeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func WithAddr(addr string) func(*Server) {
	return func(s *Server) {
		s.Addr = addr
		s.server.Addr = addr
	}
}

func WithIdleTimeout(d time.Duration) func(*Server) {
	return func(s *Server) {
		s.server.IdleTimeout = d
	}
}

func WithLogger(log *slog.Logger) func(*Server) {
	return func(s *Server) {
		s.Logger = log
	}
}

func WithReadTimeout(d time.Duration) func(*Server) {
	return func(s *Server) {
		s.server.ReadTimeout = d
	}
}

func WithWriteTimeout(d time.Duration) func(*Server) {
	return func(s *Server) {
		s.server.WriteTimeout = d
	}
}

func WithShutdownTimeout(d time.Duration) func(*Server) {
	return func(s *Server) {
		s.ShutdownTimeout = d
	}
}

func (s *Server) AttachRoutesV1() {
	s.router.HandlerFunc(http.MethodGet, "/v1/healthcheck", s.handleHealthCheck)

	s.server.Handler = s.router
}
