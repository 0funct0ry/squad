package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/hooks"
	"github.com/0funct0ry/squad/internal/restserver"
	"github.com/0funct0ry/squad/internal/vtab"
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
	logger   *slog.Logger

	restManager *restserver.Manager
	restConfigs *restserver.ConfigStore

	// Virtual table modules (--modules). modulesEnabled/modulesRoot are
	// captured once (like restManager's enabled/write fields) via
	// EnableModules, called right after construction when --modules was
	// passed; mountStore is always non-nil so route handlers can operate on
	// it uniformly, but every mount route is refused with MODULES_DISABLED
	// while modulesEnabled is false.
	modulesEnabled bool
	modulesRoot    string
	mountStore     *vtab.MountStore

	// Lua trigger hooks (M10c), gated behind --hooks like --modules.
	// GET /api/hooks and GET /api/hooks/:id stay always accessible (mirrors
	// GET /api/modules) so the web UI can decide whether to show the Hooks
	// tab at all; every other route is refused with HOOKS_DISABLED while
	// hooksEnabled is false. hookMode/allowNet are the resolved process
	// flags reported in the Hooks tab's status strip and passed into every
	// hook run, independent of whether the feature is enabled.
	hooksEnabled bool
	hookMode     string
	allowNet     bool
}

// ConfigureHooks records the resolved --hook-mode/--allow-net flags for the
// Hooks tab's status strip and hook execution.
func (s *Server) ConfigureHooks(mode string, allowNet bool) {
	if mode != "async" {
		mode = "sync"
	}
	s.hookMode = mode
	s.allowNet = allowNet
}

// EnableHooks turns on the --hooks routes for this server (create/update/
// delete/test/log). Called right after construction when --hooks was
// passed, mirroring EnableModules.
func (s *Server) EnableHooks() {
	s.hooksEnabled = true
}

// EnableModules turns on the --modules routes for this server, recording the
// file confinement root used by file-reading modules.
func (s *Server) EnableModules(modulesRoot string) {
	s.modulesEnabled = true
	s.modulesRoot = modulesRoot
}

// parseLogLevel maps the --log-level flag value to an slog.Level, defaulting
// to info on empty/unrecognized input.
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// errBodyWriter buffers the response body only for error (>=400) responses,
// so it can be parsed for the envelope's error code/message without
// buffering every response body.
type errBodyWriter struct {
	gin.ResponseWriter
	buf bytes.Buffer
}

func (w *errBodyWriter) Write(b []byte) (int, error) {
	if w.Status() >= http.StatusBadRequest {
		w.buf.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// loggingMiddleware logs one info-level line per request (method, path,
// status, duration) and an additional error-level line, with the envelope's
// code/message, whenever the response was an error (status >= 400).
func loggingMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		ebw := &errBodyWriter{ResponseWriter: c.Writer}
		c.Writer = ebw

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		logger.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"duration_ms", duration.Milliseconds(),
		)

		if status >= http.StatusBadRequest && ebw.buf.Len() > 0 {
			var envelope struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(ebw.buf.Bytes(), &envelope); err == nil {
				logger.Error("request failed",
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"status", status,
					"code", envelope.Error.Code,
					"message", envelope.Error.Message,
				)
			}
		}
	}
}

// singleDBProvider implements restserver.DBProvider for the fixed single-DB
// flow: there is exactly one connection and one scope ("").
type singleDBProvider struct {
	db     *sql.DB
	dbPath string
}

func (p *singleDBProvider) CurrentDB() (*sql.DB, string, string, error) {
	return p.db, "", p.dbPath, nil
}

// sandboxDBProvider implements restserver.DBProvider for sandbox mode: the
// connection to serve is whichever sandbox database is currently marked
// active (there is no /rest/:dbId/:table form — see SPEC §5.7).
type sandboxDBProvider struct {
	registry *db.Registry
}

func (p *sandboxDBProvider) CurrentDB() (*sql.DB, string, string, error) {
	id, ok := p.registry.ActiveID()
	if !ok {
		return nil, "", "", fmt.Errorf("no active sandbox database selected")
	}
	entry, ok := p.registry.Get(id)
	if !ok {
		return nil, "", "", fmt.Errorf("active sandbox database %q no longer exists", id)
	}
	return entry.DB, entry.ID, entry.DisplayName, nil
}

