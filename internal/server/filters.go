package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/gin-gonic/gin"
)

// parseRowFilters builds the []db.Filter for a request from two possible
// query-param shapes:
//   - `filters=<json array>` — the structured filter-builder payload
//     (Phase 2), e.g. [{"column":"age","operator":"gt","value":21}].
//   - `filter[col]=val` — the legacy simple substring filter (pre-Phase-2),
//     kept for backward compatibility with any existing bookmarked URLs;
//     translated to a "contains" filter.
//
// Both forms may be present at once (AND-combined). A malformed `filters`
// payload returns an error the caller should surface as a VALIDATION error.
func parseRowFilters(c *gin.Context) ([]db.Filter, error) {
	var filters []db.Filter

	if raw := c.Query("filters"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &filters); err != nil {
			return nil, fmt.Errorf("invalid filters payload: %w", err)
		}
	}

	queries := c.Request.URL.Query()
	for k, v := range queries {
		if strings.HasPrefix(k, "filter[") && strings.HasSuffix(k, "]") && len(v) > 0 && v[0] != "" {
			col := k[7 : len(k)-1]
			filters = append(filters, db.Filter{Column: col, Operator: "contains", Value: v[0]})
		}
	}

	return filters, nil
}
