package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/autovideo/task-service/internal/hub"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	// In production, validate the Origin header instead.
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WSHandler handles WebSocket upgrade requests.
type WSHandler struct {
	wsHub     *hub.Hub
	logger    *zap.Logger
	jwtSecret string
}

// NewWSHandler —— 创建 WSHandler 实例，jwtSecret 用于验证 Bearer token
// NewWSHandler creates a WSHandler. jwtSecret is the shared HS256 signing key.
func NewWSHandler(wsHub *hub.Hub, logger *zap.Logger, jwtSecret string) *WSHandler {
	return &WSHandler{wsHub: wsHub, logger: logger, jwtSecret: jwtSecret}
}

// SubscribeTask —— 处理 WebSocket 升级请求，客户端订阅指定任务的实时事件
// SubscribeTask handles GET /ws/tasks/:id.
// Clients must supply a ?token=<JWT> query parameter for authentication.
func (h *WSHandler) SubscribeTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	token := c.Query("token")
	userID, err := h.parseTokenUserID(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("ws upgrade error", zap.Error(err))
		return
	}

	h.logger.Info("ws client connected to task", zap.Uint64("task_id", taskID), zap.Uint64("user_id", userID))
	h.wsHub.ServeWS(taskID, 0, userID, conn)
}

// SubscribeProject —— 处理 WebSocket 升级请求，客户端订阅指定项目下所有任务的实时事件
// SubscribeProject handles GET /ws/projects/:project_id.
// Clients subscribe to all task events for a project.
func (h *WSHandler) SubscribeProject(c *gin.Context) {
	projectIDStr := c.Param("project_id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	token := c.Query("token")
	userID, err := h.parseTokenUserID(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("ws upgrade error", zap.Error(err))
		return
	}

	h.logger.Info("ws client connected to project", zap.Uint64("project_id", projectID), zap.Uint64("user_id", userID))
	h.wsHub.ServeWS(0, projectID, userID, conn)
}

// jwtClaims mirrors the auth-service JWT payload (user_id, role, token_type).
type jwtClaims struct {
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// parseTokenUserID —— 验证 HS256 JWT 并返回用户 ID
// parseTokenUserID validates the JWT using the shared HS256 secret and returns
// the numeric user ID from the claims. Returns (0, nil) for empty tokens so
// anonymous / internal connections still work in dev.
func (h *WSHandler) parseTokenUserID(tokenStr string) (uint64, error) {
	if tokenStr == "" {
		return 0, nil // allow anonymous (dev / internal)
	}

	// If no secret is configured, fall back to treating the token as a raw user ID (dev mode).
	if h.jwtSecret == "" {
		uid, _ := strconv.ParseUint(tokenStr, 10, 64)
		return uid, nil
	}

	tok, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(h.jwtSecret), nil
	})
	if err != nil || !tok.Valid {
		return 0, errors.New("invalid token")
	}

	claims, ok := tok.Claims.(*jwtClaims)
	if !ok {
		return 0, errors.New("invalid claims")
	}

	uid, err := strconv.ParseUint(claims.UserID, 10, 64)
	if err != nil {
		return 0, errors.New("invalid user_id in token")
	}
	return uid, nil
}

