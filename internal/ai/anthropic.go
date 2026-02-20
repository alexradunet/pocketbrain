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

// Compile-time check that AnthropicProvider satisfies core.Provider.
var _ core.Provider = (*AnthropicProvider)(nil)

const anthropicAPIBase = "https://api.anthropic.com"
const anthropicVersion = "2023-06-01"
const anthropicMaxTokens = 4096
const anthropicMaxToolIterations = 10

// AnthropicConfig holds settings for the Anthropic provider.
type AnthropicConfig struct {
	APIKey   string
	Model    string // e.g. "claude-sonnet-4-20250514"
	Registry *Registry
	// BaseURL overrides the default API base (used in tests).
	BaseURL string
}

// AnthropicProvider is an HTTP-based LLM provider that speaks the Anthropic
// Messages API natively. It stores conversation history in memory per session
// and supports tool_use responses.
type AnthropicProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
	model   string
	registry *Registry

	mu       sync.Mutex
	sessions map[string][]anthropicMessage
}

// NewAnthropicProvider creates an AnthropicProvider from the given config.
func NewAnthropicProvider(cfg AnthropicConfig) *AnthropicProvider {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = anthropicAPIBase
	}
	reg := cfg.Registry
	if reg == nil {
		reg = NewRegistry()
	}
	return &AnthropicProvider{
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		baseURL:  base,
		apiKey:   cfg.APIKey,
		model:    cfg.Model,
		registry: reg,
		sessions: make(map[string][]anthropicMessage),
	}
}

// ---------------------------------------------------------------------------
// core.Provider implementation
// ---------------------------------------------------------------------------

// CreateSession generates a new unique session ID.
func (p *AnthropicProvider) CreateSession(_ context.Context, title string) (string, error) {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := fmt.Sprintf("anthropic-%s-%s", sanitizeTitle(title), hex.EncodeToString(b))

	p.mu.Lock()
	p.sessions[id] = nil
	p.mu.Unlock()

	return id, nil
}

// SendMessage sends userText to the model within the given session and returns
// the assistant reply. The system prompt is sent as a top-level field per the
// Anthropic Messages API spec. Tool calls are executed up to
// anthropicMaxToolIterations times before returning.
func (p *AnthropicProvider) SendMessage(ctx context.Context, sessionID, system, userText string) (string, error) {
	// Append user message to history.
	p.appendMessage(sessionID, anthropicMessage{
		Role:    "user",
		Content: []anthropicContentBlock{{Type: "text", Text: userText}},
	})

	tools := anthropicToolDefsFromRegistry(p.registry)

	for i := 0; i < anthropicMaxToolIterations; i++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		history := p.getHistory(sessionID)
		resp, err := p.sendMessages(ctx, system, history, tools)
		if err != nil {
			return "", fmt.Errorf("anthropic: send messages: %w", err)
		}

		// Collect text and tool_use blocks from the response.
		var textContent string
		var toolUseBlocks []anthropicContentBlock
		for _, block := range resp.Content {
			switch block.Type {
			case "text":
				textContent = block.Text
			case "tool_use":
				toolUseBlocks = append(toolUseBlocks, block)
			}
		}

		// Store the assistant message (with all content blocks) in history.
		p.appendMessage(sessionID, anthropicMessage{
			Role:    "assistant",
			Content: resp.Content,
		})

		// If no tool_use blocks, we are done.
		if len(toolUseBlocks) == 0 {
			return textContent, nil
		}

		// Execute each tool and collect results into a single user message.
		var resultBlocks []anthropicContentBlock
		for _, tb := range toolUseBlocks {
			result := executeAnthropicToolCall(p.registry, tb)
			resultBlocks = append(resultBlocks, anthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: tb.ID,
				Content:   result,
			})
		}
		p.appendMessage(sessionID, anthropicMessage{
			Role:    "user",
			Content: resultBlocks,
		})
	}

	// Max iterations reached — return last non-empty assistant text.
	hist := p.getHistory(sessionID)
	for i := len(hist) - 1; i >= 0; i-- {
		if hist[i].Role == "assistant" {
			for _, block := range hist[i].Content {
				if block.Type == "text" && block.Text != "" {
					return block.Text, nil
				}
			}
		}
	}
	return "max tool loop iterations reached", nil
}

