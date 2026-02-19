package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helper: create a test server that returns canned OpenAI-compatible responses
// ---------------------------------------------------------------------------

// chatResponse builds a minimal OpenAI chat completion JSON response.
func chatResponse(content string, toolCalls []toolCallResponse) string {
	msg := map[string]any{
		"role":    "assistant",
		"content": content,
	}
	if len(toolCalls) > 0 {
		tc := make([]map[string]any, len(toolCalls))
		for i, c := range toolCalls {
			tc[i] = map[string]any{
				"id":   c.ID,
				"type": "function",
				"function": map[string]any{
					"name":      c.Name,
					"arguments": c.Arguments,
				},
			}
		}
		msg["tool_calls"] = tc
	}

	resp := map[string]any{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"model":   "test-model",
		"choices": []map[string]any{{"index": 0, "message": msg, "finish_reason": "stop"}},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

type toolCallResponse struct {
	ID        string
	Name      string
	Arguments string
}

// newTestServer creates an httptest server that returns the given body for every POST.
func newTestServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	}))
}

// newTestServerFunc creates an httptest server with a custom handler.
func newTestServerFunc(fn func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(fn))
}

// ---------------------------------------------------------------------------
// CreateSession
// ---------------------------------------------------------------------------

func TestFantasyProvider_CreateSession_GeneratesID(t *testing.T) {
	srv := newTestServer(chatResponse("ok", nil))
	defer srv.Close()

	p := NewFantasyProvider(FantasyConfig{
		BaseURL: srv.URL,
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
// SendMessage – stores user message
// ---------------------------------------------------------------------------

func TestFantasyProvider_SendMessage_StoresUserMessage(t *testing.T) {
	var receivedBody []byte
	srv := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, chatResponse("hello back", nil))
	})
	defer srv.Close()

	p := NewFantasyProvider(FantasyConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "test")
	_, err := p.SendMessage(context.Background(), sid, "you are helpful", "hi there")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}

	// Verify the request body contains the user message.
	var req chatCompletionRequest
	if err := json.Unmarshal(receivedBody, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	foundUser := false
	for _, m := range req.Messages {
		if m.Role == "user" && m.Content == "hi there" {
			foundUser = true
		}
	}
	if !foundUser {
		t.Error("user message 'hi there' not found in request messages")
	}
}

// ---------------------------------------------------------------------------
// SendMessage – stores assistant reply
// ---------------------------------------------------------------------------

func TestFantasyProvider_SendMessage_StoresAssistantReply(t *testing.T) {
	srv := newTestServer(chatResponse("I am the assistant", nil))
	defer srv.Close()

	p := NewFantasyProvider(FantasyConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "test")
	reply, err := p.SendMessage(context.Background(), sid, "", "hello")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if reply != "I am the assistant" {
		t.Errorf("reply = %q; want %q", reply, "I am the assistant")
	}

	// The assistant reply should now be stored in history.
	// Send a second message and check that history includes the assistant reply.
	var receivedBody []byte
	srv.Close()
	srv2 := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, chatResponse("second reply", nil))
	})
	defer srv2.Close()
	p.baseURL = srv2.URL

	_, err = p.SendMessage(context.Background(), sid, "", "follow up")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}

	var req chatCompletionRequest
	if err := json.Unmarshal(receivedBody, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	foundAssistant := false
	for _, m := range req.Messages {
		if m.Role == "assistant" && m.Content == "I am the assistant" {
			foundAssistant = true
		}
	}
	if !foundAssistant {
		t.Error("previous assistant reply not found in follow-up request messages")
	}
}

// ---------------------------------------------------------------------------
// SendMessage – includes history
// ---------------------------------------------------------------------------

func TestFantasyProvider_SendMessage_IncludesHistory(t *testing.T) {
	callCount := 0
	srv := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, chatResponse("reply-"+strings.Repeat("x", callCount), nil))
	})
	defer srv.Close()

	p := NewFantasyProvider(FantasyConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "test")

	// Send 3 messages to build history.
	for i := 0; i < 3; i++ {
		_, err := p.SendMessage(context.Background(), sid, "sys", "msg")
		if err != nil {
			t.Fatalf("SendMessage #%d error: %v", i, err)
		}
	}

	// Verify internal history has all messages (3 user + 3 assistant = 6).
	p.mu.Lock()
	histLen := len(p.sessions[sid])
	p.mu.Unlock()

	if histLen != 6 {
		t.Errorf("history length = %d; want 6", histLen)
	}
}

