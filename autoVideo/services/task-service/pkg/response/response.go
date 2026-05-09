package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response is the standard JSON envelope.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PageResponse wraps paginated results.
type PageResponse struct {
	Items    interface{} `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// OK —— 返回 HTTP 200 成功响应，携带数据负载
// OK sends a 200 response with data.
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

// Created —— 返回 HTTP 201 创建成功响应
// Created sends a 201 response with data.
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{Code: 0, Message: "created", Data: data})
}

// BadRequest —— 返回 HTTP 400 错误响应
// BadRequest sends a 400 response.
func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, Response{Code: 400, Message: msg})
}

// NotFound —— 返回 HTTP 404 错误响应
// NotFound sends a 404 response.
func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, Response{Code: 404, Message: msg})
}

// Forbidden —— 返回 HTTP 403 错误响应
// Forbidden sends a 403 response.
func Forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, Response{Code: 403, Message: msg})
}

// InternalError —— 返回 HTTP 500 错误响应
// InternalError sends a 500 response.
func InternalError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: msg})
}

// Page —— 返回 HTTP 200 分页响应，包含数据列表和分页信息
// Page sends a paginated 200 response.
func Page(c *gin.Context, items interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data: PageResponse{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}
