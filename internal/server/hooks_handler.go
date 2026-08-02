package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/0funct0ry/squad/internal/hooks"
	"github.com/gin-gonic/gin"
)

// registerHooksRoutes wires M10c's Lua trigger-hook surface for the
// single-DB flow. Unlike --modules, hooks have no enable flag: the routes are
// always live, and only mutation (create/update/delete) is gated on --write,
// exactly like the rest of the write surface. /test and /log stay available
// in read-only mode so a user can dry-run and inspect hooks without
// restarting.
func (s *Server) registerHooksRoutes(api *gin.RouterGroup) {
	h := api.Group("/hooks")
	h.GET("", s.handleListHooks)
	h.GET("/:id", s.handleGetHook)
	h.POST("", s.WriteGateMiddleware("creating a hook"), s.handleCreateHook)
	h.PATCH("/:id", s.WriteGateMiddleware("updating a hook"), s.handleUpdateHook)
	h.DELETE("/:id", s.WriteGateMiddleware("deleting a hook"), s.handleDeleteHook)
	h.POST("/:id/test", s.handleTestHook)
	h.GET("/:id/log", s.handleHookLog)
	h.DELETE("/:id/log", s.WriteGateMiddleware("clearing a hook's execution log"), s.handleClearHookLog)
}

func hookError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"ok": false, "error": gin.H{"code": code, "message": message}})
}

// hookStatus is the process state the Hooks tab's status strip renders.
func (s *Server) hookStatus() gin.H {
	return gin.H{
		"hookMode": s.hookMode,
		"write":    s.write,
		"allowNet": s.allowNet,
	}
}

// hookSummary is the light list payload — no Lua source (PROMPTS.md M10c:
// "no source code in the list payload, keep it light").
func hookSummary(h hooks.Hook) gin.H {
	return gin.H{
		"id":          h.ID,
		"table":       h.Table,
		"event":       h.Event,
		"timing":      h.Timing,
		"scope":       h.Scope,
		"name":        h.Name,
		"description": h.Description,
		"enabled":     h.Enabled,
		"createdAt":   h.CreatedAt,
		"updatedAt":   h.UpdatedAt,
	}
}

// GET /api/hooks?table=NAME
func (s *Server) handleListHooks(c *gin.Context) {
	list, err := hooks.List(s.db, c.Query("table"))
	if err != nil {
		hookError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	views := make([]gin.H, 0, len(list))
	for _, h := range list {
		views = append(views, hookSummary(h))
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
		"hooks":    views,
		"hookMode": s.hookMode,
		"write":    s.write,
		"allowNet": s.allowNet,
	}})
}

func hookIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		hookError(c, http.StatusBadRequest, "BAD_REQUEST", "hook id must be an integer")
		return 0, false
	}
	return id, true
}

// GET /api/hooks/:id
func (s *Server) handleGetHook(c *gin.Context) {
	id, ok := hookIDParam(c)
	if !ok {
		return
	}
	h, err := hooks.Get(s.db, id)
	if err != nil {
		hookError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
		"id": h.ID, "table": h.Table, "event": h.Event, "timing": h.Timing,
		"scope": h.Scope, "name": h.Name, "description": h.Description,
		"source": h.Source, "enabled": h.Enabled,
		"createdAt": h.CreatedAt, "updatedAt": h.UpdatedAt,
		"status": s.hookStatus(),
	}})
}

type hookRequest struct {
	Table       *string `json:"table"`
	Event       *string `json:"event"`
	Timing      *string `json:"timing"`
	Scope       *string `json:"scope"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Source      *string `json:"source"`
	Enabled     *bool   `json:"enabled"`
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// POST /api/hooks
func (s *Server) handleCreateHook(c *gin.Context) {
	var req hookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		hookError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	scope := deref(req.Scope)
	if scope == "" {
		scope = "row"
	}
	h, err := hooks.Create(s.db, hooks.Hook{
		Table: deref(req.Table), Event: deref(req.Event), Timing: deref(req.Timing),
		Scope: scope, Name: deref(req.Name), Description: deref(req.Description),
		Source: deref(req.Source), Enabled: enabled,
	})
	if err != nil {
		hookError(c, http.StatusBadRequest, "VALIDATION", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": hookSummary(h)})
}

// PATCH /api/hooks/:id
func (s *Server) handleUpdateHook(c *gin.Context) {
	id, ok := hookIDParam(c)
	if !ok {
		return
	}
	var req hookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		hookError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	h, err := hooks.Update(s.db, id, hooks.Patch{
		Table: req.Table, Event: req.Event, Timing: req.Timing, Scope: req.Scope,
		Name: req.Name, Description: req.Description, Source: req.Source, Enabled: req.Enabled,
	})
	if err != nil {
		status, code := http.StatusBadRequest, "VALIDATION"
		if strings.Contains(err.Error(), "not found") {
			status, code = http.StatusNotFound, "NOT_FOUND"
		}
		hookError(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": hookSummary(h)})
}

// DELETE /api/hooks/:id
func (s *Server) handleDeleteHook(c *gin.Context) {
	id, ok := hookIDParam(c)
	if !ok {
		return
	}
	if err := hooks.Delete(s.db, id); err != nil {
		hookError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"id": id}})
}

type testHookRequest struct {
	Old    map[string]any `json:"old"`
	New    map[string]any `json:"new"`
	Source *string        `json:"source"` // optional: test unsaved editor content
}

// POST /api/hooks/:id/test
//
// Runs the hook's Lua source directly against sample data — no trigger fires
// and no real table is touched by squad itself. A script that attempts a
// gated write still fails with the same READ_ONLY error a live run would
// produce, reported inside data rather than as an HTTP error.
func (s *Server) handleTestHook(c *gin.Context) {
	id, ok := hookIDParam(c)
	if !ok {
		return
	}
	h, err := hooks.Get(s.db, id)
	if err != nil {
		hookError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	var req testHookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		hookError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if req.Source != nil {
		if err := hooks.CompileCheck(*req.Source); err != nil {
			hookError(c, http.StatusBadRequest, "VALIDATION", err.Error())
			return
		}
		h.Source = *req.Source
	}

	res := hooks.Run(h, req.Old, req.New, hooks.RunConfig{
		DB: s.db, Write: s.write, AllowNet: s.allowNet, Record: true,
	})

	data := gin.H{
		"result":     res.Result,
		"message":    nilIfEmpty(res.Message),
		"logs":       res.Logs,
		"durationMs": res.DurationMs,
	}
	if res.Error != "" {
		data["error"] = res.Error
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": data})
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// GET /api/hooks/:id/log?limit=&offset=
func (s *Server) handleHookLog(c *gin.Context) {
	id, ok := hookIDParam(c)
	if !ok {
		return
	}
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	runs, err := hooks.Logs(s.db, id, limit, offset)
	if err != nil {
		hookError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	total, err := hooks.CountLogs(s.db, id)
	if err != nil {
		hookError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
		"runs": runs, "total": total, "limit": limit, "offset": offset,
	}})
}

// DELETE /api/hooks/:id/log
func (s *Server) handleClearHookLog(c *gin.Context) {
	id, ok := hookIDParam(c)
	if !ok {
		return
	}
	if err := hooks.ClearLogs(s.db, id); err != nil {
		status, code := http.StatusInternalServerError, "DB_ERROR"
		if strings.Contains(err.Error(), "not found") {
			status, code = http.StatusNotFound, "NOT_FOUND"
		}
		hookError(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"id": id}})
}
