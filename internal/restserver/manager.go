package restserver

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/gin-gonic/gin"
)

// DBProvider resolves the database a Manager should currently serve.
type DBProvider interface {
	// CurrentDB returns the connection to serve, a stable scope key for
	// ConfigStore lookups ("" in single-DB mode, the sandbox entry ID
	// otherwise), and a human-readable label used for status display and
	// active-database-changed detection. err is non-nil when there's
	// nothing to serve (sandbox mode with no active database selected).
	CurrentDB() (conn *sql.DB, scope string, label string, err error)
}

// snapshot is the one-time route table built at Start() from the
// then-current per-table config; later ConfigStore changes don't affect a
// running listener until the next Start().
type snapshot struct {
	dbLabel string
	scope   string
	tables  map[string]RouteInfo
}

// TableStatus is the per-table view returned by Status(): live configured
// state plus (when running) the snapshot the listener was actually built
// from, so callers can detect "restart needed" without duplicating the diff
// logic.
type TableStatus struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	HasPKRoute      bool   `json:"hasPkRoute"`
	WriteAllowed    bool   `json:"writeAllowed"`
	Exposed         bool   `json:"exposed"`
	Create          bool   `json:"create"`
	Update          bool   `json:"update"`
	Delete          bool   `json:"delete"`
	SnapshotExposed bool   `json:"snapshotExposed"`
	SnapshotCreate  bool   `json:"snapshotCreate"`
	SnapshotUpdate  bool   `json:"snapshotUpdate"`
	SnapshotDelete  bool   `json:"snapshotDelete"`
	RestartNeeded   bool   `json:"restartNeeded"`
}

// StatusPayload is the JSON shape returned by GET /api/rest/status.
type StatusPayload struct {
	Enabled        bool   `json:"enabled"`
	Write          bool   `json:"write"`
	Running        bool   `json:"running"`
	BindAddr       string `json:"bindAddr"`
	Port           int    `json:"port"`
	DBLabel        string `json:"dbLabel,omitempty"`
	StartedAt      string `json:"startedAt,omitempty"`
	LastStopReason string `json:"lastStopReason,omitempty"`
}

// Manager owns the lifecycle of the separate REST listener: building a
// route-table snapshot from the current per-table config at Start(), serving
// it until Stop(), and never mutating that snapshot from later config
// changes (SPEC §5.7's explicit "restart to apply" contract).
type Manager struct {
	mu       sync.Mutex
	enabled  bool // = --rest at launch; immutable after construction
	write    bool // = --write at launch; immutable after construction
	bindAddr string
	port     int
	provider DBProvider
	configs  *ConfigStore

	httpServer     *http.Server
	running        bool
	snap           *snapshot
	startedAt      time.Time
	lastStopReason string
}

// NewManager constructs a Manager. enabled/write are captured once (they
// mirror --rest/--write at process launch and never change).
func NewManager(enabled, write bool, bindAddr string, port int, provider DBProvider, configs *ConfigStore) *Manager {
	return &Manager{
		enabled:  enabled,
		write:    write,
		bindAddr: bindAddr,
		port:     port,
		provider: provider,
		configs:  configs,
	}
}

// Enabled reports whether --rest was passed at launch.
func (m *Manager) Enabled() bool { return m.enabled }

// Start snapshots the current per-table config and schema, builds a fresh
// route table from it, and starts the listener. Only mounts write routes
// when both --write and the table's toggle are on, per SPEC §5.7 — a
// disabled write route is simply absent from the snapshot, not 403-gated.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		return fmt.Errorf("--rest was not passed at launch")
	}
	if m.running {
		return fmt.Errorf("REST listener is already running")
	}

	conn, scope, label, err := m.provider.CurrentDB()
	if err != nil {
		return fmt.Errorf("no active database to serve: %w", err)
	}

	tables, err := db.GetTables(conn)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}

	cfgSnapshot := m.configs.Snapshot(scope)
	routeTables := make(map[string]RouteInfo)
	for _, t := range tables {
		cfg := cfgSnapshot[t.Name]
		if !cfg.Exposed {
			continue
		}
		schema, err := db.GetTableSchema(conn, t.Name)
		if err != nil {
			continue // skip tables we can't introspect rather than failing the whole start
		}
		routeTables[t.Name] = ResolveRouteInfo(t, schema, cfg, m.write)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	registerRoutes(engine, conn, routeTables)

	m.snap = &snapshot{dbLabel: label, scope: scope, tables: routeTables}
	m.httpServer = &http.Server{Addr: fmt.Sprintf("%s:%d", m.bindAddr, m.port), Handler: engine}
	m.startedAt = time.Now()
	m.lastStopReason = ""
	m.running = true

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.httpServer.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			m.running = false
			m.snap = nil
			m.httpServer = nil
			return fmt.Errorf("failed to start REST listener: %w", err)
		}
	case <-time.After(150 * time.Millisecond):
		// listener came up fine (or is still binding); don't block Start()
		// forever on a server that's supposed to run indefinitely.
	}

	return nil
}

// Stop gracefully shuts down the listener, if running, recording reason for
// later display (e.g. "manual", "active database changed", "process exit").
func (m *Manager) Stop(reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked(reason)
}

func (m *Manager) stopLocked(reason string) error {
	if !m.running {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := m.httpServer.Shutdown(ctx)
	m.running = false
	m.snap = nil
	m.httpServer = nil
	m.lastStopReason = reason
	return err
}

// NotifyActiveDBChanged stops the listener (reason "active database
// changed") if it's running and was snapshotted against a different DB label
// than newLabel. Returns whether it actually stopped anything.
func (m *Manager) NotifyActiveDBChanged(newLabel string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running || m.snap == nil || m.snap.dbLabel == newLabel {
		return false
	}
	_ = m.stopLocked("active database changed")
	return true
}

// Status reports the listener's current run state.
func (m *Manager) Status() StatusPayload {
	m.mu.Lock()
	defer m.mu.Unlock()

	payload := StatusPayload{
		Enabled:        m.enabled,
		Write:          m.write,
		Running:        m.running,
		BindAddr:       m.bindAddr,
		Port:           m.port,
		LastStopReason: m.lastStopReason,
	}
	if m.running && m.snap != nil {
		payload.DBLabel = m.snap.dbLabel
		payload.StartedAt = m.startedAt.Format(time.RFC3339)
	}
	return payload
}

// RunningSnapshotTables returns the table->RouteInfo map the listener was
// built from, and the scope it was built against, if currently running.
func (m *Manager) RunningSnapshotTables() (running bool, scope string, tables map[string]RouteInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running || m.snap == nil {
		return false, "", nil
	}
	return true, m.snap.scope, m.snap.tables
}
