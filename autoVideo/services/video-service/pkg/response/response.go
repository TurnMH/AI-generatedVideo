package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// OK —— 返回 HTTP 200 成功响应，携带数据
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: 200, Message: "ok", Data: data})
}

// Fail —— 返回指定 HTTP 状态码的失败响应
func Fail(c *gin.Context, httpCode, bizCode int, msg string) {
	c.JSON(httpCode, Response{Code: bizCode, Message: msg, Data: nil})
}

// BadRequest —— 返回 HTTP 400 错误响应
func BadRequest(c *gin.Context, msg string) {
	Fail(c, http.StatusBadRequest, 400, msg)
}

// Unauthorized —— 返回 HTTP 401 未授权响应
func Unauthorized(c *gin.Context) {
	Fail(c, http.StatusUnauthorized, 401, "unauthorized")
}

// NotFound —— 返回 HTTP 404 未找到响应
func NotFound(c *gin.Context, msg string) {
	Fail(c, http.StatusNotFound, 404, msg)
}

// InternalError —— 返回 HTTP 500 内部错误响应
func InternalError(c *gin.Context, msg string) {
	Fail(c, http.StatusInternalServerError, 500, msg)
}
