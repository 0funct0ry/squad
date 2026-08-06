package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/gin-gonic/gin"
)

// setupSandboxRoutes wires the /api/sandbox/* namespace: registry-management
// routes plus a 1:1 mirror of every single-DB route, scoped per :id.
func (s *Server) setupSandboxRoutes(api *gin.RouterGroup) {
	sandbox := api.Group("/sandbox/dbs")
	{
		sandbox.GET("", s.handleSandboxListDBs)
		sandbox.POST("", s.handleSandboxUploadDB)
		sandbox.POST("/new", s.handleSandboxCreateDB)
		sandbox.DELETE("/:id", s.handleSandboxDeleteDB)
		sandbox.PATCH("/:id", s.handleSandboxRenameDB)
		sandbox.GET("/:id/download", s.handleSandboxDownloadDB)
		sandbox.POST("/active", s.handleSandboxSetActive)
	}

	scoped := sandbox.Group("/:id")
	scoped.Use(s.sandboxResolveMiddleware())
	{
		scoped.GET("/meta", scopedHandler((*Server).handleMeta))
		scoped.GET("/tables", scopedHandler((*Server).handleTables))
		scoped.GET("/tables/:name/schema", scopedHandler((*Server).handleTableSchema))
		scoped.GET("/tables/:name/rows", scopedHandler((*Server).handleTableRows))
		scoped.POST("/query", scopedHandler((*Server).handleQuery))
		scoped.GET("/tables/:name/export", scopedHandler((*Server).handleTableExport))
		scoped.POST("/export/query", scopedHandler((*Server).handleQueryExport))

		scoped.POST("/ddl", scopedHandler((*Server).handlePostDDL))
		scoped.POST("/tables", scopedHandler((*Server).handleCreateTable))
		scoped.PATCH("/tables/:name", scopedHandler((*Server).handleAlterTable))
		scoped.DELETE("/tables/:name", scopedHandler((*Server).handleDropTable))
		scoped.POST("/tables/:name/rows", scopedHandler((*Server).handleInsertRow))
		scoped.PATCH("/tables/:name/rows", scopedHandler((*Server).handleUpdateRow))
		scoped.DELETE("/tables/:name/rows", scopedHandler((*Server).handleDeleteRow))
		scoped.POST("/tables/:name/rows/bulk-delete", scopedHandler((*Server).handleBulkDeleteRows))
		scoped.POST("/transform/template", scopedHandler((*Server).handleTransformTemplate))
		scoped.POST("/import/preview", scopedHandler((*Server).handleImportPreview))
		scoped.POST("/tables/:name/import", scopedHandler((*Server).handleImportIntoTable))
		scoped.POST("/tables/import", scopedHandler((*Server).handleImportCreateTable))
		scoped.GET("/tables/:name/seed/plan", scopedHandler((*Server).handleSeedPlan))
		scoped.POST("/tables/:name/seed", scopedHandler((*Server).handleSeedTable))
		scoped.GET("/seed/generators/catalog", scopedHandler((*Server).handleSeedGeneratorsCatalog))
		scoped.GET("/seed/generators/:name/sample", scopedHandler((*Server).handleSeedGeneratorSample))

		scoped.GET("/modules", scopedHandler((*Server).handleModulesInfo))
		scoped.GET("/modules/mounts", scopedHandler((*Server).handleListMounts))
		scoped.POST("/modules/mounts", scopedHandler((*Server).handleCreateMount))
		scoped.DELETE("/modules/mounts/:alias", scopedHandler((*Server).handleDeleteMount))
		scoped.POST("/modules/mounts/:alias/preview", scopedHandler((*Server).handlePreviewMount))
	}

	s.registerExamplesRoutes(api)
}

const scopedServerKey = "scopedServer"

// sandboxResolveMiddleware resolves :id against the registry and stashes a
// throwaway *Server scoped to that entry's connection in the gin context.
// Sandbox databases are always read-write — there is no READ_ONLY path.
func (s *Server) sandboxResolveMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		entry, ok := s.registry.Get(id)
		if !ok {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"ok":    false,
				"error": gin.H{"code": "NOT_FOUND", "message": "sandbox database not found"},
			})
			return
		}
		c.Set(scopedServerKey, &Server{
			router:         s.router,
			db:             entry.DB,
			dbPath:         entry.Path,
			write:          true,
			modulesEnabled: s.modulesEnabled,
			modulesRoot:    s.modulesRoot,
			mountStore:     s.mountStore,
		})
		c.Next()
		if c.Request.Method != http.MethodGet {
			s.registry.Touch(id)
		}
	}
}

