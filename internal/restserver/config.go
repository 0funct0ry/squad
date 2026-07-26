// Package restserver implements the separate-listener, opt-in auto-REST
// subsystem described in SPEC.md §5.7: per-table CRUD JSON endpoints on their
// own port, gated by --rest/--write and independent per-table toggles, never
// protected by --token. Config here is in-memory only and reset every
// process restart.
package restserver

import "sync"

// TableConfig holds the user-controlled REST toggles for a single table.
// Create/Update/Delete are only ever meaningful when the process was also
// started with --write; that is enforced by callers (Manager.Start and the
// control-route handlers), not by ConfigStore itself.
type TableConfig struct {
	Exposed bool `json:"exposed"`
	Create  bool `json:"create"`
	Update  bool `json:"update"`
	Delete  bool `json:"delete"`
}

// ConfigStore holds in-memory, never-persisted per-table REST config scoped
// by an opaque dbScope key ("" in single-DB mode, the sandbox entry ID
// otherwise) so sandbox mode doesn't leak one database's toggles onto
// another.
type ConfigStore struct {
	mu     sync.RWMutex
	tables map[string]map[string]TableConfig // dbScope -> tableName -> config
}

// NewConfigStore creates an empty ConfigStore.
func NewConfigStore() *ConfigStore {
	return &ConfigStore{tables: make(map[string]map[string]TableConfig)}
}

// Get returns the config for (dbScope, table), or the zero value if unset.
func (c *ConfigStore) Get(dbScope, table string) TableConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if m, ok := c.tables[dbScope]; ok {
		return m[table]
	}
	return TableConfig{}
}

// Set stores the config for (dbScope, table).
func (c *ConfigStore) Set(dbScope, table string, cfg TableConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.tables[dbScope]
	if !ok {
		m = make(map[string]TableConfig)
		c.tables[dbScope] = m
	}
	m[table] = cfg
}

// Snapshot returns a copy of every table's config currently set for dbScope,
// keyed by table name. Tables with no explicit config are not included.
func (c *ConfigStore) Snapshot(dbScope string) map[string]TableConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]TableConfig, len(c.tables[dbScope]))
	for k, v := range c.tables[dbScope] {
		out[k] = v
	}
	return out
}

// Forget discards all per-table config for dbScope, e.g. when a sandbox
// database is deleted.
func (c *ConfigStore) Forget(dbScope string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.tables, dbScope)
}
