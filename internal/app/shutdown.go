package app

import (
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/pocketbrain/pocketbrain/internal/store"
)

// shutdown coordinates graceful shutdown of all components.
type shutdown struct {
	logger  *slog.Logger
	cancel  func()
	db      *store.DB
	once    sync.Once
	mu      sync.Mutex
	closers []func()
}

func newShutdown(logger *slog.Logger, cancel func(), db *store.DB) *shutdown {
	return &shutdown{
		logger: logger,
		cancel: cancel,
		db:     db,
	}
}

// addCloser registers a function to be called during shutdown, in LIFO order.
func (s *shutdown) addCloser(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closers = append(s.closers, fn)
}

func (s *shutdown) handleSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		sig := <-sigCh
		s.logger.Info("received signal", "signal", sig.String())
		s.cancel()
	}()
}

func (s *shutdown) run() {
	s.once.Do(func() {
		s.logger.Info("shutting down...")

		// Call registered closers in LIFO order.
		s.mu.Lock()
		closers := make([]func(), len(s.closers))
		copy(closers, s.closers)
		s.mu.Unlock()

		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}

		// Close database.
		if err := s.db.Close(); err != nil {
			s.logger.Error("database close error", "error", err)
		}

		s.logger.Info("shutdown complete")
	})
}
