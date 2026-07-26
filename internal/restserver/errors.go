package restserver

import "github.com/gin-gonic/gin"

// writeError emits SPEC §5.7's dedicated /rest/* error shape
// ({"error": code, "message": text}) — deliberately not the {ok,data}
// envelope used by /api/*.
func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": code, "message": message})
}
