package main

import (
	"context"
	"errors"
	"testing"
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