// SendMessageNoReply injects userText into the session history.
// The reply is stored in history but not returned.
func (p *AnthropicProvider) SendMessageNoReply(ctx context.Context, sessionID, userText string) error {
	_, err := p.SendMessage(ctx, sessionID, "", userText)
	return err
}

// RecentContext returns a condensed string of the last few messages in the
// session, suitable for context injection.
func (p *AnthropicProvider) RecentContext(_ context.Context, sessionID string) (string, error) {
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
		// Skip pure tool-result user messages for context summary.
		if m.Role == "user" && len(m.Content) > 0 && m.Content[0].Type == "tool_result" {
			continue
		}
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		for _, block := range m.Content {
			if block.Type == "text" {
				sb.WriteString(block.Text)
			}
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// ---------------------------------------------------------------------------
// Internal: HTTP transport
// ---------------------------------------------------------------------------

func (p *AnthropicProvider) sendMessages(
	ctx context.Context,
	system string,
	messages []anthropicMessage,
	tools []anthropicTool,
) (*anthropicResponse, error) {
	reqBody := anthropicRequest{
		Model:     p.model,
		MaxTokens: anthropicMaxTokens,
		Messages:  messages,
	}
	if system != "" {
		reqBody.System = system
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

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

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// ---------------------------------------------------------------------------
// Internal: session history helpers
// ---------------------------------------------------------------------------

func (p *AnthropicProvider) appendMessage(sessionID string, msg anthropicMessage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessions[sessionID] = append(p.sessions[sessionID], msg)
}

func (p *AnthropicProvider) getHistory(sessionID string) []anthropicMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	hist := p.sessions[sessionID]
	cp := make([]anthropicMessage, len(hist))
	copy(cp, hist)
	return cp
}

// ---------------------------------------------------------------------------
// Internal: tool execution
// ---------------------------------------------------------------------------

// executeAnthropicToolCall runs a tool_use block and returns the result string.
func executeAnthropicToolCall(reg *Registry, block anthropicContentBlock) string {
	tool, ok := reg.Get(block.Name)
	if !ok {
		return fmt.Sprintf("Error: unknown tool %q", block.Name)
	}

	args := block.Input
	if args == nil {
		args = make(map[string]any)
	}

	result, err := tool.Execute(args)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return result
}

// ---------------------------------------------------------------------------
// Internal: tool definition conversion
// ---------------------------------------------------------------------------

// anthropicToolDefsFromRegistry converts Registry tools into the Anthropic
// tool format (input_schema instead of parameters).
func anthropicToolDefsFromRegistry(reg *Registry) []anthropicTool {
	if reg == nil {
		return nil
	}
	all := reg.All()
	if len(all) == 0 {
		return nil
	}

	defs := make([]anthropicTool, 0, len(all))
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

		defs = append(defs, anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: anthropicSchema{
				Type:       "object",
				Properties: props,
				Required:   required,
			},
		})
	}
	return defs
}

// ---------------------------------------------------------------------------
// Wire types: Anthropic Messages API
// ---------------------------------------------------------------------------

type anthropicMessage struct {
	Role    string                 `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`          // tool_use
	Name      string         `json:"name,omitempty"`        // tool_use
	Input     map[string]any `json:"input,omitempty"`       // tool_use
	ToolUseID string         `json:"tool_use_id,omitempty"` // tool_result
	Content   string         `json:"content,omitempty"`     // tool_result (string form)
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicResponse struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Role       string                 `json:"role"`
	Content    []anthropicContentBlock `json:"content"`
	Model      string                 `json:"model"`
	StopReason string                 `json:"stop_reason"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema anthropicSchema `json:"input_schema"`
}

type anthropicSchema struct {
	Type       string                       `json:"type"`
	Properties map[string]map[string]string `json:"properties"`
	Required   []string                     `json:"required,omitempty"`
}
