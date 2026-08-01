package udf

import (
	"database/sql/driver"
	"fmt"
	"strconv"
)

// argString coerces a driver.Value arg to a string the way SQLite's own
// functions do (numbers render in their canonical text form, NULL -> "").
func argString(v driver.Value) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case bool:
		if t {
			return "1"
		}
		return "0"
	default:
		return fmt.Sprintf("%v", t)
	}
}

func argBytes(v driver.Value) []byte {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return t
	case string:
		return []byte(t)
	default:
		return []byte(argString(v))
	}
}

func argFloat(v driver.Value) (float64, error) {
	switch t := v.(type) {
	case nil:
		return 0, nil
	case float64:
		return t, nil
	case int64:
		return float64(t), nil
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, fmt.Errorf("expected a number, got %q", t)
		}
		return f, nil
	case []byte:
		return argFloat(string(t))
	default:
		return 0, fmt.Errorf("expected a number, got %T", v)
	}
}

func argInt(v driver.Value) (int64, error) {
	switch t := v.(type) {
	case nil:
		return 0, nil
	case int64:
		return t, nil
	case float64:
		return int64(t), nil
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("expected an integer, got %q", t)
		}
		return n, nil
	case []byte:
		return argInt(string(t))
	default:
		return 0, fmt.Errorf("expected an integer, got %T", v)
	}
}

func argBool(v driver.Value) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case int64:
		return t != 0
	case float64:
		return t != 0
	case string:
		return t != "" && t != "0"
	default:
		return false
	}
}

func boolResult(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
