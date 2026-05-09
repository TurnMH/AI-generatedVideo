package middleware

import (
	"strconv"
	"strings"

	"github.com/autovideo/script-service/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Auth —— JWT Bearer Token 认证中间件，验证 HMAC 签名并提取 user_id 和 role
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, 4001, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			response.Error(c, 4001, "invalid authorization header format")
			c.Abort()
			return
		}

		tokenStr := parts[1]
		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			response.Error(c, 4001, "invalid or expired token")
			c.Abort()
			return
		}
		userID, ok := parseClaimInt64(claims["user_id"])
		if !ok || userID <= 0 {
			response.Error(c, 4001, "invalid or expired token")
			c.Abort()
			return
		}
		role, _ := claims["role"].(string)

		c.Set("user_id", userID)
		c.Set("role", role)
		c.Next()
	}
}

func parseClaimInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case int:
		return int64(v), true
	case uint64:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case string:
		if v == "" {
			return 0, false
		}
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return id, true
	default:
		return 0, false
	}
}
