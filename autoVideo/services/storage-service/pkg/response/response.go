package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response is the standard API envelope.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success —— 返回 HTTP 200 成功响应，携带数据
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

// Error —— 返回指定 HTTP 状态码的错误响应
func Error(c *gin.Context, httpCode int, msg string) {
	c.JSON(httpCode, Response{Code: httpCode, Message: msg})
}

// BadRequest —— 返回 HTTP 400 错误响应
func BadRequest(c *gin.Context, msg string) {
	Error(c, http.StatusBadRequest, msg)
}

// InternalError —— 返回 HTTP 500 内部错误响应
func InternalError(c *gin.Context, msg string) {
	Error(c, http.StatusInternalServerError, msg)
}

// NotFound —— 返回 HTTP 404 未找到响应
func NotFound(c *gin.Context, msg string) {
	Error(c, http.StatusNotFound, msg)
}

// Forbidden —— 返回 HTTP 403 禁止访问响应
func Forbidden(c *gin.Context, msg string) {
	Error(c, http.StatusForbidden, msg)
}
