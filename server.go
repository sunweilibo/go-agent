package main

import (
	"encoding/json/v2"
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
	// mux.HandleFunc("POST /api/sessions", s.handleCreateSession)
	// mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	return mux
}

type healthPayload struct {
	Status string `json:"status"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthPayload{Status: "ok"})
}
