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

// Success —— 返回 HTTP 200 成功响应，包含统一的 JSON 格式数据
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "ok",
		Data:    data,
	})
}

// Error —— 返回指定 HTTP 状态码的错误响应，包含错误码和错误信息
func Error(c *gin.Context, httpStatus int, msg string) {
	c.JSON(httpStatus, Response{
		Code:    httpStatus,
		Message: msg,
		Data:    nil,
	})
}
