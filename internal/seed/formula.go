package seed

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

// topoSortColumns returns a topologically-sorted column order for the given
// columns such that every formula column appears after all of its declared
// dependencies (formulaDeps[colName]). It uses Kahn's algorithm and returns
// an error on an unknown-column reference, a self-reference, or a cycle.
func topoSortColumns(columns map[string]ColumnSpec, formulaDeps map[string][]string) ([]string, error) {
	inDegree := make(map[string]int, len(columns))
	adj := make(map[string][]string) // dep -> list of columns that depend on it

	for name := range columns {
		inDegree[name] = 0
	}

	for col, deps := range formulaDeps {
		if _, ok := columns[col]; !ok {
			return nil, fmt.Errorf("formula column %q not found among selected columns", col)
		}
		for _, dep := range deps {
			if dep == col {
				return nil, fmt.Errorf("circular dependency detected among formula columns: %q references itself", col)
			}
			if _, ok := columns[dep]; !ok {
				return nil, fmt.Errorf("formula column %q references unknown column %q", col, dep)
			}
			adj[dep] = append(adj[dep], col)
			inDegree[col]++
		}
	}

	// Deterministic starting order: sorted column names with in-degree 0.
	names := make([]string, 0, len(columns))
	for name := range columns {
		names = append(names, name)
	}
	sortStrings(names)

	var queue []string
	for _, name := range names {
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}

	var order []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)

		next := adj[cur]
		sortStrings(next)
		for _, dependent := range next {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(order) != len(columns) {
		return nil, fmt.Errorf("circular dependency detected among formula columns")
	}

	return order, nil
}

func sortStrings(s []string) {
	// small local insertion sort to avoid importing "sort" repeatedly across
	// this file's hot loops; correctness over micro-perf, list sizes are tiny.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// buildFormulaDeps builds the column -> dependency-list map for every column
// whose generator is "formula", reading options["columns"] (expected []any
// of strings, per the wire JSON shape).
func buildFormulaDeps(columns map[string]ColumnSpec) map[string][]string {
	deps := make(map[string][]string)
	for name, spec := range columns {
		if spec.Generator != "formula" {
			continue
		}
		deps[name] = optStringSlice(spec.Options, "columns")
	}
	return deps
}

// ValidateFormulaDependencies checks that every formula column's declared
// dependencies exist among columns and that no cycle or self-reference
// exists, without actually generating any values.
func ValidateFormulaDependencies(columns map[string]ColumnSpec) error {
	deps := buildFormulaDeps(columns)
	if len(deps) == 0 {
		return nil
	}
	_, err := topoSortColumns(columns, deps)
	return err
}

// evalFormula evaluates spec's "expression" option against rowSoFar (values
// already generated for columns earlier in topo order) and returns the
// result. rowSoFar must contain every column referenced by the expression --
// guaranteed by topo ordering upstream.
func (g *RowGenerator) evalFormula(colName string, spec ColumnSpec, rowSoFar map[string]any) (any, error) {
	exprStr := optString(spec.Options, "expression", "")
	if exprStr == "" {
		return nil, fmt.Errorf("formula column %q: missing options.expression", colName)
	}
	expr, err := parser.ParseExpr(exprStr)
	if err != nil {
		return nil, fmt.Errorf("formula column %q: %w", colName, err)
	}
	return evalNode(expr, rowSoFar)
}

// evalNode evaluates a restricted whitelist of AST node kinds: *ast.Ident,
// *ast.BasicLit, *ast.ParenExpr, and *ast.BinaryExpr limited to + - * /. This
// whitelist IS the safety mechanism -- any other node kind (calls, selectors,
// indexing, unary ops, etc.) is rejected with an error rather than silently
// ignored, since falling through would mean arbitrary-looking expressions
// silently produce wrong or undefined results instead of failing loudly.
func evalNode(n ast.Expr, row map[string]any) (any, error) {
	switch node := n.(type) {
	case *ast.Ident:
		v, ok := row[node.Name]
		if !ok {
			return nil, fmt.Errorf("formula: undefined column reference %q", node.Name)
		}
		return v, nil

	case *ast.BasicLit:
		switch node.Kind {
		case token.INT:
			var i int64
			if _, err := fmt.Sscanf(node.Value, "%d", &i); err != nil {
				return nil, fmt.Errorf("formula: invalid integer literal %q", node.Value)
			}
			return i, nil
		case token.FLOAT:
			var f float64
			if _, err := fmt.Sscanf(node.Value, "%g", &f); err != nil {
				return nil, fmt.Errorf("formula: invalid float literal %q", node.Value)
			}
			return f, nil
		case token.STRING:
			s, err := strconv.Unquote(node.Value)
			if err != nil {
				return nil, fmt.Errorf("formula: invalid string literal %q: %w", node.Value, err)
			}
			return s, nil
		default:
			return nil, fmt.Errorf("formula: unsupported literal kind %v", node.Kind)
		}

	case *ast.ParenExpr:
		return evalNode(node.X, row)

	case *ast.BinaryExpr:
		switch node.Op {
		case token.ADD, token.SUB, token.MUL, token.QUO:
		default:
			return nil, fmt.Errorf("formula: unsupported operator %q", node.Op.String())
		}
		left, err := evalNode(node.X, row)
		if err != nil {
			return nil, err
		}
		right, err := evalNode(node.Y, row)
		if err != nil {
			return nil, err
		}
		return applyBinaryOp(node.Op, left, right)

	default:
		return nil, fmt.Errorf("formula: unsupported expression syntax (%T)", n)
	}
}

// applyBinaryOp implements +, -, *, / over the whitelisted operand types.
// '+' does string concatenation when both operands are strings, otherwise
// numeric addition. '-', '*' are numeric only. '/' is numeric and always
// yields a float64.
func applyBinaryOp(op token.Token, left, right any) (any, error) {
	if op == token.ADD {
		if ls, lok := left.(string); lok {
			if rs, rok := right.(string); rok {
				return ls + rs, nil
			}
		}
	}

	lf, lok := toFloat(left)
	rf, rok := toFloat(right)
	if !lok || !rok {
		return nil, fmt.Errorf("formula: operator %q requires numeric operands, got %T and %T", op.String(), left, right)
	}

	switch op {
	case token.ADD:
		return lf + rf, nil
	case token.SUB:
		return lf - rf, nil
	case token.MUL:
		return lf * rf, nil
	case token.QUO:
		return lf / rf, nil
	default:
		return nil, fmt.Errorf("formula: unsupported operator %q", op.String())
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	default:
		return 0, false
	}
}
