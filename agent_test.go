package main

import (
	"context"
	"testing"
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
