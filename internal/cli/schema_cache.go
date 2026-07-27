package cli

import "github.com/0funct0ry/squad/internal/db"

// cachedTables returns table/view names, fetched once per session via
// internal/db.GetTables and invalidated after any executed DDL statement.
func (s *State) cachedTables() []string {
	if s.cacheValid && s.tablesCache != nil {
		return s.tablesCache
	}
	tables, err := db.GetTables(s.DB)
	if err != nil {
		return nil
	}
	names := make([]string, len(tables))
	for i, t := range tables {
		names[i] = t.Name
	}
	s.tablesCache = names
	s.cacheValid = true
	return names
}

// cachedColumns returns the column names for table, fetched once per session
// via internal/db.GetTableSchema (the same PRAGMA table_xinfo reader used by
// the /api/tables/:name handler) and invalidated after any executed DDL.
func (s *State) cachedColumns(table string) []string {
	if s.columnsCache == nil {
		s.columnsCache = map[string][]string{}
	}
	if cols, ok := s.columnsCache[table]; ok && s.cacheValid {
		return cols
	}
	schema, err := db.GetTableSchema(s.DB, table)
	if err != nil {
		return nil
	}
	names := make([]string, len(schema.Columns))
	for i, c := range schema.Columns {
		names[i] = c.Name
	}
	s.columnsCache[table] = names
	s.cacheValid = true
	return names
}
