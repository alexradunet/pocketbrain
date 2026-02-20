package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// TestToolLoop_NoToolCalls_ReturnsText
// ---------------------------------------------------------------------------

func TestToolLoop_NoToolCalls_ReturnsText(t *testing.T) {
	srv := newTestServer(chatResponse("plain text answer", nil))
	defer srv.Close()

	p := NewFantasyProvider(FantasyConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	reg := NewRegistry()
	sid, _ := p.CreateSession(context.Background(), "test")

	result, err := RunToolLoop(context.Background(), p, reg, sid, "system", "hello", 5)
	if err != nil {
		t.Fatalf("RunToolLoop error: %v", err)
	}
	if result != "plain text answer" {
		t.Errorf("result = %q; want %q", result, "plain text answer")
	}
}

// ---------------------------------------------------------------------------
// TestToolLoop_SingleToolCall_ExecutesAndContinues
// ---------------------------------------------------------------------------

func TestToolLoop_SingleToolCall_ExecutesAndContinues(t *testing.T) {
	var callCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			// First call: return a tool call.
			_, _ = io.WriteString(w, chatResponse("", []toolCallResponse{
				{ID: "call_1", Name: "get_time", Arguments: `{}`},
			}))
		} else {
			// Second call (after tool result): return final text.
			_, _ = io.WriteString(w, chatResponse("The time is 12:00 PM", nil))
		}
	}))
	defer srv.Close()

	reg := NewRegistry()
	reg.Register(&Tool{
		Name:        "get_time",
		Description: "Get current time",
		Execute: func(args map[string]any) (string, error) {
			return "12:00 PM", nil
		},
	})

	p := NewFantasyProvider(FantasyConfig{
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "test-model",
		Registry: reg,
	})
	sid, _ := p.CreateSession(context.Background(), "test")

	result, err := RunToolLoop(context.Background(), p, reg, sid, "", "what time is it?", 5)
	if err != nil {
		t.Fatalf("RunToolLoop error: %v", err)
	}
	if result != "The time is 12:00 PM" {
		t.Errorf("result = %q; want %q", result, "The time is 12:00 PM")
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("API call count = %d; want 2", atomic.LoadInt32(&callCount))
	}
}

// ---------------------------------------------------------------------------
// TestToolLoop_MultipleToolCalls_ExecutesAll
// ---------------------------------------------------------------------------

func TestToolLoop_MultipleToolCalls_ExecutesAll(t *testing.T) {
	var callCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			// Return two tool calls at once.
			_, _ = io.WriteString(w, chatResponse("", []toolCallResponse{
				{ID: "call_a", Name: "add", Arguments: `{"a":"1","b":"2"}`},
				{ID: "call_b", Name: "add", Arguments: `{"a":"3","b":"4"}`},
			}))
		} else {
			_, _ = io.WriteString(w, chatResponse("Results: 3 and 7", nil))
		}
	}))
	defer srv.Close()

	reg := NewRegistry()
	reg.Register(&Tool{
		Name:        "add",
		Description: "Add two numbers",
		Parameters: []ToolParam{
			{Name: "a", Type: "string", Required: true},
			{Name: "b", Type: "string", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			return fmt.Sprintf("%s+%s", argString(args, "a"), argString(args, "b")), nil
		},
	})

	p := NewFantasyProvider(FantasyConfig{
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "test-model",
		Registry: reg,
	})
	sid, _ := p.CreateSession(context.Background(), "test")

	result, err := RunToolLoop(context.Background(), p, reg, sid, "", "add stuff", 5)
	if err != nil {
		t.Fatalf("RunToolLoop error: %v", err)
	}
	if result != "Results: 3 and 7" {
		t.Errorf("result = %q; want %q", result, "Results: 3 and 7")
	}
}

// ---------------------------------------------------------------------------
// TestToolLoop_MaxIterations_StopsLoop
// ---------------------------------------------------------------------------

func TestToolLoop_MaxIterations_StopsLoop(t *testing.T) {
	// Server always returns tool calls, never plain text.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, chatResponse("", []toolCallResponse{
			{ID: "call_loop", Name: "noop", Arguments: `{}`},
		}))
	}))
	defer srv.Close()

	reg := NewRegistry()
	reg.Register(&Tool{
		Name:        "noop",
		Description: "Does nothing",
		Execute: func(args map[string]any) (string, error) {
			return "done", nil
		},
	})

	p := NewFantasyProvider(FantasyConfig{
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "test-model",
		Registry: reg,
	})
	sid, _ := p.CreateSession(context.Background(), "test")

	result, err := RunToolLoop(context.Background(), p, reg, sid, "", "go", 3)
	if err != nil {
		t.Fatalf("RunToolLoop error: %v", err)
	}
	// Should return whatever the last response was (even if it had tool calls),
	// since we hit max iterations.
	if result == "" {
		t.Error("expected non-empty result after max iterations")
	}
}

// ---------------------------------------------------------------------------
// TestToolLoop_ToolError_ReturnsErrorResult
// ---------------------------------------------------------------------------

