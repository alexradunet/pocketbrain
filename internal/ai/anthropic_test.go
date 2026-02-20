package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers: canned Anthropic-format response builders
// ---------------------------------------------------------------------------

// anthropicTextResponse builds a minimal Anthropic Messages API JSON response
// containing a single text content block.
func anthropicTextResponse(text string) string {
	resp := map[string]any{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       "test-model",
		"stop_reason": "end_turn",
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// anthropicToolUseResponse builds a response that contains a tool_use block
// followed by a text block.
func anthropicToolUseResponse(id, name string, input map[string]any) string {
	resp := map[string]any{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       "test-model",
		"stop_reason": "tool_use",
		"content": []map[string]any{
			{"type": "text", "text": "Let me use a tool."},
			{"type": "tool_use", "id": id, "name": name, "input": input},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// newAnthropicTestServer creates an httptest server returning the given body.
func newAnthropicTestServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))
}

// newAnthropicTestServerFunc creates an httptest server with a custom handler.
func newAnthropicTestServerFunc(fn func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(fn))
}

// ---------------------------------------------------------------------------
// TestAnthropicProvider_CreateSession_GeneratesID
// ---------------------------------------------------------------------------

func TestAnthropicProvider_CreateSession_GeneratesID(t *testing.T) {
	p := NewAnthropicProvider(AnthropicConfig{
		BaseURL: "http://unused",
		APIKey:  "test-key",
		Model:   "test-model",
	})

	id, err := p.CreateSession(context.Background(), "my-session")
	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}
	if id == "" {
		t.Fatal("CreateSession returned empty ID")
	}

	// Each call should return a unique ID.
	id2, err := p.CreateSession(context.Background(), "other")
	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}
	if id == id2 {
		t.Errorf("expected unique IDs, got %q twice", id)
	}
}

// ---------------------------------------------------------------------------
// TestAnthropicProvider_SendMessage_BasicText
// ---------------------------------------------------------------------------

func TestAnthropicProvider_SendMessage_BasicText(t *testing.T) {
	srv := newAnthropicTestServer(anthropicTextResponse("hello from claude"))
	defer srv.Close()

	p := NewAnthropicProvider(AnthropicConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "test")
	reply, err := p.SendMessage(context.Background(), sid, "", "hi")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if reply != "hello from claude" {
		t.Errorf("reply = %q; want %q", reply, "hello from claude")
	}
}

// ---------------------------------------------------------------------------
// TestAnthropicProvider_SendMessage_IncludesSystemPrompt
// ---------------------------------------------------------------------------

func TestAnthropicProvider_SendMessage_IncludesSystemPrompt(t *testing.T) {
	var receivedBody []byte
	srv := newAnthropicTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, anthropicTextResponse("ok"))
	})
	defer srv.Close()

	p := NewAnthropicProvider(AnthropicConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "test")
	_, err := p.SendMessage(context.Background(), sid, "you are a helpful assistant", "hello")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}

	// Verify system is a top-level field, not a message.
	var req map[string]any
	if err := json.Unmarshal(receivedBody, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	sys, ok := req["system"]
	if !ok {
		t.Fatal("request missing top-level 'system' field")
	}
	if sys != "you are a helpful assistant" {
		t.Errorf("system = %q; want %q", sys, "you are a helpful assistant")
	}

	// Ensure system is NOT inside the messages array.
	msgs, _ := req["messages"].([]any)
	for _, m := range msgs {
		msg, _ := m.(map[string]any)
		if msg["role"] == "system" {
			t.Error("system prompt should be top-level, not a message with role=system")
		}
	}
}

// ---------------------------------------------------------------------------
// TestAnthropicProvider_SendMessage_SetsCorrectHeaders
// ---------------------------------------------------------------------------

