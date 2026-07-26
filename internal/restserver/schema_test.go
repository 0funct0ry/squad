package restserver

import (
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func TestResolveKeyColumn(t *testing.T) {
	cases := []struct {
		name    string
		schema  *db.TableSchema
		wantCol string
		wantOK  bool
	}{
		{
			name:    "single column PK",
			schema:  &db.TableSchema{Type: "table", PrimaryKey: []string{"id"}},
			wantCol: "id",
			wantOK:  true,
		},
		{
			name:    "no PK on ordinary rowid table",
			schema:  &db.TableSchema{Type: "table", PrimaryKey: nil, WithoutRowid: false},
			wantCol: "rowid",
			wantOK:  true,
		},
		{
			name:    "composite PK on rowid table falls back to rowid",
			schema:  &db.TableSchema{Type: "table", PrimaryKey: []string{"a", "b"}, WithoutRowid: false},
			wantCol: "rowid",
			wantOK:  true,
		},
		{
			name:    "WITHOUT ROWID composite key has no usable identity",
			schema:  &db.TableSchema{Type: "table", PrimaryKey: []string{"a", "b"}, WithoutRowid: true},
			wantCol: "",
			wantOK:  false,
		},
		{
			name:    "WITHOUT ROWID single column PK is still usable",
			schema:  &db.TableSchema{Type: "table", PrimaryKey: []string{"id"}, WithoutRowid: true},
			wantCol: "id",
			wantOK:  true,
		},
		{
			name:    "view never gets a key route regardless of PK shape",
			schema:  &db.TableSchema{Type: "view", PrimaryKey: []string{"id"}},
			wantCol: "",
			wantOK:  false,
		},
		{
			name:    "view with no PK also has no key route",
			schema:  &db.TableSchema{Type: "view"},
			wantCol: "",
			wantOK:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			col, ok := resolveKeyColumn(tc.schema)
			if ok != tc.wantOK || col != tc.wantCol {
				t.Errorf("resolveKeyColumn() = (%q, %v), want (%q, %v)", col, ok, tc.wantCol, tc.wantOK)
			}
		})
	}
}

func TestResolveRouteInfo_WriteGating(t *testing.T) {
	tbl := db.TableInfo{Name: "users", Type: "table"}
	schema := &db.TableSchema{Name: "users", Type: "table", PrimaryKey: []string{"id"}}
	cfg := TableConfig{Exposed: true, Create: true, Update: true, Delete: true}

	info := ResolveRouteInfo(tbl, schema, cfg, false)
	if info.Create || info.Update || info.Delete {
		t.Errorf("expected all write methods disabled when write=false, got %+v", info)
	}

	info = ResolveRouteInfo(tbl, schema, cfg, true)
	if !info.Create || !info.Update || !info.Delete {
		t.Errorf("expected all write methods enabled when write=true and toggles on, got %+v", info)
	}
}

func TestResolveRouteInfo_ViewNeverGetsWriteRoutes(t *testing.T) {
	tbl := db.TableInfo{Name: "user_view", Type: "view"}
	schema := &db.TableSchema{Name: "user_view", Type: "view", PrimaryKey: []string{"id"}}
	cfg := TableConfig{Exposed: true, Create: true, Update: true, Delete: true}

	info := ResolveRouteInfo(tbl, schema, cfg, true)
	if info.Create || info.Update || info.Delete {
		t.Errorf("expected views to never get write routes, got %+v", info)
	}
	if info.HasPKRoute {
		t.Errorf("expected views to never get a :pk route, got HasPKRoute=true")
	}
}
