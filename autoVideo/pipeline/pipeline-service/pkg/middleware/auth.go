package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/autovideo/pipeline-service/pkg/response"
	"github.com/gin-gonic/gin"
)

// Auth JWT 鉴权中间件（HS256，手动解析，无外部 JWT 库依赖）
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c)
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			response.Unauthorized(c)
			c.Abort()
			return
		}

		tokenStr := parts[1]
		claims, err := parseJWT(tokenStr, secret)
		if err != nil {
			response.Unauthorized(c)
			c.Abort()
			return
		}

		if userID, exists := claims["user_id"]; exists {
			c.Set("user_id", userID)
		}
		if projectID, exists := claims["project_id"]; exists {
			c.Set("project_id", projectID)
		}

		c.Next()
	}
}

// parseJWT 手动解析并验证 HS256 JWT
func parseJWT(tokenStr, secret string) (map[string]interface{}, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errInvalidToken
	}

	// 验证签名
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expectedSig), []byte(parts[2])) {
		return nil, errInvalidToken
	}

	// 解码 payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errInvalidToken
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errInvalidToken
	}

	return claims, nil
}

var errInvalidToken = &authError{"invalid token"}

type authError struct{ msg string }

// Error —— 返回认证错误的描述信息
func (e *authError) Error() string { return e.msg }
