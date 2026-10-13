package main

import (
	"context"
	"errors"
	"fmt"
)

type Tool struct {
	Definition ToolDefinition
	Execute    func(ctx context.Context, arguments string) (string, error)
}

type ToolRegistry map[string]Tool

func newToolRegistry() (ToolRegistry, []ToolDefinition, error) {
	tools := []Tool{
		{
			Definition: ToolDefinition{
				Type: "function",
				Function: FunctionDefinition{
					Name:        "wait_seconds",
					Description: "等待指定秒数，范围为 1 到 30 秒。",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"seconds": map[string]any{
								"type":    "integer",
								"minimum": 1,
								"maximum": 30,
							},
						},
						"required": []string{"seconds"},
					},
				},
			},
			Execute: waitSeconds,
		},
		{
			Definition: ToolDefinition{
				Type: "function",
				Function: FunctionDefinition{
					Name:        "get_current_time",
					Description: "获取程序所在机器的当前时间，包含时区",
					Parameters: map[string]any{
						"type":       "object",
						"properties": map[string]any{},
						"required":   []string{},
					},
				},
			},
			Execute: func(ctx context.Context, arguments string) (string, error) {
				return getCurrentTime(), nil
			},
		},
		{
			Definition: ToolDefinition{
				Type: "function",
				Function: FunctionDefinition{
					Name:        "add_numbers",
					Description: "计算两个数字的和",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"a": map[string]any{
								"type":        "number",
								"description": "第一个加数",
							},
							"b": map[string]any{
								"type":        "number",
								"description": "第二个加数",
							},
						},
						"required": []string{"a", "b"},
					},
				},
			},
			Execute: addNumbers,
		},
	}
	registry := make(ToolRegistry)
	definitions := make([]ToolDefinition, 0, len(tools))

	for _, tool := range tools {
		name := tool.Definition.Function.Name
		if name == "" {
			return nil, nil, errors.New("工具名称不能为空")
		}

		if tool.Execute == nil {
			return nil, nil, fmt.Errorf("工具 %q 没有执行函数", name)
		}
		if _, exists := registry[name]; exists {
			return nil, nil, fmt.Errorf("工具 %q 重复注册", name)
		}
		registry[name] = tool
		definitions = append(definitions, tool.Definition)
	}

	return registry, definitions, nil
}
