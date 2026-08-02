package server

import (
	"net/http"

	"github.com/0funct0ry/squad/internal/cli"
	"github.com/gin-gonic/gin"
)

// TransformTemplateRequest batches a "apply custom Go template" Data-tab
// transform preview/apply: render tmplText once per value, with that value
// bound to `{{.Value}}`, reusing internal/cli's generator/formula FuncMap
// (the same one `squad cli` SQL templating uses) via cli.RenderRowTransformTemplate.
type TransformTemplateRequest struct {
	Template string        `json:"template" binding:"required"`
	Values   []interface{} `json:"values" binding:"required"`
}

// POST /api/transform/template
//
// Pure computation over caller-supplied values — no database access, so this
// is available regardless of --write (same rationale as /api/tables/:name/seed's
// dry-run preview: it only becomes a write once the caller applies the result
// via the existing row-update endpoints, which are already write-gated).
func (s *Server) handleTransformTemplate(c *gin.Context) {
	var req TransformTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	results := make([]interface{}, len(req.Values))
	for i, v := range req.Values {
		rendered, err := cli.RenderRowTransformTemplate(req.Template, v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "VALIDATION", "message": err.Error()},
			})
			return
		}
		results[i] = rendered
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": gin.H{"results": results},
	})
}
