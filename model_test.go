package main

import (
	"encoding/json/v2"
	"strings"
	"testing"
)

func TestEmptyToolContentIsSerialized(t *testing.T) {
	req := ChatRequest{
		Model: "test-model",
		Messages: []Message{{
			Role:       "tool",
			Content:    "",
			ToolCallID: "call-1",
		}},
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"content":""`) {
		t.Fatalf("工具消息缺少空 content: %s", body)
	}
}