func TestAnthropicProvider_SendMessage_SetsCorrectHeaders(t *testing.T) {
	var gotAPIKey, gotVersion, gotAuth string
	srv := newAnthropicTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, anthropicTextResponse("ok"))
	})
	defer srv.Close()

	p := NewAnthropicProvider(AnthropicConfig{
		BaseURL: srv.URL,
		APIKey:  "sk-ant-test123",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "test")
	_, err := p.SendMessage(context.Background(), sid, "", "hello")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}

	if gotAPIKey != "sk-ant-test123" {
		t.Errorf("x-api-key = %q; want %q", gotAPIKey, "sk-ant-test123")
	}
	if gotVersion != anthropicVersion {
		t.Errorf("anthropic-version = %q; want %q", gotVersion, anthropicVersion)
	}
	if gotAuth != "" {
		t.Errorf("Authorization header should not be set, got %q", gotAuth)
	}
}

// ---------------------------------------------------------------------------
// TestAnthropicProvider_SendMessage_ToolExecution
// ---------------------------------------------------------------------------

func TestAnthropicProvider_SendMessage_ToolExecution(t *testing.T) {
	callCount := 0
	srv := newAnthropicTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			_, _ = io.WriteString(w, anthropicToolUseResponse(
				"toolu_1", "echo_tool", map[string]any{"msg": "world"},
			))
		} else {
			_, _ = io.WriteString(w, anthropicTextResponse("tool result was: echo:world"))
		}
	})
	defer srv.Close()

	reg := NewRegistry()
	reg.Register(&Tool{
		Name:        "echo_tool",
		Description: "echoes the msg",
		Parameters: []ToolParam{
			{Name: "msg", Type: "string", Description: "message", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			msg, _ := args["msg"].(string)
			return "echo:" + msg, nil
		},
	})

	p := NewAnthropicProvider(AnthropicConfig{
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "test-model",
		Registry: reg,
	})

	sid, _ := p.CreateSession(context.Background(), "test")
	reply, err := p.SendMessage(context.Background(), sid, "", "run the tool")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if reply != "tool result was: echo:world" {
		t.Errorf("reply = %q; want %q", reply, "tool result was: echo:world")
	}
	if callCount != 2 {
		t.Errorf("API called %d times; want 2", callCount)
	}
}

// ---------------------------------------------------------------------------
// TestAnthropicProvider_SendMessage_MultipleToolCalls
// ---------------------------------------------------------------------------

func TestAnthropicProvider_SendMessage_MultipleToolCalls(t *testing.T) {
	callCount := 0
	srv := newAnthropicTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// Two tool_use blocks in one response.
			resp := map[string]any{
				"id":          "msg_multi",
				"type":        "message",
				"role":        "assistant",
				"model":       "test-model",
				"stop_reason": "tool_use",
				"content": []map[string]any{
					{"type": "tool_use", "id": "toolu_a", "name": "add_tool", "input": map[string]any{"a": 1, "b": 2}},
					{"type": "tool_use", "id": "toolu_b", "name": "add_tool", "input": map[string]any{"a": 3, "b": 4}},
				},
			}
			b, _ := json.Marshal(resp)
			_, _ = w.Write(b)
		} else {
			_, _ = io.WriteString(w, anthropicTextResponse("results: 3 and 7"))
		}
	})
	defer srv.Close()

	reg := NewRegistry()
	reg.Register(&Tool{
		Name:        "add_tool",
		Description: "adds two numbers",
		Parameters: []ToolParam{
			{Name: "a", Type: "number", Description: "first", Required: true},
			{Name: "b", Type: "number", Description: "second", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			// JSON numbers decode as float64.
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			return fmt.Sprintf("%g", a+b), nil
		},
	})

	p := NewAnthropicProvider(AnthropicConfig{
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "test-model",
		Registry: reg,
	})

	sid, _ := p.CreateSession(context.Background(), "test")
	reply, err := p.SendMessage(context.Background(), sid, "", "add some numbers")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if reply != "results: 3 and 7" {
		t.Errorf("reply = %q; want %q", reply, "results: 3 and 7")
	}
	if callCount != 2 {
		t.Errorf("API called %d times; want 2", callCount)
	}

	// History: user, assistant(2 tool_use), user(2 tool_results), assistant(final) = 4
	p.mu.Lock()
	histLen := len(p.sessions[sid])
	p.mu.Unlock()
	if histLen != 4 {
		t.Errorf("history length = %d; want 4", histLen)
	}
}

