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
