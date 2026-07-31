package vtab

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Mount is one active `CREATE VIRTUAL TABLE temp."<alias>" USING <module>(...)`
// instance, tracked as desired state so it can be replayed onto whichever
// pooled connection a request lands on (see WithMounts).
type Mount struct {
	Alias           string            `json:"alias"`
	Module          string            `json:"module"`
	Args            map[string]string `json:"args"`
	DeclaredColumns []string          `json:"declaredColumns"`
	CreatedAt       time.Time         `json:"createdAt"`
}

// MountStore holds every active mount, in-memory and never persisted —
// mounts are process-lifetime only, modelled on internal/restserver's
// ConfigStore. Registration is process-global (not per-scope), so a single
// flat map (no dbScope dimension) is enough: the same mount alias is
// replayed into whichever database's temp schema a request needs it in.
type MountStore struct {
	mu     sync.Mutex
	mounts map[string]Mount
	// everMounted is the union of every alias this store has ever held,
	// including ones since removed. *sql.Conn.Close() (used throughout
	// this package and by callers via WithMounts) returns the connection to
	// database/sql's pool rather than closing it, so a temp virtual table
	// created on that connection outlives the mount being removed from
	// this store — a later request can land back on that same pooled
	// connection and still see the "unmounted" table. WithMounts uses this
	// set to issue a DROP TABLE IF EXISTS for every alias ever seen, not
	// just the ones currently active, so an unmount is enforced on whatever
	// connection a request happens to draw, regardless of pool reuse.
	everMounted map[string]bool
}

// NewMountStore creates an empty MountStore.
func NewMountStore() *MountStore {
	return &MountStore{mounts: make(map[string]Mount), everMounted: make(map[string]bool)}
}

// ValidateAlias checks alias validity against reserved names and existing
// mounts, but not against the target database's own tables/views — callers
// with DB access should also check sqlite_master before calling Add.
func (s *MountStore) ValidateAlias(alias string) error {
	if alias == "" {
		return fmt.Errorf("alias must not be empty")
	}
	if Exists(alias) {
		return fmt.Errorf("alias %q collides with a module name", alias)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.mounts[alias]; ok {
		return fmt.Errorf("alias %q is already mounted", alias)
	}
	return nil
}

// Add records a new mount. Callers must call ValidateAlias (and check for
// an existing table/view) first.
func (s *MountStore) Add(m Mount) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mounts[m.Alias] = m
	s.everMounted[m.Alias] = true
}

// Remove drops a mount by alias. Reports whether it existed.
func (s *MountStore) Remove(alias string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.mounts[alias]; !ok {
		return false
	}
	delete(s.mounts, alias)
	return true
}

// Get returns one mount by alias.
func (s *MountStore) Get(alias string) (Mount, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.mounts[alias]
	return m, ok
}

// List returns every active mount, sorted by alias.
func (s *MountStore) List() []Mount {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Mount, 0, len(s.mounts))
	for _, m := range s.mounts {
		out = append(out, m)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Alias < out[j-1].Alias; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Len reports the number of active mounts.
func (s *MountStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.mounts)
}

// staleAliases returns every alias this store has ever mounted that is not
// currently active — the tombstones WithMounts must drop on whatever
// connection it's handed, since that connection may still carry a virtual
// table for an alias this store no longer considers mounted.
func (s *MountStore) staleAliases() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for alias := range s.everMounted {
		if _, active := s.mounts[alias]; !active {
			out = append(out, alias)
		}
	}
	return out
}

// BuildMountSQL renders the CREATE VIRTUAL TABLE statement for a mount.
func BuildMountSQL(alias, module string, args map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `CREATE VIRTUAL TABLE temp.%s USING %s(`, quoteIdentifier(alias), module)
	first := true
	for _, k := range sortedKeys(args) {
		if !first {
			b.WriteString(", ")
		}
		first = false
		fmt.Fprintf(&b, "%s=%s", k, quoteSQLString(args[k]))
	}
	b.WriteString(")")
	return b.String()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteSQLString(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

// WithMounts acquires a pinned *sql.Conn, replays every active mount onto it
// as CREATE VIRTUAL TABLE temp."<alias>" USING ..., then runs fn on that
// same connection. With an empty store it skips straight to fn(conn) — the
// no-mounts path is behaviorally identical to querying db directly.
func WithMounts(ctx context.Context, database *sql.DB, store *MountStore, fn func(*sql.Conn) error) error {
	conn, err := database.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if store != nil {
		// Drop any alias this store once mounted but no longer does. The
		// connection just acquired may be a pooled one that created that
		// alias's virtual table on a previous request; since it was never
		// dropped when the mount was removed (see MountStore.everMounted),
		// it would otherwise still be queryable here after "unmount".
		for _, alias := range store.staleAliases() {
			if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp.`+quoteIdentifier(alias)); err != nil {
				return fmt.Errorf("failed to drop stale mount %q: %w", alias, err)
			}
		}

		for _, m := range store.List() {
			// *sql.Conn.Close() returns the connection to database/sql's
			// pool rather than terminating it, so a temp table created here
			// can still be present the next time this same pooled
			// connection is handed out. DROP IF EXISTS first makes the
			// replay idempotent regardless of whether this connection has
			// seen this mount before.
			if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp.`+quoteIdentifier(m.Alias)); err != nil {
				return fmt.Errorf("failed to replay mount %q: %w", m.Alias, err)
			}
			stmt := BuildMountSQL(m.Alias, m.Module, m.Args)
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("failed to replay mount %q: %w", m.Alias, err)
			}
		}
	}

	return fn(conn)
}
