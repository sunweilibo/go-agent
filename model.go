package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"os"
	"time"
)

type Message struct {
	Role       string     `json:"role,omitempty"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}
type ToolCall struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ChatRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

func requestModel(ctx context.Context, payload ChatRequest) (Message, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return Message{}, fmt.Errorf("请设置 DEEPSEEK_API_KEY")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("编码请求失败: %w", err)
	}
	// 构建 http 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.deepseek.com/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Message{}, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// 发送请求
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return Message{}, fmt.Errorf("请求模型失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Message{}, fmt.Errorf("模型服务返回错误：%v", resp.StatusCode)
	}

	// 解析响应
	var result ChatResponse
	if err := json.UnmarshalRead(resp.Body, &result); err != nil {
		return Message{}, fmt.Errorf("解析相应失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return Message{}, fmt.Errorf("模型没有返回回答")
	}
	return result.Choices[0].Message, nil
}

func askDeepSeek(ctx context.Context, task string) (string, error) {
	registry, definition := newToolRegistry()
	payload := ChatRequest{
		Model: "deepseek-flash",
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
		Tools: definition,
	}

	const maxSteps = 10

	for step := 0; step < maxSteps; step++ {
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
			tool, ok := registry[call.Function.Name]
			if !ok {
				return "", fmt.Errorf("未知工具： %s", call.Function.Name)
			}

			toolResult, toolErr := tool.Execute(call.Function.Arguments)
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
	return "", fmt.Errorf("已达到最大模型请求次数 %d，任务尚未完成", maxSteps)
}
