package server

import (
	"database/sql"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/web"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router *gin.Engine
	db     *sql.DB
	dbPath string
	write  bool
}

func NewServer(database *sql.DB, dbPath string, write bool) *Server {
	// Disable debug logs by default to keep output clean, unless needed
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		router: gin.New(), // Use gin.New() and custom recovery/logger to control logging
		db:     database,
		dbPath: dbPath,
		write:  write,
	}

	s.router.Use(gin.Recovery())
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	api := s.router.Group("/api")
	{
		api.GET("/meta", s.handleMeta)
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

func (s *Server) handleMeta(c *gin.Context) {
	sqliteVer, size, err := db.Meta(s.db, s.dbPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	name := filepath.Base(s.dbPath)
	if s.dbPath == ":memory:" {
		name = ":memory:"
	}

	mode := "ro"
	if s.write {
		mode = "rw"
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"data": gin.H{
			"name":          name,
			"mode":          mode,
			"sqliteVersion": sqliteVer,
			"sizeBytes":     size,
		},
	})
}

func (s *Server) Start(addr string) error {
	return s.router.Run(addr)
}

func (s *Server) Handler() http.Handler {
	return s.router
}
