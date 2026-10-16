package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestPostgresStoreCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store := &PostgresSessionStore{}
	session := NewSession()

	if err := store.Save(ctx, session); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save 错误 =%v, 期望 context.Canceled", err)
	}

	if _, err := store.Load(ctx, session.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("Load 错误 = %v, 期望 cntext.Canceled", err)
	}
}

func TestPostgresStoreMissingSession(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("未配置数据库连接串")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := openDatabase(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	store := &PostgresSessionStore{Pool: pool}
	missingID := NewSession().ID

	_, err = store.Load(ctx, missingID)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Load 错误 = %v", err)
	}
}
