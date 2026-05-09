// 本文件实现 JWT 鉴权中间件：从 HTTP 请求头中提取 Token，验证后把用户信息存入上下文。
// 涉及 Go 基础知识：闭包（closure）、gin.HandlerFunc 类型、字符串操作、类型断言。
package middleware

import (
	"strings"

	"github.com/autovideo/auth-service/pkg/jwt"
	"github.com/autovideo/auth-service/pkg/response"
	"github.com/gin-gonic/gin"
)

// const 块定义常量——多个常量可放在同一个括号里，值在编译期确定、不可修改。
const (
	ContextUserID = "userID"
	ContextRole   = "role"
)

// AuthRequired 验证 Authorization: Bearer <token>，将 userID/role 写入 Context
//
// 【闭包 & 高阶函数】
// 函数签名：接收 string，返回 gin.HandlerFunc（即 func(*gin.Context)）。
// 返回的匿名函数"捕获"了外部变量 accessSecret，形成闭包（closure）——
// 即使 AuthRequired 已返回，内层函数仍能访问 accessSecret。
func AuthRequired(accessSecret string) gin.HandlerFunc {
	// 【gin.HandlerFunc】类型定义为 func(*gin.Context)，是 Gin 框架中间件的标准签名。
	return func(c *gin.Context) {
		// c.GetHeader 从 HTTP 请求头中读取指定字段的值
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "invalid token")
			// return 提前结束当前函数，后续代码不再执行（相当于拒绝请求）
			return
		}

		// 【strings.SplitN】按分隔符 " " 最多拆成 2 段，返回一个字符串切片（slice）。
		// 例如 "Bearer abc123" → ["Bearer", "abc123"]
		// 第三个参数 2 表示最多分成 2 个子串，避免 Token 内含空格时被多余拆分。
		parts := strings.SplitN(authHeader, " ", 2)
		// 【len()】内置函数，返回切片的元素个数。
		// 【strings.EqualFold】不区分大小写地比较两个字符串（如 "bearer" == "Bearer"）。
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Unauthorized(c, "invalid token")
			return
		}

		// parts[1] 通过下标访问切片中的第二个元素（下标从 0 开始）
		tokenStr := parts[1]
		// jwt.ParseToken 解析 Token 字符串，返回 (claims, error) 两个值——Go 的多返回值特性。
		claims, err := jwt.ParseToken(tokenStr, accessSecret)
		if err != nil {
			response.Unauthorized(c, "invalid token")
			return
		}

		// 用 != 比较来确认 Token 类型是 access 而非 refresh
		if claims.TokenType != jwt.TokenTypeAccess {
			response.Unauthorized(c, "invalid token")
			return
		}

		// c.Set 将键值对存入 gin.Context，供同一请求的后续处理函数读取
		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextRole, claims.Role)
		// c.Next() 把控制权交给下一个中间件或路由处理函数
		c.Next()
	}
}

// GetUserID 从 gin.Context 中取 userID
func GetUserID(c *gin.Context) string {
	// c.Get 返回 (value interface{}, exists bool)；这里用 _ 忽略第二个返回值。
	val, _ := c.Get(ContextUserID)
	// 【类型断言】val.(string) 将 interface{} 类型的 val 转为 string。
	// 双返回值写法：ok 为 true 表示断言成功，false 表示 val 不是 string（此时 id 为零值 ""）。
	id, _ := val.(string)
	return id
}

// GetRole 从 gin.Context 中取 role
func GetRole(c *gin.Context) string {
	val, _ := c.Get(ContextRole)
	// 同上，类型断言：把 interface{} 断言为 string
	role, _ := val.(string)
	return role
}
