package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/autovideo/image-service/pkg/response"
	"github.com/gin-gonic/gin"
)

type jwtClaims struct {
	UserID    int64 `json:"user_id"`
	ProjectID int64 `json:"project_id"`
}

// UnmarshalJSON —— 自定义 JSON 反序列化，支持将字符串或数字类型的 user_id/project_id 解析为 int64
func (c *jwtClaims) UnmarshalJSON(data []byte) error {
	var raw struct {
		UserID    json.RawMessage `json:"user_id"`
		ProjectID json.RawMessage `json:"project_id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.UserID = parseIntOrString(raw.UserID)
	c.ProjectID = parseIntOrString(raw.ProjectID)
	return nil
}

// parseIntOrString —— 将 JSON 原始值解析为 int64，支持数字和字符串两种格式
func parseIntOrString(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var n int64
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v
		}
	}
	return 0
}

// Auth —— JWT Bearer Token 认证中间件，验证 HS256 签名并提取 user_id 和 project_id
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Error(c, http.StatusUnauthorized, "invalid authorization format")
			c.Abort()
			return
		}

		claims, err := parseJWT(parts[1], secret)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "invalid token: "+err.Error())
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("project_id", claims.ProjectID)
		c.Next()
	}
}

// parseJWT —— 解析并验证 JWT token，返回解码后的 claims 或错误
func parseJWT(token, secret string) (*jwtClaims, error) {
	segments := strings.Split(token, ".")
	if len(segments) != 3 {
		return nil, errors.New("malformed token")
	}

	// Verify HMAC-SHA256 signature.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(segments[0] + "." + segments[1]))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expectedSig), []byte(segments[2])) {
		return nil, errors.New("signature mismatch")
	}

	// Decode payload.
	payload, err := base64.RawURLEncoding.DecodeString(segments[1])
	if err != nil {
		return nil, errors.New("invalid payload encoding")
	}

	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errors.New("invalid payload json")
	}
	return &claims, nil
}
