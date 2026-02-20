package app

import (
	"context"
	"log/slog"

	"github.com/pocketbrain/pocketbrain/internal/tui"
)

// BusHandler is a slog.Handler that publishes log records to the TUI event bus.
type BusHandler struct {
	bus   *tui.EventBus
	level slog.Level
	attrs []slog.Attr
}

func NewBusHandler(bus *tui.EventBus, level slog.Level) *BusHandler {
	return &BusHandler{bus: bus, level: level}
}

func (h *BusHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *BusHandler) Handle(_ context.Context, r slog.Record) error {
	fields := make(map[string]any)
	for _, a := range h.attrs {
		fields[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		fields[a.Key] = a.Value.Any()
		return true
	})

	var level string
	switch {
	case r.Level >= slog.LevelError:
		level = "ERROR"
	case r.Level >= slog.LevelWarn:
		level = "WARN"
	case r.Level >= slog.LevelInfo:
		level = "INFO"
	default:
		level = "DEBUG"
	}

	h.bus.PublishLog(level, r.Message, fields)
	return nil
}

func (h *BusHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &BusHandler{bus: h.bus, level: h.level, attrs: newAttrs}
}

func (h *BusHandler) WithGroup(name string) slog.Handler {
	// Groups not needed for TUI display; treat as flat.
	return h
}
