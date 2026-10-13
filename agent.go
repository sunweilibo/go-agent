// Package main 主包
package main

import (
	"context"
	"fmt"
)

type Agent struct {
	Model       string
	Registry    ToolRegistry
	Definitions []ToolDefinition
	MaxSteps    int
}

func NewAgent(model string) *Agent {
	registry, definitions := newToolRegistry()

	return &Agent{
		Model:       model,
		Registry:    registry,
		Definitions: definitions,
		MaxSteps:    10,
	}
}

func (a *Agent) Run(
	ctx context.Context,
	task string,
) (string, error) {
	payload := ChatRequest{
		Model: a.Model,
		Messages: []Message{
			{
				Role: "system",
				Content: "你是一个助手。" +
					"查询当前时间时必须调用 get_current_time 工具。" +
					"涉及加法计算时必须调用 add_numbers 工具。" +
					"获得工具结果后，根据结果回答用户。",
			},
			{
				Role:    "user",
				Content: task,
			},
		},
		Tools: a.Definitions,
	}

	for step := 0; step < a.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		fmt.Printf("第 %d 轮请求模型", step+1)

		message, err := requestModel(ctx, payload)
		if err != nil {
			fmt.Printf("怎么就失败了呢%v", err)
			return "", err
		}

		if len(message.ToolCalls) == 0 {
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

			toolResult, toolErr := tool.Execute(ctx, call.Function.Arguments)
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
