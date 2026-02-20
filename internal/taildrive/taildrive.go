// Package taildrive provides a local HTTP file server that exposes the
// workspace directory. At deployment time this server can be fronted by
// Tailscale (Taildrive) to make the workspace accessible across the tailnet.
package taildrive

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Config holds the settings for the Taildrive file-serving service.
type Config struct {
	Enabled   bool
	ShareName string
	AutoShare bool
	RootDir   string // workspace root path
	Logger    *slog.Logger
}

// Service manages an HTTP file server that serves the workspace directory.
type Service struct {
	cfg      Config
	server   *http.Server
	listener net.Listener
	logger   *slog.Logger
	mu       sync.Mutex
}

// New validates the config and returns a ready-to-start Service.
// When cfg.Enabled is false the returned service is a no-op stub.
func New(cfg Config) (*Service, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if cfg.Enabled {
		if err := validateConfig(cfg); err != nil {
			return nil, fmt.Errorf("taildrive config: %w", err)
		}
	}

	return &Service{
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Start begins serving the workspace directory over HTTP on a localhost
// auto-assigned port. If the service is disabled this is a no-op.
func (s *Service) Start() error {
	if !s.cfg.Enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return errors.New("taildrive: already started")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("taildrive listen: %w", err)
	}

	s.listener = ln
	s.server = &http.Server{
		Handler:      s.handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := s.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("taildrive server error", "error", err)
		}
	}()

	s.logger.Info("taildrive started",
		"addr", ln.Addr().String(),
		"shareName", s.cfg.ShareName,
		"rootDir", s.cfg.RootDir,
	)

	return nil
}

// Stop gracefully shuts down the HTTP server. If the service is disabled or
// was never started this is a no-op.
func (s *Service) Stop() error {
	if !s.cfg.Enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.logger.Info("taildrive stopping")
	err := s.server.Shutdown(ctx)

	s.server = nil
	s.listener = nil

	if err != nil {
		return fmt.Errorf("taildrive shutdown: %w", err)
	}
	return nil
}

// Addr returns the listen address (host:port) after Start has been called.
// Returns an empty string if the service is not running.
func (s *Service) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// handler builds the HTTP handler with path traversal protection.
func (s *Service) handler() http.Handler {
	fs := http.FileServer(http.Dir(s.cfg.RootDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the URL path and reject any traversal attempts.
		cleaned := filepath.ToSlash(filepath.Clean(r.URL.Path))

		// After cleaning, the path must not escape the root.
		// filepath.Clean resolves ".." segments, but we double-check.
		if strings.Contains(cleaned, "..") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// Verify the resolved filesystem path stays within root.
		absPath := filepath.Join(s.cfg.RootDir, filepath.FromSlash(cleaned))
		absPath, err := filepath.Abs(absPath)
		if err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		rootAbs, err := filepath.Abs(s.cfg.RootDir)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// The resolved path must be the root itself or a child of the root.
		if absPath != rootAbs && !strings.HasPrefix(absPath, rootAbs+string(filepath.Separator)) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// Serve via the standard file server.
		fs.ServeHTTP(w, r)
	})
}

// validateConfig checks that required fields are set and the root directory
// exists on disk.
func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.ShareName) == "" {
		return errors.New("share name must not be empty")
	}

	if strings.TrimSpace(cfg.RootDir) == "" {
		return errors.New("root directory must not be empty")
	}

	info, err := os.Stat(cfg.RootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("root directory does not exist: %s", cfg.RootDir)
		}
		return fmt.Errorf("root directory stat: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("root path is not a directory: %s", cfg.RootDir)
	}

	return nil
}