// ---------------------------------------------------------------------------
// TestAnthropicProvider_SendMessage_StoresHistory
// ---------------------------------------------------------------------------

func TestAnthropicProvider_SendMessage_StoresHistory(t *testing.T) {
	callCount := 0
	srv := newAnthropicTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		switch callCount {
		case 1:
			_, _ = io.WriteString(w, anthropicTextResponse("first reply"))
		default:
			_, _ = io.WriteString(w, anthropicTextResponse("second reply"))
		}
	})
	defer srv.Close()

	p := NewAnthropicProvider(AnthropicConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "test")

	_, err := p.SendMessage(context.Background(), sid, "", "first message")
	if err != nil {
		t.Fatalf("SendMessage 1 error: %v", err)
	}

	// Capture the second request body to verify history is included.
	var secondBody []byte
	srv.Close()
	srv2 := newAnthropicTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, anthropicTextResponse("second reply"))
	})
	defer srv2.Close()
	p.baseURL = srv2.URL

	_, err = p.SendMessage(context.Background(), sid, "", "second message")
	if err != nil {
		t.Fatalf("SendMessage 2 error: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(secondBody, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	msgs, _ := req["messages"].([]any)
	// Should have: user(first), assistant(first reply), user(second) = 3
	if len(msgs) != 3 {
		t.Errorf("messages count = %d; want 3", len(msgs))
	}
}

// ---------------------------------------------------------------------------
// TestAnthropicProvider_RecentContext_ReturnsLastN
// ---------------------------------------------------------------------------

func TestAnthropicProvider_RecentContext_ReturnsLastN(t *testing.T) {
	callCount := 0
	srv := newAnthropicTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		switch callCount {
		case 1:
			_, _ = io.WriteString(w, anthropicTextResponse("first reply"))
		case 2:
			_, _ = io.WriteString(w, anthropicTextResponse("second reply"))
		default:
			_, _ = io.WriteString(w, anthropicTextResponse("third reply"))
		}
	})
	defer srv.Close()

	p := NewAnthropicProvider(AnthropicConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "test")
	for _, msg := range []string{"hello", "how are you", "what's up"} {
		_, err := p.SendMessage(context.Background(), sid, "", msg)
		if err != nil {
			t.Fatalf("SendMessage error: %v", err)
		}
	}

	ctx, err := p.RecentContext(context.Background(), sid)
	if err != nil {
		t.Fatalf("RecentContext error: %v", err)
	}
	if ctx == "" {
		t.Fatal("RecentContext returned empty string")
	}
	if !strings.Contains(ctx, "third reply") {
		t.Errorf("RecentContext missing 'third reply'; got: %q", ctx)
	}
	if !strings.Contains(ctx, "what's up") {
		t.Errorf("RecentContext missing \"what's up\"; got: %q", ctx)
	}
}

// ---------------------------------------------------------------------------
// TestAnthropicProvider_RecentContext_EmptySession
// ---------------------------------------------------------------------------

func TestAnthropicProvider_RecentContext_EmptySession(t *testing.T) {
	p := NewAnthropicProvider(AnthropicConfig{
		BaseURL: "http://unused",
		APIKey:  "test-key",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "empty")
	ctx, err := p.RecentContext(context.Background(), sid)
	if err != nil {
		t.Fatalf("RecentContext error: %v", err)
	}
	if ctx != "" {
		t.Errorf("RecentContext = %q; want empty string for empty session", ctx)
	}
}

// ---------------------------------------------------------------------------
// TestAnthropicProvider_SendMessage_APIError
// ---------------------------------------------------------------------------

func TestAnthropicProvider_SendMessage_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"api_error","message":"server error"}}`)
	}))
	defer srv.Close()

	p := NewAnthropicProvider(AnthropicConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "test")
	_, err := p.SendMessage(context.Background(), sid, "", "hi")
	if err == nil {
		t.Fatal("expected error from API returning 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status 500, got: %v", err)
	}
}
