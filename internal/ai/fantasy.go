package ai

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// Compile-time check that FantasyProvider satisfies core.Provider.
var _ core.Provider = (*FantasyProvider)(nil)

// maxToolIterations caps the number of tool-call rounds in SendMessage.
const maxToolIterations = 10

// FantasyConfig holds settings for the Fantasy AI provider.
type FantasyConfig struct {
	BaseURL  string // e.g. "https://api.openai.com" or any compatible endpoint
	APIKey   string
	Model    string // e.g. "gpt-4o", "claude-sonnet-4-20250514"
	Registry *Registry
}

// FantasyProvider is an HTTP-based LLM provider that speaks the OpenAI-
// compatible chat completions API. It stores conversation history in memory
// per session and supports function-calling (tool_use) responses.
type FantasyProvider struct {
	client   *http.Client
	baseURL  string
	apiKey   string
	model    string
	registry *Registry

	mu       sync.Mutex
	sessions map[string][]chatMessage // sessionID -> message history
}

// NewFantasyProvider creates a FantasyProvider from the given config.
func NewFantasyProvider(cfg FantasyConfig) *FantasyProvider {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	reg := cfg.Registry
	if reg == nil {
		reg = NewRegistry()
	}
	return &FantasyProvider{
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		baseURL:  baseURL,
		apiKey:   cfg.APIKey,
		model:    cfg.Model,
		registry: reg,
		sessions: make(map[string][]chatMessage),
	}
}

// ---------------------------------------------------------------------------
// core.Provider implementation
// ---------------------------------------------------------------------------

// CreateSession generates a new unique session ID. The title is stored as
// metadata but the session is otherwise empty until messages are sent.
func (p *FantasyProvider) CreateSession(_ context.Context, title string) (string, error) {
	id := generateSessionID(title)

	p.mu.Lock()
	p.sessions[id] = nil
	p.mu.Unlock()

	return id, nil
}

