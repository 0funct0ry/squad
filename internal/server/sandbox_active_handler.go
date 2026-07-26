package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// POST /api/sandbox/active {id} — marks id as the sandbox database that
// /rest/* (and the REST control routes) resolve against. If the REST
// listener is currently running against a different database, it is
// auto-stopped (SPEC §5.7: switching the active DB while the listener is
// running stops it, since the mounted route snapshot belonged to the DB
// that's no longer active).
func (s *Server) handleSandboxSetActive(c *gin.Context) {
	var req struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "id is required"},
		})
		return
	}

	entry, ok := s.registry.Get(req.ID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"ok":    false,
			"error": gin.H{"code": "NOT_FOUND", "message": "sandbox database not found"},
		})
		return
	}

	if err := s.registry.SetActive(req.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	restStopped := s.restManager.NotifyActiveDBChanged(entry.DisplayName)

	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"activeId": req.ID, "restStopped": restStopped}})
}
