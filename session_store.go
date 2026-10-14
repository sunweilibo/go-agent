package main

import (
	"context"
)

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
	return loadSession(id)
}
