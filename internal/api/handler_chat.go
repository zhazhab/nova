package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"nova/internal/agent"
	novaApp "nova/internal/app"
)

// handleChat 处理聊天请求：启动后台 Task，然后以 SSE 流订阅事件。
func (s *Server) handleChat(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	var req agent.ChatRequest
	if err := c.BindJSON(&req); err != nil {
		writeError(c, consts.StatusBadRequest, "无效请求体")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(c, consts.StatusBadRequest, "消息不能为空")
		return
	}

	task := s.app.StartTask(req)
	if task == nil {
		writeError(c, consts.StatusConflict, "尚未选择书籍工作区，请先在书籍管理页选择或创建书籍")
		return
	}
	log.Printf("[agent-sse] attach new chat task_id=%s", task.ID())
	streamTask(c, task)
}

// handleChatStream 重连到当前活跃任务的事件流（回放已有事件 + 继续接收新事件）。
func (s *Server) handleChatStream(ctx context.Context, c *app.RequestContext) {
	task := s.app.ActiveTask()
	if task == nil {
		writeError(c, consts.StatusNotFound, "没有活跃任务")
		return
	}
	log.Printf("[agent-sse] attach active chat task_id=%s status=%s", task.ID(), task.Status())
	streamTask(c, task)
}

// handleChatActive 查询当前是否有活跃任务。
func (s *Server) handleChatActive(ctx context.Context, c *app.RequestContext) {
	task := s.app.ActiveTask()
	if task == nil {
		c.JSON(consts.StatusOK, map[string]interface{}{
			"active": false,
		})
		return
	}
	status := task.Status()
	c.JSON(consts.StatusOK, map[string]interface{}{
		"active": status == novaApp.TaskRunning,
		"status": status,
	})
}

// handleChatAbort 终止当前活跃任务。
func (s *Server) handleChatAbort(ctx context.Context, c *app.RequestContext) {
	if task := s.app.ActiveTask(); task != nil {
		log.Printf("[agent-sse] abort requested task_id=%s status=%s", task.ID(), task.Status())
	}
	s.app.AbortTask()
	c.JSON(consts.StatusOK, map[string]string{"status": "ok"})
}

// streamTask 将 Task 的事件（快照 + 实时）以 SSE 格式写入 HTTP 响应。
func streamTask(c *app.RequestContext, task *novaApp.Task) {
	c.Response.Header.Set("Content-Type", "text/event-stream")
	c.Response.Header.Set("Cache-Control", "no-cache")
	c.Response.Header.Set("Connection", "keep-alive")
	c.Response.ImmediateHeaderFlush = true

	pr, pw := io.Pipe()

	go func() {
		var ch <-chan agent.Event
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("[agent-sse] stream panic recovered task_id=%s err=%v", task.ID(), recovered)
			}
			if ch != nil {
				task.Unsubscribe(ch)
			}
			_ = pw.Close()
		}()
		var snapshot []agent.Event
		snapshot, ch = task.Subscribe()
		log.Printf("[agent-sse] stream start task_id=%s replay=%d", task.ID(), len(snapshot))

		// 回放已有事件
		for _, ev := range snapshot {
			if err := writeSSE(pw, ev.Type, ev.Data); err != nil {
				log.Printf("[agent-sse] stream interrupted task_id=%s phase=replay event=%s err=%v", task.ID(), ev.Type, err)
				return
			}
		}

		// 持续接收新事件
		for ev := range ch {
			if err := writeSSE(pw, ev.Type, ev.Data); err != nil {
				log.Printf("[agent-sse] stream interrupted task_id=%s phase=live event=%s err=%v", task.ID(), ev.Type, err)
				return
			}
		}
		log.Printf("[agent-sse] stream end task_id=%s status=%s", task.ID(), task.Status())
	}()

	c.Response.SetBodyStream(pr, -1)
}

// writeSSE 写入一条 SSE 事件。
func writeSSE(w io.Writer, eventType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
	return err
}
