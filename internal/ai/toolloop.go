package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

// RunToolLoop implements the send → tool_use → tool_result → send loop.
// It sends the user message, and if the model responds with tool calls, it
// executes them and feeds the results back. The loop stops when the model
// returns a plain text response (no tool calls) or maxIterations is reached.
func RunToolLoop(
	ctx context.Context,
	p *FantasyProvider,
	reg *Registry,
	sessionID string,
	system string,
	userText string,
	maxIterations int,
) (string, error) {
	// Append the initial user message to session history.
	p.appendMessage(sessionID, chatMessage{Role: "user", Content: userText})

	tools := toolDefsFromRegistry(reg)

	for i := 0; i < maxIterations; i++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		msgs := p.buildMessages(sessionID, system)

		resp, err := p.sendRaw(ctx, msgs, tools)
		if err != nil {
			return "", fmt.Errorf("tool loop iteration %d: %w", i, err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("tool loop: no choices in response")
		}

		choice := resp.Choices[0]

		// Store the assistant message in history.
		assistantMsg := chatMessage{
			Role:      "assistant",
			Content:   choice.Message.Content,
			ToolCalls: choice.Message.ToolCalls,
		}
		p.appendMessage(sessionID, assistantMsg)

		// If no tool calls, we're done.
		if len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content, nil
		}

		// Execute each tool call and append results to history.
		for _, tc := range choice.Message.ToolCalls {
			result := executeToolCall(reg, tc)
			p.appendMessage(sessionID, chatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	// Max iterations reached — return whatever content we have from the last response.
	hist := p.getHistory(sessionID)
	for i := len(hist) - 1; i >= 0; i-- {
		if hist[i].Role == "assistant" && hist[i].Content != "" {
			return hist[i].Content, nil
		}
	}
	return "max tool loop iterations reached", nil
}

// executeToolCall runs a single tool call and returns the result string.
func executeToolCall(reg *Registry, tc toolCall) string {
	tool, ok := reg.Get(tc.Function.Name)
	if !ok {
		return fmt.Sprintf("Error: unknown tool %q", tc.Function.Name)
	}

	var args map[string]any
	if tc.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return fmt.Sprintf("Error: invalid tool arguments: %v", err)
		}
	}
	if args == nil {
		args = make(map[string]any)
	}

	result, err := tool.Execute(args)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return result
}
