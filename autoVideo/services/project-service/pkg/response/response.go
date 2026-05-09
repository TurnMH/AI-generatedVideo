package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ListResponse struct {
	Code     int         `json:"code"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data,omitempty"`
	PageInfo *PageInfo   `json:"page_info,omitempty"`
}

type PageInfo struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// OK —— 返回 HTTP 200 成功响应
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "ok",
		Data:    data,
	})
}

// OKList —— 返回 HTTP 200 带分页信息的列表响应
func OKList(c *gin.Context, data interface{}, page, pageSize int, total int64) {
	c.JSON(http.StatusOK, ListResponse{
		Code:    200,
		Message: "ok",
		Data:    data,
		PageInfo: &PageInfo{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	})
}

// Fail —— 返回指定 HTTP 状态码和错误码的失败响应
func Fail(c *gin.Context, httpStatus, code int, message string) {
	c.AbortWithStatusJSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

// BadRequest —— 返回 HTTP 400 参数错误响应
func BadRequest(c *gin.Context, message string) {
	Fail(c, http.StatusBadRequest, 400, message)
}

// Unauthorized —— 返回 HTTP 401 未授权响应
func Unauthorized(c *gin.Context, message string) {
	Fail(c, http.StatusUnauthorized, 401, message)
}

// Forbidden —— 返回 HTTP 403 禁止访问响应
func Forbidden(c *gin.Context, message string) {
	Fail(c, http.StatusForbidden, 403, message)
}

// NotFound —— 返回 HTTP 404 资源不存在响应
func NotFound(c *gin.Context, message string) {
	Fail(c, http.StatusNotFound, 404, message)
}

// InternalError —— 返回 HTTP 500 内部错误响应
func InternalError(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, 500, message)
}
