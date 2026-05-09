// 本文件封装了 JWT（JSON Web Token）的生成与解析功能，用于用户身份认证。
// 核心知识点：const 常量组、struct 嵌入（embedding）、[]byte 与 string 互转、
// 类型断言（type assertion）、闭包（匿名函数）、错误包装（%w）。
package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// const (...) 定义一组常量。Go 的常量在编译期确定，不可修改。
// 大写开头 = 包外可见（exported），其他包可以用 jwt.TokenTypeAccess 来引用。
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Claims 自定义 JWT 载荷结构体。
// `json:"user_id"` 是结构体标签（struct tag），控制 JSON 序列化时的字段名。
// jwt.RegisteredClaims 没有字段名，这叫"嵌入"（embedding）——
// 相当于把 RegisteredClaims 的所有字段和方法"继承"到 Claims 里，可以直接访问。
type Claims struct {
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// GenerateAccessToken 生成访问令牌，默认 15 分钟过期，ttlMinutes 为 0 时使用默认值
// 参数列表中 userID, role, secret string 是简写，表示三个参数都是 string 类型。
func GenerateAccessToken(userID, role, secret string, ttlMinutes int) (string, error) {
	if ttlMinutes <= 0 {
		ttlMinutes = 15
	}
	// &Claims{...} 创建 Claims 结构体并取地址，得到 *Claims 指针。
	// 嵌入字段 RegisteredClaims 需要用字段名显式初始化。
	claims := &Claims{
		UserID:    userID,
		Role:      role,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			// time.Duration(ttlMinutes) 将 int 强制转换为 Duration 类型，Go 不允许不同类型直接运算。
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(ttlMinutes) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "autovideo-auth",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// []byte(secret) 将 string 转换为字节切片（[]byte）。
	// string 在 Go 中是不可变的 UTF-8 字节序列，[]byte 是可变的字节切片。
	// SignedString 要求传入 []byte 类型的密钥。
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	// 返回空字符串 "" 作为错误时的零值，成功时返回签名后的 token 字符串。
	return signed, nil
}

// GenerateRefreshToken 生成刷新令牌，默认 7 天过期，ttlDays 为 0 时使用默认值
func GenerateRefreshToken(userID, secret string, ttlDays int) (string, error) {
	if ttlDays <= 0 {
		ttlDays = 7
	}
	claims := &Claims{
		UserID:    userID,
		TokenType: TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(ttlDays) * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "autovideo-auth",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign refresh token: %w", err)
	}
	return signed, nil
}

// ParseToken 解析并验证 token
func ParseToken(tokenStr, secret string) (*Claims, error) {
	// func(t *jwt.Token) (interface{}, error) 是一个匿名函数（闭包），作为回调传入。
	// 闭包可以捕获外层变量 secret，所以内部能直接使用 secret。
	// 返回值 interface{} 是空接口，可以接收任意类型——Go 1.18 后也可写作 any。
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// t.Method.(*jwt.SigningMethodHMAC) 是"类型断言"（type assertion）。
		// 语法：value, ok := x.(具体类型)，ok 为 true 表示断言成功。
		// 这里用 _ 丢弃值，只关心 ok，目的是确认签名算法是 HMAC 系列。
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	// token.Claims.(*Claims) 又是一次类型断言，将通用的 Claims 接口转为我们自定义的 *Claims。
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}
