package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

// Config configures the live studio HTTP server.
type Config struct {
	Host      string
	Port      int
	StatsPath string
	Client    *bundleradar.Client
	Watch     bool
}

// Server provides the live web UI HTTP server adapter.
type Server struct {
	host        string
	port        int
	statsPath   string
	client      *bundleradar.Client
	bundle      *core.Bundle
	mux         *http.ServeMux
	httpServer  *http.Server
	listener    net.Listener
	watch       bool
	stopWatcher chan struct{}
	checkpoints []*BuildCheckpoint
	baselineID  string
	clients     map[chan []byte]struct{}
	clientsMu   sync.Mutex
	mu          sync.RWMutex
}

// New creates an initialized HTTP server adapter.
func New(cfg Config) (*Server, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Client == nil {
		cfg.Client = bundleradar.New()
	}

	s := &Server{
		host:        cfg.Host,
		port:        cfg.Port,
		statsPath:   cfg.StatsPath,
		client:      cfg.Client,
		watch:       cfg.Watch,
		mux:         http.NewServeMux(),
		clients:     make(map[chan []byte]struct{}),
		checkpoints: make([]*BuildCheckpoint, 0),
	}

	s.setupRoutes()
	return s, nil
}

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": bundleradar.ToolVersion,
		})
	})
	s.mux.HandleFunc("/api/bundle", s.handleGetBundle)
	s.mux.HandleFunc("/api/history", s.handleGetHistory)
	s.mux.HandleFunc("/api/baseline", s.handleSetBaseline)
	s.mux.HandleFunc("/api/diff", s.handleGetDiff)
	s.mux.HandleFunc("/api/events", s.handleGetEvents)
	s.registerStaticRoutes()
}

// Handler returns the HTTP handler with developer CORS enabled.
func (s *Server) Handler() http.Handler {
	allowedOrigin := fmt.Sprintf("http://%s:%d", s.host, s.port)
	if s.host == "127.0.0.1" || s.host == "localhost" || s.host == "::1" {
		allowedOrigin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.mux.ServeHTTP(w, r)
	})
}

// Start binds the listener and starts serving HTTP requests asynchronously.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind %s: %w", addr, err)
	}
	s.listener = ln
	s.port = ln.Addr().(*net.TCPAddr).Port

	s.httpServer = &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		_ = s.httpServer.Serve(ln)
	}()

	if s.watch && s.statsPath != "" {
		s.StartWatcher()
	}

	return nil
}

// Close gracefully terminates the server with a bounded 5-second timeout.
func (s *Server) Close() error {
	s.StopWatcher()

	s.clientsMu.Lock()
	for ch := range s.clients {
		close(ch)
	}
	s.clients = make(map[chan []byte]struct{})
	s.clientsMu.Unlock()

	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// SetBundle updates the server's cached bundle AST.
func (s *Server) SetBundle(bundle *core.Bundle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bundle = bundle
}

// Bundle returns the server's cached bundle AST.
func (s *Server) Bundle() *core.Bundle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bundle
}

// URL returns the base URL of the running server.
func (s *Server) URL() string {
	if s.listener == nil {
		return fmt.Sprintf("http://%s:%d", s.host, s.port)
	}
	return fmt.Sprintf("http://%s", s.listener.Addr().String())
}
