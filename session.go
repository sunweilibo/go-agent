package main

import "crypto/rand"

type Session struct {
	ID       string
	Messages []Message
}

func NewSession() *Session {
	return &Session{
		ID: rand.Text(),
		Messages: []Message{
			{
				Role:    "system",
				Content: "你是一个助手." + "查询当前时间时必须调用 get_current_time 工具." + "设计加法计算时必须调用 add_numbers 工具." + "获取工具结果后,根据结果回答客户.",
			},
		},
	}
}