// SendMessage sends userText to the model within the given session and returns
// the assistant reply. The system prompt overrides the session system prompt
// when non-empty. If the model responds with tool calls and the registry has
// tools, the tool loop executes up to maxToolIterations times before returning.
func (p *FantasyProvider) SendMessage(ctx context.Context, sessionID, system, userText string) (string, error) {
	// Append user message to history.
	p.appendMessage(sessionID, chatMessage{Role: "user", Content: userText})

	// Build tool definitions once.
	tools := toolDefsFromRegistry(p.registry)
	hasTools := len(tools) > 0

	for i := 0; i < maxToolIterations; i++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		// Build request messages from current history.
		msgs := p.buildMessages(sessionID, system)

		// Call the API.
		resp, err := p.chatCompletion(ctx, msgs, tools)
		if err != nil {
			return "", fmt.Errorf("fantasy: chat completion: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("fantasy: no choices in response")
		}

		choice := resp.Choices[0]

		// Store the full assistant message (including any tool_calls).
		assistantMsg := chatMessage{
			Role:      "assistant",
			Content:   choice.Message.Content,
			ToolCalls: choice.Message.ToolCalls,
		}
		p.appendMessage(sessionID, assistantMsg)

		// If no tool calls, or no tools registered, return content immediately.
		if !hasTools || len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content, nil
		}

		// Execute each tool call and append results to history.
		for _, tc := range choice.Message.ToolCalls {
			result := executeToolCall(p.registry, tc)
			p.appendMessage(sessionID, chatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	// Max iterations reached — return last non-empty assistant content.
	hist := p.getHistory(sessionID)
	for i := len(hist) - 1; i >= 0; i-- {
		if hist[i].Role == "assistant" && hist[i].Content != "" {
			return hist[i].Content, nil
		}
	}
	return "max tool loop iterations reached", nil
}

// SendMessageNoReply injects userText into the session history by sending it
// to the model. The reply is stored in history but not returned.
func (p *FantasyProvider) SendMessageNoReply(ctx context.Context, sessionID, userText string) error {
	_, err := p.SendMessage(ctx, sessionID, "", userText)
	return err
}

// RecentContext returns a condensed string of the last few messages in the
// session, suitable for context injection.
func (p *FantasyProvider) RecentContext(_ context.Context, sessionID string) (string, error) {
	const maxMessages = 10

	p.mu.Lock()
	hist, ok := p.sessions[sessionID]
	p.mu.Unlock()

	if !ok || len(hist) == 0 {
		return "", nil
	}

	start := 0
	if len(hist) > maxMessages {
		start = len(hist) - maxMessages
	}
	recent := hist[start:]

	var sb strings.Builder
	for _, m := range recent {
		if m.Role == "tool" {
			continue // skip tool result messages for context summary
		}
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// Close is a no-op for the HTTP-based provider; there are no persistent
// connections to tear down.
func (p *FantasyProvider) Close() error {
	return nil
}

// ---------------------------------------------------------------------------
// SendMessageRaw sends a pre-built message list (used by the tool loop).
// It does NOT touch session history; the caller is responsible for that.
// ---------------------------------------------------------------------------

func (p *FantasyProvider) sendRaw(ctx context.Context, msgs []chatMessage, tools []toolDef) (*chatCompletionResponse, error) {
	return p.chatCompletion(ctx, msgs, tools)
}

// ---------------------------------------------------------------------------
// Internal: HTTP transport
// ---------------------------------------------------------------------------

func (p *FantasyProvider) chatCompletion(ctx context.Context, msgs []chatMessage, tools []toolDef) (*chatCompletionResponse, error) {
	reqBody := chatCompletionRequest{
		Model:    p.model,
		Messages: msgs,
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error %d: %s", httpResp.StatusCode, truncateForLog(string(respBody), 500))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// ---------------------------------------------------------------------------
// Internal: session history helpers
// ---------------------------------------------------------------------------

func (p *FantasyProvider) appendMessage(sessionID string, msg chatMessage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessions[sessionID] = append(p.sessions[sessionID], msg)
}

func (p *FantasyProvider) appendMessages(sessionID string, msgs []chatMessage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessions[sessionID] = append(p.sessions[sessionID], msgs...)
}

func (p *FantasyProvider) getHistory(sessionID string) []chatMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	hist := p.sessions[sessionID]
	cp := make([]chatMessage, len(hist))
	copy(cp, hist)
	return cp
}

func (p *FantasyProvider) buildMessages(sessionID, system string) []chatMessage {
	var msgs []chatMessage
	if system != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: system})
	}
	msgs = append(msgs, p.getHistory(sessionID)...)
	return msgs
}

// ---------------------------------------------------------------------------
// Internal: tool definition conversion
// ---------------------------------------------------------------------------

// toolDefsFromRegistry converts the Registry tools into the OpenAI function
// calling format.
func toolDefsFromRegistry(reg *Registry) []toolDef {
	if reg == nil {
		return nil
	}
	all := reg.All()
	if len(all) == 0 {
		return nil
	}

	defs := make([]toolDef, 0, len(all))
	for _, tool := range all {
		props := make(map[string]map[string]string)
		var required []string
		for _, param := range tool.Parameters {
			props[param.Name] = map[string]string{
				"type":        param.Type,
				"description": param.Description,
			}
			if param.Required {
				required = append(required, param.Name)
			}
		}

		defs = append(defs, toolDef{
			Type: "function",
			Function: toolFuncDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters: toolParamsDef{
					Type:       "object",
					Properties: props,
					Required:   required,
				},
			},
		})
	}
	return defs
}

// ---------------------------------------------------------------------------
// Internal: session ID generation
// ---------------------------------------------------------------------------

func generateSessionID(title string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("fantasy-%s-%s", sanitizeTitle(title), hex.EncodeToString(b))
}

func sanitizeTitle(title string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", "\\", "-")
	s := r.Replace(strings.ToLower(strings.TrimSpace(title)))
	if len(s) > 32 {
		s = s[:32]
	}
	return s
}

// ---------------------------------------------------------------------------
// Wire types: OpenAI-compatible chat completions API
// ---------------------------------------------------------------------------

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolCallFunc `json:"function"`
}

type toolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded arguments
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []toolDef     `json:"tools,omitempty"`
}

type chatCompletionResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Model   string              `json:"model"`
	Choices []chatCompletionChoice `json:"choices"`
}

type chatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type toolDef struct {
	Type     string      `json:"type"`
	Function toolFuncDef `json:"function"`
}

type toolFuncDef struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Parameters  toolParamsDef `json:"parameters"`
}

type toolParamsDef struct {
	Type       string                       `json:"type"`
	Properties map[string]map[string]string `json:"properties"`
	Required   []string                     `json:"required,omitempty"`
}
