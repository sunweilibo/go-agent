package main

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

type modelFunc func(context.Context, ChatRequest) (Message, error)

func (f modelFunc) Generate(
	ctx context.Context,
	req ChatRequest,
) (Message, error) {
	return f(ctx, req)
}

func TestAgentToolCallHistory(t *testing.T) {
	var requests [][]Message

	client := modelFunc(func(_ context.Context, req ChatRequest) (Message, error) {
		requests = append(requests, append([]Message(nil), req.Messages...))

		if len(requests) == 1 {
			call := ToolCall{ID: "call-1", Type: "function"}
			call.Function.Name = "get_current_time"
			call.Function.Arguments = "{}"

			return Message{
				Role:      "assistant",
				ToolCalls: []ToolCall{call},
			}, nil
		}

		return Message{
			Role:    "assistant",
			Content: "时间查询完成",
		}, nil
	})

	agent, err := NewAgent("test-model", client)
	if err != nil {
		t.Fatal(err)
	}

	session := NewSession()
	answer, err := agent.Run(context.Background(), session, "现在几点")
	if err != nil {
		t.Fatal(err)
	}

	if answer != "时间查询完成" {
		t.Fatalf("回答 = %q", answer)
	}
	if len(requests) != 2 {
		t.Fatalf("模型请求次数 = %d, 期望 2", len(requests))
	}

	t.Logf("requests: %+v", requests)
	t.Logf("Messages: %+v", session.Messages)
	second := requests[1]
	if len(second) != 4 {
		t.Fatalf("第二次请求的消息数 = %d, 期望 4", len(second))
	}

	if second[2].Role != "assistant" || len(second[2].ToolCalls) != 1 {
		t.Fatalf("缺少发起工具调用的 assistant 消息")
	}

	if second[3].Role != "tool" || second[3].ToolCallID != "call-1" || second[3].Content == "" {
		t.Fatalf("工具结果未正确关联调用: %+v", second[3])
	}

	if len(session.Messages) != 5 {
		t.Fatalf("完成后的会话消息数 = %d, 期望5", len(session.Messages))
	}
}

func TestAgentRejectsTolBatchOverLimit(t *testing.T) {
	modelCalls := 0
	client := modelFunc(func(_ context.Context, _ ChatRequest) (Message, error) {
		modelCalls++

		first := ToolCall{ID: "call-1", Type: "function"}
		first.Function.Name = "get_current_time"
		first.Function.Arguments = "{}"

		second := ToolCall{ID: "call-2", Type: "function"}
		second.Function.Name = "get_current_time"
		second.Function.Arguments = "{}"

		return Message{
			Role:      "assistant",
			ToolCalls: []ToolCall{first, second},
		}, nil
	})

	agent, err := NewAgent("test-model", client)
	if err != nil {
		t.Fatal(err)
	}
	agent.MaxToolCalls = 1

	executed := 0
	tool := agent.Registry["get_current_time"]
	tool.Execute = func(context.Context, string) (string, error) {
		executed++
		return "已执行", nil
	}

	agent.Registry["get_current_time"] = tool

	session := NewSession()
	initialMessages := len(session.Messages)
	_, err = agent.Run(context.Background(), session, "查询两次时间")

	if err == nil {
		t.Fatal("模型工具调用超额错误")
	}

	if modelCalls != 1 {
		t.Fatalf("模型请求次数 = %d, 期望 1", modelCalls)
	}

	if executed != 0 {
		t.Fatalf("执行了 %d 次工具，期望 0", executed)
	}

	if len(session.Messages) != initialMessages {
		t.Fatalf("失败的任务不应写入会话历史")
	}
}

func TestAgentLimitsToolOutput(t *testing.T) {
	modelCalls := 0

	client := modelFunc(func(_ context.Context, req ChatRequest) (Message, error) {
		modelCalls++

		if modelCalls == 1 {
			call := ToolCall{ID: "call-1", Type: "function"}
			call.Function.Name = "get_current_time"
			call.Function.Arguments = "{}"

			return Message{
				Role:      "assistant",
				ToolCalls: []ToolCall{call},
			}, nil
		}

		if len(req.Messages) != 4 {
			t.Fatalf("第二次请求有 %d 条消息，期望 4", len(req.Messages))
		}

		result := req.Messages[3]
		if result.Role != "tool" || result.ToolCallID != "call-1" {
			t.Fatalf("工具结果未正确关联：%+v", result)
		}
		if len(result.Content) > 32 {
			t.Fatalf("工具结果为 %d 字节，超过 32", len(result.Content))
		}
		if !utf8.ValidString(result.Content) {
			t.Fatal("截断后的工具结果不是合法 UTF-8")
		}
		if !strings.HasSuffix(result.Content, "[truncated]") {
			t.Fatalf("缺少截断标记：%q", result.Content)
		}

		return Message{Role: "assistant", Content: "完成"}, nil
	})

	agent, err := NewAgent("test-model", client)
	if err != nil {
		t.Fatal(err)
	}
	agent.MaxToolOutputBytes = 32

	tool := agent.Registry["get_current_time"]
	tool.Execute = func(context.Context, string) (string, error) {
		return strings.Repeat("中", 100), nil
	}
	agent.Registry["get_current_time"] = tool

	_, err = agent.Run(context.Background(), NewSession(), "查询时间")
	if err != nil {
		t.Fatal(err)
	}
	if modelCalls != 2 {
		t.Fatalf("模型请求次数 = %d，期望 2", modelCalls)
	}
}
