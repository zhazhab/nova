package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"nova/internal/book"
)

func (s *Server) handleLoreItems(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	items, err := s.app.LoreItems()
	if err != nil {
		writeError(c, consts.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleLoreItemCreate(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	var body book.LoreItemInput
	if err := c.BindJSON(&body); err != nil {
		writeError(c, consts.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	item, err := s.app.CreateLoreItem(body)
	if err != nil {
		writeError(c, consts.StatusBadRequest, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, item)
}

func (s *Server) handleLoreItemUpdate(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	var body book.LoreItemInput
	if err := c.BindJSON(&body); err != nil {
		writeError(c, consts.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	item, err := s.app.UpdateLoreItem(c.Param("id"), body)
	if err != nil {
		writeError(c, consts.StatusBadRequest, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, item)
}

func (s *Server) handleLoreItemDelete(ctx context.Context, c *app.RequestContext) {
	if !s.requireWorkspace(c) {
		return
	}
	if err := s.app.DeleteLoreItem(c.Param("id")); err != nil {
		writeError(c, consts.StatusBadRequest, err.Error())
		return
	}
	writeJSON(c, consts.StatusOK, map[string]string{"status": "ok"})
}
