// Package main 主包
package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type Agent struct {
	Model              string           `json:"model"`
	Registry           ToolRegistry     `json:"registry"`
	Definitions        []ToolDefinition `json:"definitions"`
	Client             ModelClient      `json:"client"`
	MaxSteps           int              `json:"max_steps"`
	MaxToolCalls       int              `json:"max_tool_calls"`
	MaxToolOutputBytes int
	ToolTimeout        time.Duration
}

func NewAgent(model string, client ModelClient) (*Agent, error) {
	registry, definitions, err := newToolRegistry()
	if err != nil {
		return nil, err
	}

	return &Agent{
		Model:              model,
		Client:             client,
		Registry:           registry,
		Definitions:        definitions,
		MaxSteps:           10,
		MaxToolCalls:       30,
		MaxToolOutputBytes: 64 << 10,
		ToolTimeout:        45 * time.Second,
	}, nil
}

func (a *Agent) Run(
	ctx context.Context,
	session *Session,
	task string,
) (string, error) {
	toolCallsUsed := 0
	messages := append([]Message(nil), session.Messages...)
	messages = append(messages, Message{
		Role:    "user",
		Content: task,
	})
	payload := ChatRequest{
		Model:    a.Model,
		Messages: messages,
		Tools:    a.Definitions,
	}

	for step := 0; step < a.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		slog.Info("请求模型", "session_id", session.ID, "step", step+1)

		message, err := a.Client.Generate(ctx, payload)
		if err != nil {
			return "", err
		}

		if len(message.ToolCalls) == 0 {
			session.Messages = append(payload.Messages, message)
			return message.Content, nil
		}

		if len(message.ToolCalls) > a.MaxToolCalls-toolCallsUsed {
			return "", fmt.Errorf("已达到最大工具调用次数 %d, 任务尚未完成", a.MaxToolCalls)
		}

		toolCallsUsed += len(message.ToolCalls)

		payload.Messages = append(payload.Messages, message)

		for _, call := range message.ToolCalls {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			tool, ok := a.Registry[call.Function.Name]
			if !ok {
				return "", fmt.Errorf("未知工具： %s", call.Function.Name)
			}

			slog.Info("调用工具", "session_id", session.ID, "step", step+1, "tool", call.Function.Name, "call_id", call.ID)
			toolCtx, cancel := context.WithTimeout(ctx, a.ToolTimeout)
			toolResult, toolErr := tool.Execute(toolCtx, call.Function.Arguments)
			cancel()

			if err := ctx.Err(); err != nil {
				return "", err
			}
			if toolErr != nil {
				toolResult = "工具执行失败:" + toolErr.Error()
			}

			toolResult = limitToolOutput(toolResult, a.MaxToolOutputBytes)
			payload.Messages = append(payload.Messages, Message{
				Role:       "tool",
				Content:    toolResult,
				ToolCallID: call.ID,
			})
		}
	}
	return "", fmt.Errorf("已达到最大模型请求次数 %d，任务尚未完成", a.MaxSteps)
}

func limitToolOutput(output string, maxBytes int) string {
	const marker = "\n[truncated]"

	if len(output) <= maxBytes {
		return output
	}

	if maxBytes <= 0 {
		return ""
	}
	if maxBytes <= len(marker) {
		return marker[:maxBytes]
	}

	prefix := strings.ToValidUTF8(output[:maxBytes-len(marker)], "")
	return prefix + marker
}
