// 【小白注解】本文件是认证服务的 HTTP 处理层（Handler 层）。
// 它负责：接收 HTTP 请求 → 解析/校验 JSON 参数 → 调用 Service 层 → 返回 JSON 响应。
// 关键 Go 知识点：struct tag（json/binding）、JSON 绑定、HTTP handler 模式、strconv 类型转换、errors.Is。

package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/autovideo/auth-service/internal/service"
	"github.com/autovideo/auth-service/pkg/middleware"
	"github.com/autovideo/auth-service/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 【小白注解】AuthHandler 大写开头，是导出的 struct，外部包可以使用。
// 它持有 service 层的接口引用，遵循"依赖注入"模式。
type AuthHandler struct {
	authSvc service.AuthService
}

// 【小白注解】构造函数模式：Go 没有构造器，习惯用 NewXxx 函数来创建并返回实例。
func NewAuthHandler(authSvc service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

// 【小白注解】struct tag 是反引号 `` 里的元数据，用于告诉框架如何处理字段：
//   - json:"username"    → JSON 序列化/反序列化时使用 "username" 作为字段名
//   - binding:"required" → Gin 框架校验时该字段必填
//   - binding:"min=3,max=64" → 长度限制，binding:"email" → 必须是邮箱格式
// 小写开头的 struct（registerRequest）是包内私有的，外部无法直接引用。
type registerRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone"`
	Password string `json:"password" binding:"required,min=8"`
}

type loginPasswordRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type updateMeRequest struct {
	Username  string `json:"username"`
	Phone     string `json:"phone"`
	AvatarURL string `json:"avatar_url"`
}

// RegisterHandler POST /api/v1/auth/register
// 【小白注解】HTTP Handler 模式：每个 handler 方法接收 *gin.Context 参数，
// 它封装了 HTTP 请求和响应的所有信息（请求体、URL 参数、响应写入等）。
func (h *AuthHandler) RegisterHandler(c *gin.Context) {
	// 【小白注解】var req registerRequest —— 先声明变量，零值初始化（所有字段为空字符串）。
	var req registerRequest
	// 【小白注解】c.ShouldBindJSON(&req) —— 将请求体的 JSON 解析到 req 结构体中。
	// &req 传指针，这样函数内部可以修改 req 的字段值。
	// 同时会按 binding tag 做校验，校验失败返回 error。
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, accessToken, refreshToken, err := h.authSvc.Register(service.RegisterReq{
		Username: req.Username,
		Email:    req.Email,
		Phone:    req.Phone,
		Password: req.Password,
	})
	if err != nil {
		// 【小白注解】http.StatusConflict 是标准库定义的 HTTP 状态码常量（409）。
		response.Error(c, http.StatusConflict, 4090, err.Error())
		return
	}

	// 【小白注解】gin.H{} 是 map[string]any 的简写，方便快速构造 JSON 响应。
	response.Success(c, gin.H{
		"user":          user,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// LoginPasswordHandler POST /api/v1/auth/login/password
func (h *AuthHandler) LoginPasswordHandler(c *gin.Context) {
	var req loginPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, accessToken, refreshToken, err := h.authSvc.LoginPassword(req.Email, req.Password)
	if err != nil {
		response.Unauthorized(c, "invalid credentials")
		return
	}

	response.Success(c, gin.H{
		"user":          user,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// RefreshHandler POST /api/v1/auth/token/refresh
func (h *AuthHandler) RefreshHandler(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	newAccess, newRefresh, err := h.authSvc.RefreshToken(req.RefreshToken)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"access_token":  newAccess,
		"refresh_token": newRefresh,
	})
}

// LogoutHandler POST /api/v1/auth/logout (需要 AuthRequired)
func (h *AuthHandler) LogoutHandler(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.authSvc.Logout(req.RefreshToken); err != nil {
		response.InternalError(c, "logout failed")
		return
	}

	response.Success(c, nil)
}

// MeHandler GET /api/v1/auth/me (需要 AuthRequired)
func (h *AuthHandler) MeHandler(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	// 【小白注解】strconv.ParseUint(字符串, 进制, 位宽) —— 将字符串转为无符号整数。
	// Go 是强类型语言，不会自动转换类型，所以需要显式调用 strconv 包来做类型转换。
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		response.Unauthorized(c, "invalid token")
		return
	}

	user, err := h.authSvc.GetUser(userID)
	if err != nil {
		// 【小白注解】errors.Is(err, target) —— 判断错误链中是否包含特定错误。
		// 比起 err == target，它能穿透 fmt.Errorf("%w", err) 包装过的多层错误。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		response.InternalError(c, "get user failed")
		return
	}

	response.Success(c, user)
}

// UpdateMeHandler PUT /api/v1/auth/me (需要 AuthRequired)
func (h *AuthHandler) UpdateMeHandler(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		response.Unauthorized(c, "invalid token")
		return
	}

	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := h.authSvc.UpdateUser(userID, service.UpdateUserReq{
		Username:  req.Username,
		AvatarURL: req.AvatarURL,
		Phone:     req.Phone,
	})
	if err != nil {
		response.Error(c, http.StatusConflict, 4090, err.Error())
		return
	}

	response.Success(c, user)
}
