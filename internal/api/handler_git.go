package api

import (
	"context"
	"errors"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"nova/internal/book"
)

// handleGitStatus GET /api/git/status — 返回当前书籍 Git 状态。
func (s *Server) handleGitStatus(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	status, err := s.app.GitStatus(ctx)
	if err != nil {
		writeGitError(c, err)
		return
	}
	writeJSON(c, consts.StatusOK, status)
}

// handleGitHistory GET /api/git/history?limit=20 — 返回最近提交历史。
func (s *Server) handleGitHistory(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	history, err := s.app.GitHistory(ctx, limit)
	if err != nil {
		writeGitError(c, err)
		return
	}
	writeJSON(c, consts.StatusOK, map[string]interface{}{"commits": history})
}

// handleGitDiff GET /api/git/diff?path=optional — 返回当前工作区 diff。
func (s *Server) handleGitDiff(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	output, err := s.app.GitDiff(ctx, c.Query("path"))
	if err != nil {
		writeGitError(c, err)
		return
	}
	writeJSON(c, consts.StatusOK, map[string]string{"diff": output})
}

// handleGitInit POST /api/git/init — 初始化当前书籍 Git 仓库。
func (s *Server) handleGitInit(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	result, err := s.app.InitGit(ctx)
	if err != nil {
		writeGitError(c, err)
		return
	}
	writeJSON(c, consts.StatusOK, result)
}

// handleGitCommit POST /api/git/commit — 创建书籍版本。
func (s *Server) handleGitCommit(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := c.BindJSON(&req); err != nil || req.Message == "" {
		writeError(c, consts.StatusBadRequest, "请提供版本说明")
		return
	}

	result, err := s.app.CreateGitVersion(ctx, req.Message)
	if err != nil {
		writeGitError(c, err)
		return
	}
	writeJSON(c, consts.StatusOK, result)
}

// handleGitRollback POST /api/git/rollback — 回滚整本书到指定版本。
func (s *Server) handleGitRollback(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	var req struct {
		Hash string `json:"hash"`
	}
	if err := c.BindJSON(&req); err != nil || req.Hash == "" {
		writeError(c, consts.StatusBadRequest, "请提供回滚版本")
		return
	}

	result, err := s.app.RollbackGitVersion(ctx, req.Hash)
	if err != nil {
		writeGitError(c, err)
		return
	}
	writeJSON(c, consts.StatusOK, result)
}

// handleGitStash POST /api/git/stash — 暂存当前未提交内容。
func (s *Server) handleGitStash(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	result, err := s.app.StashGitChanges(ctx)
	if err != nil {
		writeGitError(c, err)
		return
	}
	writeJSON(c, consts.StatusOK, result)
}

// handleGitStashPop POST /api/git/stash/pop — 恢复最近一次暂存内容。
func (s *Server) handleGitStashPop(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	result, err := s.app.PopGitStash(ctx)
	if err != nil {
		writeGitError(c, err)
		return
	}
	writeJSON(c, consts.StatusOK, result)
}

// handleGitCommand POST /api/git/command — 执行受限 Git 命令。
func (s *Server) handleGitCommand(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	var req struct {
		Command string `json:"command"`
	}
	if err := c.BindJSON(&req); err != nil || req.Command == "" {
		writeError(c, consts.StatusBadRequest, "请提供 command 参数")
		return
	}

	result, err := s.app.RunGitCommand(ctx, req.Command)
	if err != nil {
		writeGitError(c, err)
		return
	}
	writeJSON(c, consts.StatusOK, result)
}

func writeGitError(c *app.RequestContext, err error) {
	switch {
	case errors.Is(err, book.ErrGitNotInstalled):
		writeError(c, consts.StatusInternalServerError, err.Error())
	case errors.Is(err, book.ErrGitNotInit):
		writeError(c, consts.StatusBadRequest, err.Error())
	case errors.Is(err, book.ErrGitDirty):
		writeError(c, consts.StatusBadRequest, err.Error())
	case errors.Is(err, book.ErrGitClean):
		writeError(c, consts.StatusBadRequest, err.Error())
	default:
		writeError(c, consts.StatusBadRequest, err.Error())
	}
}
