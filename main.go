package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	agent, err := NewAgent("deepseek-flash", DeepSeekClient{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化 Agent 失败:", err)
		os.Exit(1)
	}

	sessionID := flag.String("session", "", "要继续的会话 ID")
	flag.Parse()

	var session *Session
	if *sessionID == "" {
		session = NewSession()
	} else {
		session, err = loadSession(*sessionID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "加载会话失败:", err)
			os.Exit(1)
		}
	}

	fmt.Fprintln(os.Stderr, "会话 ID:", session.ID)

	if len(os.Args) > 1 {
		task := strings.TrimSpace(strings.Join(flag.Args(), " "))
		if task == "" {
			fmt.Fprintln(os.Stderr, "请提供任务")
			os.Exit(1)
		}
		if err := runTask(agent, session, task); err != nil {
			fmt.Fprintln(os.Stderr, "执行失败", err)
			os.Exit(1)
		}
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 1<<20)

	fmt.Fprintf(os.Stderr, "进入对话模式,输入/new 开始新会话, /exit 退出")

	for {
		fmt.Fprintln(os.Stderr, "> ")
		if !scanner.Scan() {
			break
		}

		task := strings.TrimSpace(scanner.Text())

		switch task {
		case "":
			continue
		case "/exit":
			return
		case "/new":
			session = NewSession()
			fmt.Fprintln(os.Stderr, "新会话 ID:", session.ID)
			continue
		}

		if err := runTask(agent, session, task); err != nil {
			fmt.Fprintln(os.Stderr, "执行失败:", err)
			if errors.Is(err, context.Canceled) {
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "读取输入失败:", err)
		os.Exit(1)
	}
}

func runTask(agent *Agent, session *Session, task string) error {
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ctx, cancel := context.WithTimeout(signalCtx, 2*time.Minute)
	defer cancel()

	answer, err := agent.Run(ctx, session, task)
	if err != nil {
		return err
	}

	fmt.Println(answer)
	if err := saveSession(session); err != nil {
		return fmt.Errorf("回答已输出,但保存会话失败: %w", err)
	}
	return nil
}
