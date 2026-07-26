package server

import (
	"net/http"

	"github.com/0funct0ry/squad/internal/examples"
	"github.com/gin-gonic/gin"
)

// registerExamplesRoutes registers GET /api/examples and
// GET /api/examples/:slug, but only when --examples was passed at startup.
// When disabled, these routes are simply never registered, so requests fall
// through to the standard NoRoute JSON-404 handler.
func (s *Server) registerExamplesRoutes(api *gin.RouterGroup) {
	if !s.examples {
		return
	}
	api.GET("/examples", s.handleListExamples)
	api.GET("/examples/:slug", s.handleGetExample)
}

func (s *Server) handleListExamples(c *gin.Context) {
	all := examples.All()
	metas := make([]examples.Meta, 0, len(all))
	for _, e := range all {
		metas = append(metas, examples.Meta{Slug: e.Slug, Name: e.Name, Description: e.Description})
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": metas,
	})
}

func (s *Server) handleGetExample(c *gin.Context) {
	slug := c.Param("slug")
	example, ok := examples.ByName(slug)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"ok":    false,
			"error": gin.H{"code": "NOT_FOUND", "message": "example not found"},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": example,
	})
}
