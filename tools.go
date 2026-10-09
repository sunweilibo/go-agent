// Package main 主包
package main

import (
	"encoding/json/v2"
	"fmt"
	"time"
)

func getCurrentTime() string {
	return time.Now().Format(time.RFC3339)
}

type AddNumbersArgs struct {
	A *float64 `json:"a"`
	B *float64 `json:"b"`
}

func addNumbers(arguments string) (string, error) {
	var args AddNumbersArgs

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("参数不是合法的数字 JSON: %w", err)
	}

	if args.A == nil || args.B == nil {
		return "", fmt.Errorf("必须提供数字参数 a 和 b")
	}

	sum := *args.A + *args.B
	return fmt.Sprintf("%g", sum), nil
}
