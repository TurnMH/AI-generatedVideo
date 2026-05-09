package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
)

var errInvalidToken = errors.New("invalid token")

// hmacSHA256 —— 使用 SHA256 算法计算 HMAC 签名
func hmacSHA256(data, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// hmacEqual —— 安全比较两个 HMAC 值是否相等
func hmacEqual(a, b []byte) bool {
	return hmac.Equal(a, b)
}

// base64URLDecode —— 解码 Base64 URL 编码字符串，自动补齐 padding
func base64URLDecode(s string) ([]byte, error) {
	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

// extractUserID —— 从 JWT payload 中提取 user_id 或 sub 字段
func extractUserID(payload []byte) (int64, error) {
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, errInvalidToken
	}
	if uid := parseClaimInt64(claims["user_id"]); uid != 0 {
		return uid, nil
	}
	if uid := parseClaimInt64(claims["sub"]); uid != 0 {
		return uid, nil
	}
	return 0, errInvalidToken
}

// parseClaimInt64 —— 将 JWT claim 值转换为 int64，支持 float64/int64/string 类型
func parseClaimInt64(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case string:
		n, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			return n
		}
	}
	return 0
}
