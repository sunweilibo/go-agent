package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresSessionStore struct {
	Pool *pgxpool.Pool
}

func (s *PostgresSessionStore) Save(ctx context.Context, session *Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.Pool == nil {
		return fmt.Errorf("数据库连接池未配置")
	}

	if session == nil || !validSessionID.MatchString(session.ID) {
		return fmt.Errorf("无效的会话")
	}

	messages, err := json.Marshal(session.Messages)
	if err != nil {
		return fmt.Errorf("编码会话消息失败:%w", err)
	}

	const query = `
		insert into public.agent_sessions
		    (session_id, version, messages, updated_at)
		values ($1, $2, $3::jsonb, now())
		on conflict (session_id) do update
		set version = excluded.version,
			messages = excluded.messages,
		  updated_at = now()
		`
	_, err = s.Pool.Exec(ctx, query, session.ID, 1, string(messages))
	if err != nil {
		return fmt.Errorf("保存会话到数据库失败: %w", err)
	}

	return nil
}

func (s *PostgresSessionStore) Load(ctx context.Context, id string) (*Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.Pool == nil {
		return nil, fmt.Errorf("数据库连接池未配置")
	}

	if !validSessionID.MatchString(id) {
		return nil, fmt.Errorf("无效的会话 ID")
	}

	var version int
	var rawMessages string

	err := s.Pool.QueryRow(ctx, `select version, messages::text
		from public.agent_sessions
		where session_id = $1`, id).Scan(&version, &rawMessages)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, id)
	}

	if err != nil {
		return nil, fmt.Errorf("读取数据库会话失败: %w", err)
	}

	if version != 1 {
		return nil, fmt.Errorf("不支持的会话版本: %d", version)
	}

	var messages []Message
	if err := json.Unmarshal([]byte(rawMessages), &messages); err != nil {
		return nil, fmt.Errorf("解析会话消息失败:%w", err)
	}

	if len(messages) == 0 || messages[0].Role != "system" {
		return nil, fmt.Errorf("会话缺少系统信息")
	}

	return &Session{
		ID:       id,
		Messages: messages,
	}, nil
}

var _ SessionStore = (*PostgresSessionStore)(nil)
