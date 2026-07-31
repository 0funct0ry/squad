package server

import (
	"net/http"
	"strconv"

	"github.com/0funct0ry/squad/internal/vtab"
	"github.com/gin-gonic/gin"
)

// registerModulesRoutes wires the --modules control surface for the
// single-DB flow, following registerRestControlRoutes's pattern: the routes
// are always mounted, and each handler checks s.modulesEnabled itself
// (MODULES_DISABLED, not 404, when --modules wasn't passed) — never gated on
// --write, since mounts only ever touch the temp schema.
func (s *Server) registerModulesRoutes(api *gin.RouterGroup) {
	mods := api.Group("/modules")
	mods.GET("", s.handleModulesInfo)
	mods.GET("/mounts", s.handleListMounts)
	mods.POST("/mounts", s.handleCreateMount)
	mods.DELETE("/mounts/:alias", s.handleDeleteMount)
	mods.POST("/mounts/:alias/preview", s.handlePreviewMount)
}

func modulesDisabledError(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{
		"ok":    false,
		"error": gin.H{"code": "MODULES_DISABLED", "message": "virtual table modules are off; relaunch with --modules to enable them"},
	})
}

func mountToView(m vtab.Mount) gin.H {
	return gin.H{
		"alias":           m.Alias,
		"module":          m.Module,
		"args":            m.Args,
		"declaredColumns": m.DeclaredColumns,
		"createdAt":       m.CreatedAt,
	}
}

func (s *Server) handleModulesInfo(c *gin.Context) {
	mounts := s.mountStore.List()
	views := make([]gin.H, 0, len(mounts))
	for _, m := range mounts {
		views = append(views, mountToView(m))
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
		"enabled":     s.modulesEnabled,
		"write":       s.write,
		"modulesRoot": s.modulesRoot,
		"catalog":     vtab.Catalog(),
		"mounts":      views,
	}})
}

func (s *Server) handleListMounts(c *gin.Context) {
	if !s.modulesEnabled {
		modulesDisabledError(c)
		return
	}
	mounts := s.mountStore.List()
	views := make([]gin.H, 0, len(mounts))
	for _, m := range mounts {
		views = append(views, mountToView(m))
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": views})
}

type createMountRequest struct {
	Module string            `json:"module"`
	Alias  string            `json:"alias"`
	Args   map[string]string `json:"args"`
}

func (s *Server) handleCreateMount(c *gin.Context) {
	if !s.modulesEnabled {
		modulesDisabledError(c)
		return
	}
	var req createMountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": err.Error()}})
		return
	}

	m, err := vtab.CreateMount(c.Request.Context(), s.db, s.mountStore, req.Module, req.Alias, req.Args)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "MOUNT_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "data": mountToView(m)})
}

func (s *Server) handleDeleteMount(c *gin.Context) {
	if !s.modulesEnabled {
		modulesDisabledError(c)
		return
	}
	alias := c.Param("alias")
	if !vtab.DropMount(s.mountStore, alias) {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": gin.H{"code": "NOT_FOUND", "message": "no active mount named " + alias}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"alias": alias}})
}

func (s *Server) handlePreviewMount(c *gin.Context) {
	if !s.modulesEnabled {
		modulesDisabledError(c)
		return
	}
	alias := c.Param("alias")
	limit := 20
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	cols, rows, err := vtab.PreviewMount(c.Request.Context(), s.db, s.mountStore, alias, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "MOUNT_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"columns": cols, "rows": rows}})
}
