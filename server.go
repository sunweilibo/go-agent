package main

import (
	"encoding/json/v2"
	"errors"
	"log/slog"
	"net/http"
)

type sessionPayload struct {
	ID       string    `json:"id"`
	Messages []Message `json:"messages"`
}

type Server struct {
	store SessionStore
}

// writeJSON 写出一个 JSON 响应。
// v2 没有 Encoder 类型，流式写出对应 json.MarshalWrite。
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.MarshalWrite(w, value)
}

// writeError 统一错误响应格式
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func NewServer(store SessionStore) *Server {
	if store == nil {
		panic("NewServer: store 不能为 nil")
	}
	return &Server{store: store}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	return mux
}

type healthPayload struct {
	Status string `json:"status"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthPayload{Status: "ok"})
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	session := NewSession()
	if err := s.store.Save(r.Context(), session); err != nil {
		slog.Error("创建会话失败", "error", err)
		writeError(w, http.StatusInternalServerError, "创建会话失败")
		return
	}
	writeJSON(w, http.StatusCreated, sessionPayload{
		ID:       session.ID,
		Messages: session.Messages,
	})
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validSessionID.MatchString(id) {
		writeError(w, http.StatusBadRequest, "无效的会话 ID")
		return
	}

	session, err := s.store.Load(r.Context(), id)
	switch {
	case errors.Is(err, ErrSessionNotFound):
		writeError(w, http.StatusNotFound, "会话不存在")
	case err != nil:
		slog.Error("加载会话失败", "session_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "加载会话失败")
	default:
		writeJSON(w, http.StatusOK, sessionPayload{
			ID:       session.ID,
			Messages: session.Messages,
		})
	}
}
