package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a middleware that allows requests from the given origins.
// If allowedOrigins is empty, it falls back to localhost:3000 (dev default).
func CORS(allowedOrigins ...string) gin.HandlerFunc {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		origins[o] = struct{}{}
	}
	// Dev fallback
	if len(origins) == 0 {
		origins["http://localhost:3000"] = struct{}{}
		origins["http://127.0.0.1:3000"] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := origins[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", strings.Join([]string{
			"Authorization",
			"Content-Type",
			"X-User-ID",
			"X-Requested-With",
		}, ", "))
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
