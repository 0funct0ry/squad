package restserver

import (
	"strings"

	"github.com/0funct0ry/squad/internal/db"
)

// RouteInfo describes how a table/view can be routed on /rest/*, resolved
// once at Manager.Start() time from the table's schema and the effective
// per-table config.
type RouteInfo struct {
	Table      string
	Type       string // "table" or "view"
	HasPKRoute bool
	PKColumn   string // resolved column name, or "rowid"; "" if HasPKRoute is false
	Create     bool
	Update     bool
	Delete     bool
}

// ResolveRouteInfo computes a table's effective RouteInfo from its schema and
// currently configured toggles, applying the write-gating rule (create/
// update/delete only ever true when write is true) and view restrictions
// (views only ever get GET routes, per SPEC §5.7). Shared by Manager.Start
// (to build the running snapshot) and the /api/rest/tables control route (to
// report live vs. snapshot state).
func ResolveRouteInfo(t db.TableInfo, schema *db.TableSchema, cfg TableConfig, write bool) RouteInfo {
	keyCol, hasPK := resolveKeyColumn(schema)
	info := RouteInfo{
		Table:      t.Name,
		Type:       t.Type,
		HasPKRoute: hasPK,
		PKColumn:   keyCol,
	}
	isView := strings.EqualFold(t.Type, "view")
	if !isView {
		info.Create = write && cfg.Create
		info.Update = write && cfg.Update && hasPK
		info.Delete = write && cfg.Delete && hasPK
	}
	return info
}

// resolveKeyColumn implements SPEC §5.7's primary-key resolution rule:
// single-column PK -> that column; composite PK or no PK on an ordinary
// rowid table -> fall back to "rowid"; WITHOUT ROWID tables with a composite
// key (or otherwise no single-column identity) -> no usable key, list-only.
// Views never have a rowid, so they always get list-only routes regardless
// of what PrimaryKey/WithoutRowid happen to report.
func resolveKeyColumn(schema *db.TableSchema) (col string, ok bool) {
	if strings.EqualFold(schema.Type, "view") {
		return "", false
	}
	if len(schema.PrimaryKey) == 1 {
		return schema.PrimaryKey[0], true
	}
	if !schema.WithoutRowid {
		// Ordinary rowid table with a composite PK or no explicit PK at all
		// -> rowid is always a usable, unique fallback identity.
		return "rowid", true
	}
	return "", false
}
