package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	logger "k8s.io/klog/v2"
)

type ServerOption func(*Server)

// WithTimeout sets the shutdown timeout for the server.
func WithTimeout(timeout time.Duration) ServerOption {
	return func(s *Server) {
		s.shutdownTimeout = timeout
	}
}

type Server struct {
	httpServer      http.Server
	listener        net.Listener
	endpoint        string
	shutdownTimeout time.Duration
}

func NewServer(port int, endpoint string, handler http.Handler, options ...ServerOption) (*Server, error) {
	addr := "0"
	if port != 0 {
		addr = ":" + strconv.Itoa(port)
	}

	listener, err := newListener(addr)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle(endpoint, handler)

	s := &Server{
		endpoint:        endpoint,
		listener:        listener,
		httpServer:      http.Server{Handler: mux},
		shutdownTimeout: 30 * time.Second,  // Default value
	}

	for _, opt := range options {
		opt(s)
	}

	return s, nil
}

func (s *Server) Start() (err error) {
	if s.listener == nil {
		logger.Infof("Serving endpoint %s is disabled", s.endpoint)
		return
	}
	
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("serving endpoint %s failed: %v", s.endpoint, r)
		}
	}()
	
	logger.Infof("Started serving endpoint %s at %s", s.endpoint, s.listener.Addr())
	if e := s.httpServer.Serve(s.listener); e != http.ErrServerClosed {
		return fmt.Errorf("serving endpoint %s failed: %v", s.endpoint, e)
	}
	return
}

func (s *Server) Shutdown() error {
	if s.listener == nil {
		return nil
	}
	
	logger.Info("Stopping server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try graceful shutdown
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown server gracefully: %v", err)
	}
	return s.httpServer.Shutdown(shutdownCtx)
}

// newListener creates a new TCP listener bound to the given address.
func newListener(addr string) (net.Listener, error) {
	// Add a case to disable serving altogether
	if addr == "0" {
		return nil, nil
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %v", err)
	}

	return listener, nil
}
