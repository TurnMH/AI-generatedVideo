package middleware

import (
	"strings"

	"github.com/autovideo/video-service/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Auth —— 返回 JWT Bearer Token 验证中间件，将 user_id 写入上下文
// Auth validates a Bearer JWT token.
// For simplicity the secret is passed in; claims are set on the context.
func Auth(secret string, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Unauthorized(c)
			c.Abort()
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		userID, err := parseToken(token, secret)
		if err != nil {
			logger.Warn("invalid token", zap.Error(err))
			response.Unauthorized(c)
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}

// parseToken —— 验证 HS256 JWT 签名并提取 user_id，返回用户 ID
// parseToken does a minimal HS256 JWT verification and returns the user_id claim.
func parseToken(tokenStr, secret string) (int64, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return 0, errInvalidToken
	}

	// Verify signature
	sig, err := base64URLDecode(parts[2])
	if err != nil {
		return 0, errInvalidToken
	}
	expected := hmacSHA256([]byte(parts[0]+"."+parts[1]), []byte(secret))
	if !hmacEqual(sig, expected) {
		return 0, errInvalidToken
	}

	// Decode payload
	payload, err := base64URLDecode(parts[1])
	if err != nil {
		return 0, errInvalidToken
	}
	return extractUserID(payload)
}
