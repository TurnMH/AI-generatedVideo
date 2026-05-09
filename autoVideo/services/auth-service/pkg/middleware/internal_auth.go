package middleware

import (
	"strings"

	jwtpkg "github.com/autovideo/auth-service/pkg/jwt"
	"github.com/autovideo/auth-service/pkg/response"
	"github.com/gin-gonic/gin"
)

// InternalAuth 仅允许 role=service 的服务间 JWT 访问内部接口。
func InternalAuth(accessSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "invalid token")
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Unauthorized(c, "invalid token")
			return
		}
		claims, err := jwtpkg.ParseToken(parts[1], accessSecret)
		if err != nil {
			response.Unauthorized(c, "invalid token")
			return
		}
		if claims.Role != "service" || claims.TokenType != jwtpkg.TokenTypeAccess {
			response.Forbidden(c, "service role required")
			return
		}
		c.Next()
	}
}