package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestDatabaseConnection(t *testing.T) {
	if os.Getenv("SUPABASE_URL") == "" {
		t.Skip("未设置 DATABASE_URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := openDatabase(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var count int64
	err = pool.QueryRow(
		ctx,
		"select count(*) from public.agent_sessions",
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("连接成功；当前有 %d 个会话", count)
}
