// 本文件的作用：统一 HTTP 响应格式。
// 所有 API 接口都通过这里的函数返回 JSON，保证前端拿到的结构一致：
//   { "code": 200, "message": "ok", "data": ... }
// 同时封装了常见 HTTP 错误码（400/401/403/404/429/500）的快捷方法。

package response

import (
	"net/http" // 标准库，提供 HTTP 状态码常量，如 http.StatusOK = 200

	"github.com/gin-gonic/gin" // 第三方 Web 框架，处理 HTTP 请求/响应
)

// Response —— 统一的 JSON 响应结构体。
// struct tag `json:"code"` 告诉 JSON 序列化器：把 Code 字段输出为 "code"。
// `json:"data,omitempty"` 中的 omitempty 表示：如果 Data 为空，则 JSON 中省略该字段。
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"` // interface{} 是"空接口"，可以接收任意类型的值
}

// Success 返回 200 成功响应
// 函数签名解读：
//   - c *gin.Context : 参数 c，类型是 *gin.Context（指针）。
//     gin 把每个 HTTP 请求的所有信息都放在 Context 里，通过指针传递避免拷贝。
//   - data interface{} : 第二个参数，接受任意类型的数据
//   - 无返回值（函数签名末尾没有返回类型）
func Success(c *gin.Context, data interface{}) {
	// Response{Code: 200, ...} —— 结构体字面量，用字段名初始化
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "ok",
		Data:    data,
	})
}

// Error 返回错误响应，httpStatus 为 HTTP 状态码，code 为业务错误码
// 多个参数用逗号分隔；相同类型的相邻参数不能合并写（这里 httpStatus 和 code 分开声明）
func Error(c *gin.Context, httpStatus int, code int, message string) {
	// AbortWithStatusJSON —— 返回 JSON 并中止后续中间件的执行
	c.AbortWithStatusJSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

// 以下是便捷函数：对 Error 的封装，省去每次传 HTTP 状态码和业务码。
// 它们都调用上面的 Error 函数，体现了 Go 中"组合复用"的思想。

// BadRequest 400
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, 4000, message)
}

// Unauthorized 401
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, 4001, message)
}

// Forbidden 403
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, 4003, message)
}

// NotFound 404
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, 4004, message)
}

// TooManyRequests 429
// 注意这个函数只有一个参数 c，message 写死了 —— 限流场景下消息固定
func TooManyRequests(c *gin.Context) {
	Error(c, http.StatusTooManyRequests, 4029, "too many requests")
}

// InternalError 500
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, 5000, message)
}
