package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		if err := runServe(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "HTTP 服务失败", err)
			os.Exit(1)
		}
		return
	}
	if err := os.MkdirAll("data/runs", 0o700); err != nil {
		fmt.Fprintln(os.Stderr, "创建日志目录失败", err)
		os.Exit(1)
	}

	logFile, err := os.OpenFile("data/runs/events.jsonl", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "打开执行日志失败", err)
		os.Exit(1)
	}

	defer logFile.Close()

	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stderr, logFile), nil))
	slog.SetDefault(logger)

	agent, err := NewAgent("deepseek-flash", DeepSeekClient{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化 Agent 失败:", err)
		os.Exit(1)
	}

	sessionID := flag.String("session", "", "要继续的会话 ID")
	storeName := flag.String("store", "file", "会话存储: file 或 postgres")
	readRoot := flag.String("read-root", "", "允许 read_file 读取的目录(绝对路径)")
	flag.Parse()

	if *readRoot != "" {
		if err = agent.EnableReadFile(*readRoot); err != nil {
			fmt.Fprintln(os.Stderr, "启用文件读取失败:", err)
			os.Exit(1)
		}
	}

	var store SessionStore

	storeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	store, closeStore, err := openSessionStore(storeCtx, *storeName)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer closeStore()

	var session *Session
	if *sessionID == "" {
		session = NewSession()
	} else {
		session, err = store.Load(context.Background(), *sessionID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "加载会话失败:", err)
			os.Exit(1)
		}
	}

	fmt.Fprintln(os.Stderr, "会话 ID:", session.ID)

	if len(flag.Args()) > 0 {
		task := strings.TrimSpace(strings.Join(flag.Args(), " "))
		if task == "" {
			fmt.Fprintln(os.Stderr, "请提供任务")
			os.Exit(1)
		}
		if err := runTask(agent, store, session, task); err != nil {
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

		if err := runTask(agent, store, session, task); err != nil {
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

func runTask(agent *Agent, store SessionStore, session *Session, task string) (runErr error) {
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	runID := rand.Text()
	started := time.Now()

	slog.Info("任务开始", "run_Id", runID, "session_id", session.ID)

	defer func() {
		status := "completed"
		switch {
		case errors.Is(runErr, context.Canceled):
			status = "canceled"
		case errors.Is(runErr, context.DeadlineExceeded):
			status = "timeout"
		case runErr != nil:
			status = "failed"
		}
		slog.Info("任务结束", "run_id", runID, "session_id", session.ID, "status", status, "duration_ms", time.Since(started).Milliseconds())
	}()

	answer, err := agent.Run(signalCtx, session, task)
	if err != nil {
		return err
	}

	fmt.Println(answer)
	if err := store.Save(signalCtx, session); err != nil {
		return fmt.Errorf("回答已输出,但保存会话失败: %w", err)
	}
	return nil
}

func openSessionStore(ctx context.Context, name string) (SessionStore, func(), error) {
	switch name {
	case "file":
		return FileSessionStore{}, func() {}, nil
	case "postgres":
		pool, err := openDatabase(ctx)
		if err != nil {
			return nil, func() {}, fmt.Errorf("选择数据库失败: %w", err)
		}
		return &PostgresSessionStore{Pool: pool}, pool.Close, nil
	default:
		return nil, func() {}, fmt.Errorf("未知的存储方式:%s", name)
	}
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "监听地址")
	storeName := fs.String("store", "file", "会话存储: file 或 postgres")
	fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	storeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	store, closeStore, err := openSessionStore(storeCtx, *storeName)
	cancel()
	if err != nil {
		return err
	}
	defer closeStore()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           NewServer(store).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("HTTP 服务启动", "addr", *addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	return srv.Shutdown(shutdownCtx)
}
