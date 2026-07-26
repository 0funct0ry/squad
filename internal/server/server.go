package server

import (
	"database/sql"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/web"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router   *gin.Engine
	db       *sql.DB
	dbPath   string
	write    bool
	examples bool
	registry *db.Registry // non-nil in sandbox mode; nil for the single-DB flow
}

func NewServer(database *sql.DB, dbPath string, write bool, examplesEnabled bool) *Server {
	// Disable debug logs by default to keep output clean, unless needed
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		router:   gin.New(), // Use gin.New() and custom recovery/logger to control logging
		db:       database,
		dbPath:   dbPath,
		write:    write,
		examples: examplesEnabled,
	}

	s.router.Use(gin.Recovery())
	s.setupRoutes()
	return s
}

// NewSandboxServer starts a Server with no fixed database — every request is
// routed through the registry to a per-database connection instead.
func NewSandboxServer(registry *db.Registry, examplesEnabled bool) *Server {
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		router:   gin.New(),
		registry: registry,
		examples: examplesEnabled,
	}

	s.router.Use(gin.Recovery())
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	api := s.router.Group("/api")
	if s.registry == nil {
		s.setupSingleDBRoutes(api)
	} else {
		s.setupSandboxRoutes(api)
	}

	// Embedded SPA serving
	distFS, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		panic(err)
	}

	// Serve static files and fallback to index.html
	s.router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{
				"ok":    false,
				"error": gin.H{"code": "NOT_FOUND", "message": "API endpoint not found"},
			})
			return
		}

		// Clean path for fs.Open
		filePath := strings.TrimPrefix(path, "/")
		if filePath == "" {
			filePath = "index.html"
		}

		// Check if file is index.html to serve directly and avoid redirect loops
		if filePath == "index.html" {
			indexData, err := fs.ReadFile(distFS, "index.html")
			if err != nil {
				c.String(http.StatusInternalServerError, "Internal Server Error")
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
			return
		}

		// Check if file exists in embed FS
		file, err := distFS.Open(filePath)
		if err == nil {
			file.Close()
			c.FileFromFS(filePath, http.FS(distFS))
			return
		}

		// Fallback to index.html for client routing
		indexData, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
	})
}

func (s *Server) setupSingleDBRoutes(api *gin.RouterGroup) {
	api.GET("/meta", s.handleMeta)
	api.GET("/tables", s.handleTables)
	api.GET("/tables/:name/schema", s.handleTableSchema)
	api.GET("/tables/:name/rows", s.handleTableRows)
	api.POST("/query", s.handleQuery)
	api.GET("/tables/:name/export", s.handleTableExport)
	api.POST("/export/query", s.handleQueryExport)

	// Write-mode DDL & table editor endpoints
	api.POST("/ddl", s.WriteGateMiddleware("executing DDL"), s.handlePostDDL)
	api.POST("/tables", s.WriteGateMiddleware("creating table"), s.handleCreateTable)
	api.PATCH("/tables/:name", s.WriteGateMiddleware("altering table"), s.handleAlterTable)
	api.DELETE("/tables/:name", s.WriteGateMiddleware("dropping table"), s.handleDropTable)
	api.POST("/tables/:name/rows", s.WriteGateMiddleware("inserting row"), s.handleInsertRow)
	api.PATCH("/tables/:name/rows", s.WriteGateMiddleware("updating row"), s.handleUpdateRow)
	api.DELETE("/tables/:name/rows", s.WriteGateMiddleware("deleting row"), s.handleDeleteRow)
	api.GET("/tables/:name/seed/plan", s.WriteGateMiddleware("seeding table"), s.handleSeedPlan)
	api.POST("/tables/:name/seed", s.WriteGateMiddleware("seeding table"), s.handleSeedTable)
	api.GET("/seed/generators/:name/sample", s.WriteGateMiddleware("seeding table"), s.handleSeedGeneratorSample)

	s.registerExamplesRoutes(api)
}

func (s *Server) handleMeta(c *gin.Context) {
	meta, err := db.GetDBMeta(s.db, s.dbPath, s.write)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": meta,
	})
}

func (s *Server) handleTables(c *gin.Context) {
	tables, err := db.GetTables(s.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": tables,
	})
}

func (s *Server) handleTableSchema(c *gin.Context) {
	name := c.Param("name")
	schema, err := db.GetTableSchema(s.db, name)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"ok":    false,
				"error": gin.H{"code": "NOT_FOUND", "message": "table or view not found"},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": schema,
	})
}

func (s *Server) handleTableRows(c *gin.Context) {
	name := c.Param("name")

	// Parse parameters
	limit := 100
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if oStr := c.Query("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	orderBy := c.Query("orderBy")
	dir := c.Query("dir")

	// Parse column filters (e.g. filter[id]=12 or filter[email]=ada)
	filters := make(map[string]string)
	queries := c.Request.URL.Query()
	for k, v := range queries {
		if strings.HasPrefix(k, "filter[") && strings.HasSuffix(k, "]") && len(v) > 0 {
			col := k[7 : len(k)-1]
			filters[col] = v[0]
		}
	}

	params := db.RowQueryParams{
		Limit:   limit,
		Offset:  offset,
		OrderBy: orderBy,
		Dir:     dir,
		Filters: filters,
	}

	result, err := db.GetTableRows(s.db, name, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"data": gin.H{
			"columns": result.Columns,
			"rows":    result.Rows,
			"total":   result.Total,
		},
	})
}

func (s *Server) Start(addr string) error {
	return s.router.Run(addr)
}

func (s *Server) Handler() http.Handler {
	return s.router
}
