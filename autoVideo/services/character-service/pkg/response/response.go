package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// Success —— 返回 HTTP 200 成功响应，包含数据
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "ok",
		Data:    data,
	})
}

// SuccessWithStatus —— 返回指定 HTTP 状态码的成功响应，包含数据
func SuccessWithStatus(c *gin.Context, status int, data interface{}) {
	c.JSON(status, Response{
		Code:    200,
		Message: "ok",
		Data:    data,
	})
}

// Error —— 返回指定 HTTP 状态码的错误响应，包含错误信息
func Error(c *gin.Context, status int, msg string) {
	c.JSON(status, Response{
		Code:    status,
		Message: msg,
		Data:    nil,
	})
}
