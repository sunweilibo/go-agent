package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	task := strings.TrimSpace(strings.Join(os.Args[1:], " "))

	if task == "" {
		fmt.Println("请提供任务")
		os.Exit(1)
	}

	fmt.Println("收到任务", task)

	if task == "time" {
		fmt.Println(getCurrentTime())
		return
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, 2*time.Minute)
	defer cancel()

	agent := NewAgent("deepseek-flash")
	resp, err := agent.Run(ctx, task)
	if err != nil {
		fmt.Println("执行失败", err)
		os.Exit(1)
	}

	fmt.Println(resp)
}
