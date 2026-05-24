package api

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"nova/internal/interactive"
)

func (s *Server) handleInteractiveStories(ctx context.Context, c *app.RequestContext) {
	index, err := s.app.InteractiveStories()
	if err != nil {
		writeError(c, consts.StatusConflict, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, index)
}

func (s *Server) handleInteractiveStoryCreate(ctx context.Context, c *app.RequestContext) {
	var body interactive.CreateStoryRequest
	if err := c.BindJSON(&body); err != nil {
		writeError(c, consts.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	story, err := s.app.CreateInteractiveStory(body)
	if err != nil {
		writeError(c, consts.StatusBadRequest, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, story)
}

func (s *Server) handleInteractiveStoryUpdate(ctx context.Context, c *app.RequestContext) {
	var body interactive.UpdateStoryRequest
	if err := c.BindJSON(&body); err != nil {
		writeError(c, consts.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	story, err := s.app.UpdateInteractiveStory(c.Param("id"), body)
	if err != nil {
		writeError(c, consts.StatusBadRequest, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, story)
}

func (s *Server) handleInteractiveStoryDelete(ctx context.Context, c *app.RequestContext) {
	if err := s.app.DeleteInteractiveStory(c.Param("id")); err != nil {
		writeError(c, consts.StatusBadRequest, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleInteractiveSnapshot(ctx context.Context, c *app.RequestContext) {
	snapshot, err := s.app.InteractiveSnapshot(c.Param("id"), c.Query("branch"))
	if err != nil {
		writeError(c, consts.StatusNotFound, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, snapshot)
}

func (s *Server) handleInteractiveBranches(ctx context.Context, c *app.RequestContext) {
	branches, err := s.app.InteractiveBranches(c.Param("id"))
	if err != nil {
		writeError(c, consts.StatusNotFound, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, map[string]any{"branches": branches})
}

func (s *Server) handleInteractiveBranchCreate(ctx context.Context, c *app.RequestContext) {
	var body interactive.CreateBranchRequest
	if err := c.BindJSON(&body); err != nil {
		writeError(c, consts.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	branch, err := s.app.CreateInteractiveBranch(c.Param("id"), body)
	if err != nil {
		writeError(c, consts.StatusBadRequest, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, branch)
}

func (s *Server) handleInteractiveBranchSwitch(ctx context.Context, c *app.RequestContext) {
	var body struct {
		BranchID string `json:"branch_id"`
	}
	if err := c.BindJSON(&body); err != nil {
		writeError(c, consts.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if err := s.app.SwitchInteractiveBranch(c.Param("id"), body.BranchID); err != nil {
		writeError(c, consts.StatusBadRequest, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleInteractiveChat(ctx context.Context, c *app.RequestContext) {
	var body struct {
		Mode    string `json:"mode"`
		StoryID string `json:"story_id"`
		Branch  string `json:"branch"`
		Message string `json:"message"`
	}
	if err := c.BindJSON(&body); err != nil {
		writeError(c, consts.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if strings.TrimSpace(body.Message) == "" {
		writeError(c, consts.StatusBadRequest, "消息不能为空")
		return
	}
	if body.Mode != "" && body.Mode != "story" {
		writeError(c, consts.StatusBadRequest, "当前仅支持 story 子模式")
		return
	}

	narrative := buildInteractiveNarrative(body.Message)
	turn, err := s.app.AppendInteractiveTurn(body.StoryID, body.Branch, body.Message, narrative)
	if err != nil {
		writeError(c, consts.StatusBadRequest, err.Error())
		return
	}

	c.Response.Header.Set("Content-Type", "text/event-stream")
	c.Response.Header.Set("Cache-Control", "no-cache")
	c.Response.Header.Set("Connection", "keep-alive")
	c.Response.ImmediateHeaderFlush = true
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		_ = writeSSE(pw, "chunk", map[string]string{"content": narrative})
		_ = writeSSE(pw, "done", map[string]any{"turn_id": turn.ID})
	}()
	c.Response.SetBodyStream(pr, -1)
}

func buildInteractiveNarrative(message string) string {
	text := strings.TrimSpace(message)
	return fmt.Sprintf("你选择：%s\n\n故事继续推进。周围的细节随你的行动发生变化，新的线索正在浮现。", text)
}

func (s *Server) handleInteractiveTellers(ctx context.Context, c *app.RequestContext) {
	tellers, err := s.app.InteractiveTellers()
	if err != nil {
		writeError(c, consts.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, map[string]any{"tellers": tellers})
}

func (s *Server) handleInteractiveTeller(ctx context.Context, c *app.RequestContext) {
	id := c.Param("id")
	teller, err := s.app.InteractiveTeller(id)
	if err != nil {
		writeError(c, consts.StatusNotFound, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, teller)
}
