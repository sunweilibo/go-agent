package main

type Tool struct {
	Definition ToolDefinition
	Execute    func(arguments string) (string, error)
}

type ToolRegistry map[string]Tool

func newToolRegistry() (ToolRegistry, []ToolDefinition) {
	tools := []Tool{
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
			Execute: func(arguments string) (string, error) {
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
		registry[name] = tool
		definitions = append(definitions, tool.Definition)
	}

	return registry, definitions
}
