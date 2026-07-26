package server

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/restserver"
	"github.com/gin-gonic/gin"
)

var errNoActiveSandboxDB = errors.New("no active sandbox database selected")

// registerRestControlRoutes wires the main-API control surface for the
// separate REST listener (start/stop/status/per-table config). These stay on
// the ordinary {ok,data} envelope, unlike /rest/* itself. They act on
// whichever database is "current" for this Server (the fixed single DB, or
// the sandbox registry's active entry).
func (s *Server) registerRestControlRoutes(api *gin.RouterGroup) {
	rest := api.Group("/rest")
	rest.GET("/status", s.handleRestStatus)
	rest.POST("/start", s.handleRestStart)
	rest.POST("/stop", s.handleRestStop)
	rest.GET("/tables", s.handleRestListTables)
	rest.PATCH("/tables/:name", s.handleRestUpdateTableConfig)
}

func (s *Server) handleRestStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": s.restManager.Status()})
}

func (s *Server) handleRestStart(c *gin.Context) {
	if err := s.restManager.Start(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "REST_START_FAILED", "message": err.Error()},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": s.restManager.Status()})
}

func (s *Server) handleRestStop(c *gin.Context) {
	if err := s.restManager.Stop("manual"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "REST_STOP_FAILED", "message": err.Error()},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": s.restManager.Status()})
}

// restCurrentDB resolves the DB/scope this Server's REST subsystem currently
// targets: the fixed single DB, or the sandbox registry's active entry.
func (s *Server) restCurrentDB() (conn *sql.DB, scope string, err error) {
	if s.registry == nil {
		return s.db, "", nil
	}
	id, ok := s.registry.ActiveID()
	if !ok {
		return nil, "", errNoActiveSandboxDB
	}
	entry, ok := s.registry.Get(id)
	if !ok {
		return nil, "", errNoActiveSandboxDB
	}
	return entry.DB, entry.ID, nil
}

// GET /api/rest/tables — lists every exposable table/view (SQLite-internal
// tables already excluded by db.GetTables) joined with its current REST
// config, resolved key-route info, and whether the running snapshot (if any)
// differs from the live config ("restart needed").
func (s *Server) handleRestListTables(c *gin.Context) {
	conn, scope, err := s.restCurrentDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": []restserver.TableStatus{}})
		return
	}

	tables, err := db.GetTables(conn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	running, snapScope, snapTables := s.restManager.RunningSnapshotTables()
	snapshotActive := running && snapScope == scope

	out := make([]restserver.TableStatus, 0, len(tables))
	for _, t := range tables {
		schema, err := db.GetTableSchema(conn, t.Name)
		if err != nil {
			continue
		}
		cfg := s.restConfigs.Get(scope, t.Name)
		liveInfo := restserver.ResolveRouteInfo(t, schema, cfg, s.write)

		ts := restserver.TableStatus{
			Name:         t.Name,
			Type:         t.Type,
			HasPKRoute:   liveInfo.HasPKRoute,
			WriteAllowed: s.write,
			Exposed:      cfg.Exposed,
			Create:       liveInfo.Create,
			Update:       liveInfo.Update,
			Delete:       liveInfo.Delete,
		}

		if snapshotActive {
			if snapInfo, ok := snapTables[t.Name]; ok {
				ts.SnapshotExposed = true
				ts.SnapshotCreate = snapInfo.Create
				ts.SnapshotUpdate = snapInfo.Update
				ts.SnapshotDelete = snapInfo.Delete
			}
			ts.RestartNeeded = ts.SnapshotExposed != ts.Exposed ||
				ts.SnapshotCreate != ts.Create ||
				ts.SnapshotUpdate != ts.Update ||
				ts.SnapshotDelete != ts.Delete
		}

		out = append(out, ts)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "data": out})
}

type restTableConfigUpdate struct {
	Exposed *bool `json:"exposed"`
	Create  *bool `json:"create"`
	Update  *bool `json:"update"`
	Delete  *bool `json:"delete"`
}

// PATCH /api/rest/tables/:name — partial update of a table's REST toggles.
func (s *Server) handleRestUpdateTableConfig(c *gin.Context) {
	name := c.Param("name")

	conn, scope, err := s.restCurrentDB()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "NO_ACTIVE_DB", "message": err.Error()},
		})
		return
	}

	schema, err := db.GetTableSchema(conn, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"ok":    false,
			"error": gin.H{"code": "NOT_FOUND", "message": "table or view not found"},
		})
		return
	}

	var req restTableConfigUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	isView := schema.Type == "view"
	wantsWrite := (req.Create != nil && *req.Create) || (req.Update != nil && *req.Update) || (req.Delete != nil && *req.Delete)
	if wantsWrite && isView {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "views only support the GET routes; create/update/delete cannot be enabled"},
		})
		return
	}
	if wantsWrite && !s.write {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "start squad with --write to enable create/update/delete toggles"},
		})
		return
	}

	cfg := s.restConfigs.Get(scope, name)
	if req.Exposed != nil {
		cfg.Exposed = *req.Exposed
	}
	if req.Create != nil {
		cfg.Create = *req.Create
	}
	if req.Update != nil {
		cfg.Update = *req.Update
	}
	if req.Delete != nil {
		cfg.Delete = *req.Delete
	}
	if isView {
		cfg.Create, cfg.Update, cfg.Delete = false, false, false
	}
	s.restConfigs.Set(scope, name, cfg)

	c.JSON(http.StatusOK, gin.H{"ok": true, "data": cfg})
}