func NewServer(database *sql.DB, dbPath string, write bool, examplesEnabled bool, restEnabled bool, restBindAddr string, restPort int, logLevel string) *Server {
	// Disable debug logs by default to keep output clean, unless needed
	gin.SetMode(gin.ReleaseMode)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: parseLogLevel(logLevel)}))

	s := &Server{
		router:   gin.New(), // Use gin.New() and custom recovery/logger to control logging
		db:       database,
		dbPath:   dbPath,
		write:    write,
		examples: examplesEnabled,
		logger:   logger,
		hookMode: hooks.Mode(),
		allowNet: hooks.Current().AllowNet,
	}

	s.restConfigs = restserver.NewConfigStore()
	s.mountStore = vtab.NewMountStore()
	s.restManager = restserver.NewManager(restEnabled, write, restBindAddr, restPort, &singleDBProvider{db: database, dbPath: dbPath}, s.restConfigs)

	s.router.Use(gin.Recovery())
	s.router.Use(loggingMiddleware(logger))
	s.setupRoutes()
	return s
}

// NewSandboxServer starts a Server with no fixed database — every request is
// routed through the registry to a per-database connection instead.
func NewSandboxServer(registry *db.Registry, examplesEnabled bool, restEnabled bool, restBindAddr string, restPort int, logLevel string) *Server {
	gin.SetMode(gin.ReleaseMode)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: parseLogLevel(logLevel)}))

	s := &Server{
		router: gin.New(),
		// Sandbox databases are always read-write — this also feeds the
		// REST control routes' writeAllowed/toggle-validation logic
		// (handleRestListTables/handleRestUpdateTableConfig), which read
		// s.write directly rather than going through restManager.
		write:    true,
		registry: registry,
		examples: examplesEnabled,
		logger:   logger,
		hookMode: hooks.Mode(),
		allowNet: hooks.Current().AllowNet,
	}

	s.restConfigs = restserver.NewConfigStore()
	s.mountStore = vtab.NewMountStore()
	s.restManager = restserver.NewManager(restEnabled, true, restBindAddr, restPort, &sandboxDBProvider{registry: registry}, s.restConfigs)

	s.router.Use(gin.Recovery())
	s.router.Use(loggingMiddleware(logger))
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
	s.registerRestControlRoutes(api)

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
	api.POST("/tables/:name/rows/bulk-delete", s.WriteGateMiddleware("deleting rows"), s.handleBulkDeleteRows)
	api.POST("/transform/template", s.handleTransformTemplate)
	api.POST("/import/preview", s.handleImportPreview)
	api.POST("/tables/:name/import", s.WriteGateMiddleware("importing rows"), s.handleImportIntoTable)
	api.POST("/tables/import", s.WriteGateMiddleware("importing rows"), s.handleImportCreateTable)
	api.GET("/tables/:name/seed/plan", s.WriteGateMiddleware("seeding table"), s.handleSeedPlan)
	api.POST("/tables/:name/seed", s.WriteGateMiddleware("seeding table"), s.handleSeedTable)
	api.GET("/seed/generators/catalog", s.handleSeedGeneratorsCatalog)
	api.GET("/seed/generators/:name/sample", s.WriteGateMiddleware("seeding table"), s.handleSeedGeneratorSample)

	s.registerExamplesRoutes(api)
	s.registerModulesRoutes(api)
	s.registerFunctionsRoutes(api)
	s.registerHooksRoutes(api)
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
	var schema *db.TableSchema
	err := vtab.WithMounts(c.Request.Context(), s.db, s.mountStore, func(conn *sql.Conn) error {
		var err error
		schema, err = db.GetTableSchema(db.WrapConn(conn), name)
		return err
	})
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

	filters, err := parseRowFilters(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "VALIDATION", "message": err.Error()},
		})
		return
	}

	params := db.RowQueryParams{
		Limit:   limit,
		Offset:  offset,
		OrderBy: orderBy,
		Dir:     dir,
		Filters: filters,
	}

	var result *db.RowResult
	err = vtab.WithMounts(c.Request.Context(), s.db, s.mountStore, func(conn *sql.Conn) error {
		var err error
		result, err = db.GetTableRows(db.WrapConn(conn), name, params)
		return err
	})
	if err != nil {
		if errors.Is(err, db.ErrFilterUnsupported) {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "VALIDATION", "message": err.Error()},
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

// StopRest stops the REST listener, if running. Safe to call unconditionally
// (e.g. on process shutdown) even if the listener was never started.
func (s *Server) StopRest() {
	_ = s.restManager.Stop("process exit")
}
