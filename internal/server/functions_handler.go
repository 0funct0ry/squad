package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/0funct0ry/squad/internal/udf"
	"github.com/gin-gonic/gin"
)

// registerFunctionsRoutes wires the always-on curated UDF catalog + try
// endpoints (M10b). Unlike --modules, there's no enable flag: the functions
// are registered process-wide regardless of --write, so both routes are
// unconditionally available, read-only-safe.
func (s *Server) registerFunctionsRoutes(api *gin.RouterGroup) {
	api.GET("/functions", s.handleFunctionsCatalog)
	api.POST("/functions/try", s.handleFunctionsTry)
}

func (s *Server) handleFunctionsCatalog(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
		"categories": udf.Catalog(),
	}})
}

type functionsTryRequest struct {
	Name string `json:"name"`
	Args []any  `json:"args"`
}

func (s *Server) handleFunctionsTry(c *gin.Context) {
	var req functionsTryRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "a function name is required"},
		})
		return
	}

	d, ok := udf.Find(req.Name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"ok":    false,
			"error": gin.H{"code": "NOT_FOUND", "message": fmt.Sprintf("no such function %q", req.Name)},
		})
		return
	}
	if d.Aggregate {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok": false,
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": fmt.Sprintf("%s is an aggregate function and can't be tried against bare args", d.Name),
			},
		})
		return
	}

	placeholders := make([]string, len(req.Args))
	args := make([]any, len(req.Args))
	for i, a := range req.Args {
		placeholders[i] = "?"
		args[i] = a
	}
	query := fmt.Sprintf("SELECT %s(%s)", d.Name, strings.Join(placeholders, ", "))

	var result any
	if err := s.db.QueryRow(query, args...).Scan(&result); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"result": result}})
}
