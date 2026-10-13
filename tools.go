// Package main 主包
package main

import (
	"context"
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

func addNumbers(_ context.Context, arguments string) (string, error) {
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

func waitSeconds(ctx context.Context, arguments string) (string, error) {
	var args struct {
		Seconds *int `json:"seconds"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("解析等待参数失败: %w", err)
	}

	if args.Seconds == nil {
		return "", fmt.Errorf("必须提供 seconds 参数")
	}

	seconds := *args.Seconds
	if seconds < 1 || seconds > 30 {
		return "", fmt.Errorf("Seconds 必须在 1 到 20 之间")
	}

	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
		return fmt.Sprintf("已等待%d秒", seconds), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
