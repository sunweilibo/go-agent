package main

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

type sessionSnapshot struct {
	Version  int       `json:"version"`
	ID       string    `json:"session_id"`
	Messages []Message `json:"messages"`
}

func saveSession(session *Session) error {
	dir := filepath.Join("data", "sessions")

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("创建会话目录: %w", err)
	}

	data, err := json.Marshal(sessionSnapshot{
		Version:  1,
		ID:       session.ID,
		Messages: session.Messages,
	})
	if err != nil {
		return fmt.Errorf("编码会话: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时文件: %w", err)
	}

	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入会话: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭会话文件: %w", err)
	}

	path := filepath.Join(dir, session.ID+".json")
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("保存会话失败: %w", err)
	}
	return nil
}

var validSessionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func loadSession(id string) (*Session, error) {
	if !validSessionID.MatchString(id) {
		return nil, fmt.Errorf("无效的会话 ID")
	}
	path := filepath.Join("data", "sessions", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取会话: %w", err)
	}

	var snapshot sessionSnapshot
	if err = json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("解析会话文件: %w", err)
	}

	if snapshot.Version != 1 {
		return nil, fmt.Errorf("不支持的会话文件版本: %d", snapshot.Version)
	}

	if snapshot.ID != id {
		return nil, fmt.Errorf("会话文件中的 ID 不匹配")
	}

	if len(snapshot.Messages) == 0 || snapshot.Messages[0].Role != "system" {
		return nil, fmt.Errorf("会话缺少系统信息")
	}

	return &Session{
		ID:       snapshot.ID,
		Messages: snapshot.Messages,
	}, nil
}
