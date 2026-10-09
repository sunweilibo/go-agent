package main

import (
	"fmt"
	"os"
	"strings"
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

	resp, err := askDeepSeek(task)
	if err != nil {
		fmt.Println("执行失败", err)
		os.Exit(1)
	}

	fmt.Println(resp)
}
