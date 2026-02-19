package channel

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// Manager registers ChannelAdapter instances and manages their lifecycle.
type Manager struct {
	mu       sync.RWMutex
	adapters map[string]core.ChannelAdapter
	logger   *slog.Logger
}

// NewManager creates an empty Manager.
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		adapters: make(map[string]core.ChannelAdapter),
		logger:   logger,
	}
}

// Register adds an adapter to the manager.  If an adapter with the same name
// is already registered it is replaced.
func (m *Manager) Register(adapter core.ChannelAdapter) {
	m.mu.Lock()
	m.adapters[adapter.Name()] = adapter
	m.mu.Unlock()
	m.logger.Info("channel adapter registered", "channel", adapter.Name())
}

// Start calls Start on every registered adapter concurrently.  It returns the
// first error encountered, but always waits for all goroutines to finish.
func (m *Manager) Start(handler core.MessageHandler) error {
	m.mu.RLock()
	names := make([]string, 0, len(m.adapters))
	adapters := make([]core.ChannelAdapter, 0, len(m.adapters))
	for name, a := range m.adapters {
		names = append(names, name)
		adapters = append(adapters, a)
	}
	m.mu.RUnlock()

	errs := make([]error, len(adapters))
	var wg sync.WaitGroup
	wg.Add(len(adapters))

	for i, a := range adapters {
		i, a := i, a
		go func() {
			defer wg.Done()
			errs[i] = a.Start(handler)
		}()
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	m.logger.Info("all channels started", "channels", names)
	return nil
}

// Stop calls Stop on every registered adapter concurrently.  It returns the
// first error encountered, but always waits for all goroutines to finish.
func (m *Manager) Stop() error {
	m.mu.RLock()
	adapters := make([]core.ChannelAdapter, 0, len(m.adapters))
	for _, a := range m.adapters {
		adapters = append(adapters, a)
	}
	m.mu.RUnlock()

	errs := make([]error, len(adapters))
	var wg sync.WaitGroup
	wg.Add(len(adapters))

	for i, a := range adapters {
		i, a := i, a
		go func() {
			defer wg.Done()
			errs[i] = a.Stop()
		}()
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	m.logger.Info("all channels stopped")
	return nil
}

// Send delivers text to userID via the named channel adapter.
func (m *Manager) Send(channel, userID, text string) error {
	m.mu.RLock()
	a, ok := m.adapters[channel]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown channel: %s", channel)
	}
	return a.Send(userID, text)
}

// Get returns the adapter registered under channel, or nil if not found.
func (m *Manager) Get(channel string) core.ChannelAdapter {
	m.mu.RLock()
	a := m.adapters[channel]
	m.mu.RUnlock()
	return a
}

// Channels returns the names of all registered adapters.
func (m *Manager) Channels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.adapters))
	for name := range m.adapters {
		names = append(names, name)
	}
	return names
}
