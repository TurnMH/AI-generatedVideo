package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response is the standard JSON envelope returned by all endpoints.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success —— 返回 HTTP 200 成功响应，携带数据负载
// Success writes a 200 OK response with the supplied data payload.
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// Created —— 返回 HTTP 201 创建成功响应
// Created writes a 201 Created response.
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

// Error —— 返回指定 HTTP 状态码的错误响应
// Error writes an error response with the given HTTP status code.
func Error(c *gin.Context, status int, message string) {
	c.JSON(status, Response{
		Code:    status,
		Message: message,
	})
}

// BadRequest —— 返回 HTTP 400 错误响应
// BadRequest writes a 400 Bad Request response.
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message)
}

// NotFound —— 返回 HTTP 404 错误响应
// NotFound writes a 404 Not Found response.
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message)
}

// InternalError —— 返回 HTTP 500 错误响应
// InternalError writes a 500 Internal Server Error response.
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, message)
}
