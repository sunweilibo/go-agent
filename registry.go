package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"
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

func readFile(ctx context.Context, arguments string, rootDir string) (string, error) {
	const maxBytes = 64 << 10

	if !filepath.IsAbs(rootDir) {
		return "", fmt.Errorf("读取根目录必须时绝对路径")
	}

	var args struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("解析 read_file 参数失败: %w", err)
	}
	if !filepath.IsLocal(args.Path) {
		return "", fmt.Errorf("只允许工作目录内的相对路径")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return "", fmt.Errorf("打开工作目录失败: %w", err)
	}
	defer root.Close()

	file, err := root.Open(args.Path)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(data) > maxBytes {
		return "", fmt.Errorf("文件超过%d字节", maxBytes)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf("文件包含 NUL 字节, 不是文本文件")
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("文件不是有效的 UTF-8文本")
	}

	return string(data), nil
}
