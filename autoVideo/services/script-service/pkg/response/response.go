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

// Success —— 返回 HTTP 200 成功响应，包含统一的 JSON 格式数据
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "ok",
		Data:    data,
	})
}

// Error —— 根据业务错误码映射 HTTP 状态码，返回统一格式的错误响应
func Error(c *gin.Context, code int, message string) {
	httpStatus := http.StatusInternalServerError
	switch {
	case code == 4001:
		httpStatus = http.StatusUnauthorized
	case code == 4003:
		httpStatus = http.StatusForbidden
	case code == 4004:
		httpStatus = http.StatusNotFound
	case code >= 4000 && code < 5000:
		httpStatus = http.StatusBadRequest
	}
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}