// scopedHandler adapts an existing (*Server).handleX method value to run
// against the request-scoped *Server stashed by sandboxResolveMiddleware,
// reusing every existing handler body unmodified.
func scopedHandler(fn func(*Server, *gin.Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		fn(c.MustGet(scopedServerKey).(*Server), c)
	}
}

type sandboxDBView struct {
	ID             string `json:"id"`
	DisplayName    string `json:"displayName"`
	SizeBytes      int64  `json:"sizeBytes"`
	CreatedAt      string `json:"createdAt"`
	LastModifiedAt string `json:"lastModifiedAt"`
}

func toSandboxDBView(e db.RegistryEntry) sandboxDBView {
	return sandboxDBView{
		ID:             e.ID,
		DisplayName:    e.DisplayName,
		SizeBytes:      e.SizeBytes,
		CreatedAt:      e.CreatedAt.Format(timeFormat),
		LastModifiedAt: e.LastModifiedAt.Format(timeFormat),
	}
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

// GET /api/sandbox/dbs
func (s *Server) handleSandboxListDBs(c *gin.Context) {
	entries := s.registry.List()
	views := make([]sandboxDBView, 0, len(entries))
	for _, e := range entries {
		views = append(views, toSandboxDBView(e))
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": views})
}

// POST /api/sandbox/dbs (multipart/form-data: file, optional name)
func (s *Server) handleSandboxUploadDB(c *gin.Context) {
	maxBytes := s.registry.MaxUploadBytes()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "FILE_TOO_LARGE",
					"message": fmt.Sprintf("upload exceeds max upload size of %d bytes", maxBytes),
				},
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "multipart field \"file\" is required"},
		})
		return
	}
	if fileHeader.Size > maxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"ok": false,
			"error": gin.H{
				"code":    "FILE_TOO_LARGE",
				"message": fmt.Sprintf("file exceeds max upload size of %d bytes", maxBytes),
			},
		})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "failed to read uploaded file"},
		})
		return
	}
	defer f.Close()

	displayName := c.PostForm("name")
	if displayName == "" {
		displayName = fileHeader.Filename
	}

	entry, err := s.registry.Add(displayName, f)
	if err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "FILE_TOO_LARGE",
					"message": fmt.Sprintf("file exceeds max upload size of %d bytes", maxBytes),
				},
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "INVALID_SQLITE_FILE", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "data": toSandboxDBView(*entry)})
}

// POST /api/sandbox/dbs/new (JSON: {name})
func (s *Server) handleSandboxCreateDB(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "name is required"},
		})
		return
	}

	entry, err := s.registry.Create(req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "data": toSandboxDBView(*entry)})
}

// DELETE /api/sandbox/dbs/:id
func (s *Server) handleSandboxDeleteDB(c *gin.Context) {
	id := c.Param("id")
	activeID, hasActive := s.registry.ActiveID()
	wasActive := hasActive && activeID == id

	if err := s.registry.Remove(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"ok":    false,
			"error": gin.H{"code": "NOT_FOUND", "message": err.Error()},
		})
		return
	}

	s.registry.ClearActiveIfMatches(id)
	s.restConfigs.Forget(id)
	if wasActive {
		s.restManager.NotifyActiveDBChanged("")
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"id": id}})
}

// PATCH /api/sandbox/dbs/:id (JSON: {displayName})
func (s *Server) handleSandboxRenameDB(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		DisplayName string `json:"displayName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.DisplayName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "displayName is required"},
		})
		return
	}
	if err := s.registry.Rename(id, req.DisplayName); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"ok":    false,
			"error": gin.H{"code": "NOT_FOUND", "message": err.Error()},
		})
		return
	}
	entry, _ := s.registry.Get(id)
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": toSandboxDBView(*entry)})
}

// GET /api/sandbox/dbs/:id/download
func (s *Server) handleSandboxDownloadDB(c *gin.Context) {
	id := c.Param("id")
	entry, ok := s.registry.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"ok":    false,
			"error": gin.H{"code": "NOT_FOUND", "message": "sandbox database not found"},
		})
		return
	}

	filename := sanitizeFilename(entry.DisplayName) + ".sqlite"
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.File(entry.Path)
}
