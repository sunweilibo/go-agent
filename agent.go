// Package main 主包
package main

import (
	"context"
	"fmt"
	"os"
)

type Agent struct {
	Model       string
	Registry    ToolRegistry
	Definitions []ToolDefinition
	Client      ModelClient
	MaxSteps    int
}

func NewAgent(model string, client ModelClient) (*Agent, error) {
	registry, definitions, err := newToolRegistry()
	if err != nil {
		return nil, err
	}

	return &Agent{
		Model:       model,
		Client:      client,
		Registry:    registry,
		Definitions: definitions,
		MaxSteps:    10,
	}, nil
}

func (a *Agent) Run(
	ctx context.Context,
	session *Session,
	task string,
) (string, error) {
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
		fmt.Fprintf(os.Stderr, "第 %d 轮请求模型", step+1)

		message, err := a.Client.Generate(ctx, payload)
		if err != nil {
			return "", err
		}

		if len(message.ToolCalls) == 0 {
			session.Messages = append(payload.Messages, message)
			return message.Content, nil
		}

		payload.Messages = append(payload.Messages, message)

		for _, call := range message.ToolCalls {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			tool, ok := a.Registry[call.Function.Name]
			if !ok {
				return "", fmt.Errorf("未知工具： %s", call.Function.Name)
			}

			fmt.Fprintf(os.Stderr, "正在调用工具: %s", call.Function.Name)
			toolResult, toolErr := tool.Execute(ctx, call.Function.Arguments)

			if err := ctx.Err(); err != nil {
				return "", err
			}
			if toolErr != nil {
				toolResult = "工具执行失败:" + toolErr.Error()
			}

			payload.Messages = append(payload.Messages, Message{
				Role:       "tool",
				Content:    toolResult,
				ToolCallID: call.ID,
			})
		}
	}
	return "", fmt.Errorf("已达到最大模型请求次数 %d，任务尚未完成", a.MaxSteps)
}
