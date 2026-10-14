package main

import (
	"context"
	"errors"
	"fmt"
	"os"
)

var ErrSessionNotFound = errors.New("会话不存在")

type SessionStore interface {
	Save(ctx context.Context, session *Session) error
	Load(ctx context.Context, id string) (*Session, error)
}

type FileSessionStore struct{}

func (FileSessionStore) Save(ctx context.Context, session *Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return saveSession(session)
}

func (FileSessionStore) Load(ctx context.Context, id string) (*Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	session, err := loadSession(id)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, id)
	}

	return session, err
}