// ---------------------------------------------------------------------------
// RecentContext – returns last N messages
// ---------------------------------------------------------------------------

func TestFantasyProvider_RecentContext_ReturnsLastN(t *testing.T) {
	callCount := 0
	srv := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		reply := ""
		switch callCount {
		case 1:
			reply = "first reply"
		case 2:
			reply = "second reply"
		case 3:
			reply = "third reply"
		default:
			reply = "extra"
		}
		_, _ = io.WriteString(w, chatResponse(reply, nil))
	})
	defer srv.Close()

	p := NewFantasyProvider(FantasyConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "test")

	// Build conversation history.
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

	// Should contain recent messages.
	if !strings.Contains(ctx, "third reply") {
		t.Error("RecentContext missing 'third reply'")
	}
	if !strings.Contains(ctx, "what's up") {
		t.Error("RecentContext missing 'what's up'")
	}
}

// ---------------------------------------------------------------------------
// RecentContext – empty session
// ---------------------------------------------------------------------------

func TestFantasyProvider_RecentContext_EmptySession(t *testing.T) {
	p := NewFantasyProvider(FantasyConfig{
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
// Close – no error
// ---------------------------------------------------------------------------

func TestFantasyProvider_Close_NoError(t *testing.T) {
	p := NewFantasyProvider(FantasyConfig{
		BaseURL: "http://unused",
		APIKey:  "test-key",
		Model:   "test-model",
	})

	if err := p.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SendMessageNoReply – stores message without expecting reply content
// ---------------------------------------------------------------------------

func TestFantasyProvider_SendMessageNoReply_StoresMessage(t *testing.T) {
	srv := newTestServer(chatResponse("ack", nil))
	defer srv.Close()

	p := NewFantasyProvider(FantasyConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "test")
	err := p.SendMessageNoReply(context.Background(), sid, "injected context")
	if err != nil {
		t.Fatalf("SendMessageNoReply error: %v", err)
	}

	// The injected message should appear in history.
	p.mu.Lock()
	histLen := len(p.sessions[sid])
	p.mu.Unlock()

	// user message + assistant ack = 2
	if histLen != 2 {
		t.Errorf("history length = %d; want 2", histLen)
	}
}

// ---------------------------------------------------------------------------
// SendMessage – API error
// ---------------------------------------------------------------------------

func TestFantasyProvider_SendMessage_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error": {"message": "server error"}}`)
	}))
	defer srv.Close()

	p := NewFantasyProvider(FantasyConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})

	sid, _ := p.CreateSession(context.Background(), "test")
	_, err := p.SendMessage(context.Background(), sid, "", "hi")
	if err == nil {
		t.Fatal("expected error from API returning 500, got nil")
	}
}

// ---------------------------------------------------------------------------
// SendMessage – includes tools in request when provider has tools
// ---------------------------------------------------------------------------

func TestFantasyProvider_SendMessage_IncludesToolDefinitions(t *testing.T) {
	var receivedBody []byte
	srv := newTestServerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, chatResponse("ok", nil))
	})
	defer srv.Close()

	reg := NewRegistry()
	reg.Register(&Tool{
		Name:        "get_weather",
		Description: "Get weather for a location",
		Parameters: []ToolParam{
			{Name: "location", Type: "string", Description: "City name", Required: true},
		},
	})

	p := NewFantasyProvider(FantasyConfig{
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "test-model",
		Registry: reg,
	})

	sid, _ := p.CreateSession(context.Background(), "test")
	_, err := p.SendMessage(context.Background(), sid, "", "what's the weather?")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}

	// Verify tools were included in the request.
	var req map[string]any
	if err := json.Unmarshal(receivedBody, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tools, ok := req["tools"]
	if !ok {
		t.Fatal("request missing 'tools' field")
	}
	toolList, ok := tools.([]any)
	if !ok || len(toolList) == 0 {
		t.Fatal("tools field is empty or not an array")
	}
}
