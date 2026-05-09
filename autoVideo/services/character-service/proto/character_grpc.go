// Package proto implements an internal HTTP-based RPC server that exposes
// character configuration to other services (e.g., image-service).
// Routes are served on /internal/grpc/* and are NOT protected by JWT auth.
package proto

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/autovideo/character-service/internal/service"
	"github.com/autovideo/character-service/pkg/response"
	"github.com/gin-gonic/gin"
)

type CharacterGRPCServer struct {
	svc *service.CharacterService
}

// NewCharacterGRPCServer —— 创建内部 gRPC 服务器实例，返回 *CharacterGRPCServer
func NewCharacterGRPCServer(svc *service.CharacterService) *CharacterGRPCServer {
	return &CharacterGRPCServer{svc: svc}
}

// Register —— 将内部 RPC 路由注册到指定路由组
// Register mounts the internal RPC endpoints on the given router group.
func (s *CharacterGRPCServer) Register(rg *gin.RouterGroup) {
	rg.GET("/GetCharacterConfig", s.GetCharacterConfig)
	rg.GET("/GetStylePreset", s.GetStylePreset)
}

// GetCharacterConfig —— 获取角色完整配置（含风格提示词），供内部服务调用
// GetCharacterConfig GET /internal/grpc/GetCharacterConfig?character_id=xxx
func (s *CharacterGRPCServer) GetCharacterConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Query("character_id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "character_id is required")
		return
	}
	cfg, err := s.svc.GetCharacterConfig(id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "character not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, cfg)
}

// GetStylePreset —— 根据名称获取风格预设配置，供内部服务调用
// GetStylePreset GET /internal/grpc/GetStylePreset?name=anime
func (s *CharacterGRPCServer) GetStylePreset(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		response.Error(c, http.StatusBadRequest, "name is required")
		return
	}
	sp, err := s.svc.GetStylePreset(name)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "style preset not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, sp)
}