func TestToolLoop_ToolError_ReturnsErrorResult(t *testing.T) {
	var callCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = io.WriteString(w, chatResponse("", []toolCallResponse{
				{ID: "call_err", Name: "fail_tool", Arguments: `{}`},
			}))
		} else {
			// After receiving tool error, model gives final answer.
			body, _ := io.ReadAll(r.Body)
			// Verify the error was sent back.
			if !strings.Contains(string(body), "tool execution error") {
				t.Error("tool error not found in follow-up request")
			}
			_, _ = io.WriteString(w, chatResponse("Sorry, the tool failed", nil))
		}
	}))
	defer srv.Close()

	reg := NewRegistry()
	reg.Register(&Tool{
		Name:        "fail_tool",
		Description: "Always fails",
		Execute: func(args map[string]any) (string, error) {
			return "", fmt.Errorf("tool execution error: disk full")
		},
	})

	p := NewFantasyProvider(FantasyConfig{
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "test-model",
		Registry: reg,
	})
	sid, _ := p.CreateSession(context.Background(), "test")

	result, err := RunToolLoop(context.Background(), p, reg, sid, "", "do the thing", 5)
	if err != nil {
		t.Fatalf("RunToolLoop error: %v", err)
	}
	if result != "Sorry, the tool failed" {
		t.Errorf("result = %q; want %q", result, "Sorry, the tool failed")
	}
}

// ---------------------------------------------------------------------------
// TestToolLoop_UnknownTool_ReturnsError
// ---------------------------------------------------------------------------

func TestToolLoop_UnknownTool_ReturnsError(t *testing.T) {
	var callCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = io.WriteString(w, chatResponse("", []toolCallResponse{
				{ID: "call_unknown", Name: "nonexistent_tool", Arguments: `{}`},
			}))
		} else {
			// Verify the unknown tool error was passed back.
			body, _ := io.ReadAll(r.Body)
			bodyStr := string(body)
			if !strings.Contains(bodyStr, "unknown tool") {
				t.Errorf("expected 'unknown tool' in follow-up body, got: %s", bodyStr)
			}
			_, _ = io.WriteString(w, chatResponse("I don't have that tool", nil))
		}
	}))
	defer srv.Close()

	reg := NewRegistry() // empty registry

	p := NewFantasyProvider(FantasyConfig{
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "test-model",
		Registry: reg,
	})
	sid, _ := p.CreateSession(context.Background(), "test")

	result, err := RunToolLoop(context.Background(), p, reg, sid, "", "use the tool", 5)
	if err != nil {
		t.Fatalf("RunToolLoop error: %v", err)
	}
	if result != "I don't have that tool" {
		t.Errorf("result = %q; want %q", result, "I don't have that tool")
	}
}

// ---------------------------------------------------------------------------
// TestToolLoop_ContextCancelled
// ---------------------------------------------------------------------------

func TestToolLoop_ContextCancelled(t *testing.T) {
	srv := newTestServer(chatResponse("", []toolCallResponse{
		{ID: "call_1", Name: "noop", Arguments: `{}`},
	}))
	defer srv.Close()

	reg := NewRegistry()
	reg.Register(&Tool{
		Name: "noop",
		Execute: func(args map[string]any) (string, error) {
			return "ok", nil
		},
	})

	p := NewFantasyProvider(FantasyConfig{
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "test-model",
		Registry: reg,
	})
	sid, _ := p.CreateSession(context.Background(), "test")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := RunToolLoop(ctx, p, reg, sid, "", "go", 10)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

// ---------------------------------------------------------------------------
// Helpers: verify tool definitions are serialized correctly
// ---------------------------------------------------------------------------

func TestToolDefsToOpenAI(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Tool{
		Name:        "search",
		Description: "Search the web",
		Parameters: []ToolParam{
			{Name: "query", Type: "string", Description: "Search query", Required: true},
			{Name: "limit", Type: "number", Description: "Max results", Required: false},
		},
	})

	defs := toolDefsFromRegistry(reg)
	if len(defs) != 1 {
		t.Fatalf("expected 1 tool def, got %d", len(defs))
	}

	b, _ := json.Marshal(defs[0])
	var m map[string]any
	_ = json.Unmarshal(b, &m)

	if m["type"] != "function" {
		t.Errorf("type = %v; want 'function'", m["type"])
	}

	fn, ok := m["function"].(map[string]any)
	if !ok {
		t.Fatal("missing 'function' key")
	}
	if fn["name"] != "search" {
		t.Errorf("name = %v; want 'search'", fn["name"])
	}

	params, ok := fn["parameters"].(map[string]any)
	if !ok {
		t.Fatal("missing 'parameters' key")
	}
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing 'properties' key")
	}
	if _, ok := props["query"]; !ok {
		t.Error("missing 'query' property")
	}

	required, ok := params["required"].([]any)
	if !ok {
		t.Fatal("missing 'required' key")
	}
	if len(required) != 1 || required[0] != "query" {
		t.Errorf("required = %v; want [query]", required)
	}
}
