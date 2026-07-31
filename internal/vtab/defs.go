package vtab

import (
	"github.com/0funct0ry/squad/internal/seed"
	"github.com/0funct0ry/squad/internal/vtab/modules"
	vtabdriver "modernc.org/sqlite/vtab"
)

func opt(key, label string, kind seed.OptionKind, required bool, def any, desc string) seed.OptionField {
	return seed.OptionField{Key: key, Label: label, Kind: kind, Required: required, Default: def, Description: desc}
}

func csvModuleDef() ModuleDef {
	return ModuleDef{
		Name: "csv", Group: "files", Description: "Delimited text file",
		RequiresFile: true,
		Args: []seed.OptionField{
			opt("file", "File", seed.OptKindString, true, nil, "Path, resolved inside --modules-root"),
			opt("header", "Header row", seed.OptKindBool, false, true, "Treat the first row as column names"),
			opt("delimiter", "Delimiter", seed.OptKindString, false, ",", "Field delimiter character"),
			opt("quote", "Quote", seed.OptKindString, false, `"`, "Quote character"),
		},
		factory: func() vtabdriver.Module { return &modules.CSVModule{} },
	}
}

func jsonlModuleDef() ModuleDef {
	return ModuleDef{
		Name: "jsonl", Group: "files", Description: "NDJSON or JSON array",
		RequiresFile: true,
		Args: []seed.OptionField{
			opt("file", "File", seed.OptKindString, true, nil, "Path, resolved inside --modules-root"),
			opt("root", "Root", seed.OptKindString, false, "", "JSON Pointer to the array of records"),
			opt("columns", "Columns", seed.OptKindString, false, "", "Optional explicit comma-separated key list"),
		},
		factory: func() vtabdriver.Module { return &modules.JSONLModule{} },
	}
}

func parquetModuleDef() ModuleDef {
	return ModuleDef{
		Name: "parquet", Group: "files", Description: "Columnar file",
		RequiresFile: true,
		Args: []seed.OptionField{
			opt("file", "File", seed.OptKindString, true, nil, "Path, resolved inside --modules-root"),
			opt("columns", "Columns", seed.OptKindString, false, "", "Optional projection, pushed down"),
		},
		factory: func() vtabdriver.Module { return &modules.ParquetModule{} },
	}
}

func xlsxModuleDef() ModuleDef {
	return ModuleDef{
		Name: "xlsx", Group: "files", Description: "Spreadsheet worksheet",
		RequiresFile: true,
		Args: []seed.OptionField{
			opt("file", "File", seed.OptKindString, true, nil, "Path, resolved inside --modules-root"),
			opt("sheet", "Sheet", seed.OptKindString, false, "", "Sheet name (default: first sheet)"),
			opt("range", "Range", seed.OptKindString, false, "", "Optional A1-style bound, e.g. A1:C40"),
			opt("header", "Header row", seed.OptKindBool, false, true, "Treat the first row as column names"),
		},
		factory: func() vtabdriver.Module { return &modules.XLSXModule{} },
	}
}

func yamlModuleDef() ModuleDef {
	return ModuleDef{
		Name: "yaml", Group: "files", Description: "Config / manifest file",
		RequiresFile: true,
		Args: []seed.OptionField{
			opt("file", "File", seed.OptKindString, true, nil, "Path, resolved inside --modules-root"),
			opt("root", "Root", seed.OptKindString, false, "", "Path into the document"),
			opt("multidoc", "Multi-document", seed.OptKindBool, false, false, "Treat ----separated docs as rows"),
		},
		factory: func() vtabdriver.Module { return &modules.YAMLModule{} },
	}
}

func xmlModuleDef() ModuleDef {
	return ModuleDef{
		Name: "xml", Group: "files", Description: "Element path to rows",
		RequiresFile: true,
		Args: []seed.OptionField{
			opt("file", "File", seed.OptKindString, true, nil, "Path, resolved inside --modules-root"),
			opt("path", "Path", seed.OptKindString, true, nil, "Element path, e.g. /catalog/product"),
			opt("attributes", "Attributes", seed.OptKindBool, false, true, "Expose attributes as columns"),
		},
		factory: func() vtabdriver.Module { return &modules.XMLModule{} },
	}
}

func seriesModuleDef() ModuleDef {
	return ModuleDef{
		Name: "series", Group: "generators", Description: "Numeric sequence",
		Args: []seed.OptionField{
			opt("start", "Start", seed.OptKindFloat, false, 0, "Starting value"),
			opt("stop", "Stop", seed.OptKindFloat, true, nil, "Stop value (exclusive)"),
			opt("step", "Step", seed.OptKindFloat, false, 1, "Step (may be fractional)"),
		},
		factory: func() vtabdriver.Module { return &modules.SeriesModule{} },
	}
}

func calendarModuleDef() ModuleDef {
	return ModuleDef{
		Name: "calendar", Group: "generators", Description: "Date spine with calendar attributes",
		Args: []seed.OptionField{
			opt("start", "Start", seed.OptKindDate, true, nil, "YYYY-MM-DD"),
			opt("stop", "Stop", seed.OptKindDate, true, nil, "YYYY-MM-DD"),
			opt("step", "Step", seed.OptKindString, false, "1 day", "e.g. '1 day'"),
		},
		factory: func() vtabdriver.Module { return &modules.CalendarModule{} },
	}
}

func fakeModuleDef() ModuleDef {
	return ModuleDef{
		Name: "fake", Group: "generators", Description: "Rows from squad's seed generator registry",
		Args: []seed.OptionField{
			opt("rows", "Rows", seed.OptKindInt, true, nil, "Number of rows to generate"),
			opt("seed", "Seed", seed.OptKindInt, false, nil, "Optional seed for reproducible output"),
		},
		factory: func() vtabdriver.Module { return &modules.FakeModule{} },
	}
}

func tokensModuleDef() ModuleDef {
	return ModuleDef{
		Name: "tokens", Group: "generators", Description: "Split text into rows",
		Args: []seed.OptionField{
			opt("text", "Text", seed.OptKindString, true, nil, "Text to split"),
			opt("delimiter", "Delimiter", seed.OptKindString, false, ",", "Mutually exclusive with regex"),
			opt("regex", "Regex", seed.OptKindString, false, "", "Mutually exclusive with delimiter"),
			opt("trim", "Trim", seed.OptKindBool, false, true, "Trim whitespace from each token"),
		},
		factory: func() vtabdriver.Module { return &modules.TokensModule{} },
	}
}
