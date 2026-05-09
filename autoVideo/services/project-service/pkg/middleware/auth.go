package middleware

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcjson "github.com/autovideo/project-service/pkg/codec"
	"github.com/autovideo/project-service/pkg/config"
	"github.com/autovideo/project-service/pkg/response"
)

// errGRPCUnavailable is returned by validateViaGRPC when the auth-service is
// unreachable (connection error). It is distinct from a rejection (invalid token).
var errGRPCUnavailable = errors.New("grpc auth service unavailable")

// grpcVerifyTokenReq matches auth-service VerifyToken gRPC request.
type grpcValidateReq struct {
	Token string `json:"token"`
}

// grpcVerifyTokenResp matches auth-service VerifyToken gRPC response.
type grpcValidateResp struct {
	Valid   bool   `json:"valid"`
	UserID  string `json:"user_id"`
	Role    string `json:"role"`
	Message string `json:"message"`
}

// AuthMiddleware —— JWT 认证中间件，优先 gRPC 校验，降级为本地 JWT 解析
// AuthMiddleware validates JWT tokens by first attempting gRPC call to auth-service,
// and falling back to local JWT verification if gRPC is unavailable.
func AuthMiddleware(cfg *config.Config, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Unauthorized(c, "invalid authorization header format")
			return
		}
		tokenStr := parts[1]

		// Fast-path: service-to-service tokens have role="service" and are not issued by
		// auth-service, so gRPC validation would reject them. Validate locally instead.
		if localUserID, localRole, localErr := validateLocalJWT(tokenStr, cfg.JWT.AccessSecret); localErr == nil && localRole == "service" {
			c.Set("user_id", localUserID)
			c.Set("role", "service")
			c.Next()
			return
		}

		// Try gRPC auth-service first.
		userID, role, err := validateViaGRPC(cfg.AuthService.GRPCAddr, tokenStr, logger)
		if err != nil {
			if !errors.Is(err, errGRPCUnavailable) {
				// gRPC connected and explicitly rejected the token — do not fall back.
				response.Unauthorized(c, "invalid or expired token")
				return
			}
			// gRPC is down — fall back to local JWT parse.
			logger.Warn("gRPC auth unavailable, falling back to local JWT", zap.Error(err))
			userID, role, err = validateLocalJWT(tokenStr, cfg.JWT.AccessSecret)
			if err != nil {
				response.Unauthorized(c, "invalid or expired token")
				return
			}
		}

		c.Set("user_id", userID)
		c.Set("role", role)
		c.Next()
	}
}

// validateViaGRPC —— 通过 gRPC 调用 auth-service 校验 JWT 令牌
// validateViaGRPC calls auth-service VerifyToken via gRPC JSON codec.
// Returns errGRPCUnavailable when the service cannot be reached (caller may fall back).
// Returns jwt.ErrTokenInvalidClaims when the service rejects the token (caller must 401).
func validateViaGRPC(addr, token string, logger *zap.Logger) (uint64, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return 0, "", errGRPCUnavailable
	}
	defer conn.Close()

	req := &grpcValidateReq{Token: token}
	resp := &grpcValidateResp{}

	err = conn.Invoke(ctx, "/auth.AuthService/VerifyToken", req, resp, grpc.ForceCodec(grpcjson.JSONCodec{}))
	if err != nil {
		return 0, "", errGRPCUnavailable
	}
	if !resp.Valid {
		return 0, "", jwt.ErrTokenInvalidClaims
	}
	userID, err := strconv.ParseUint(resp.UserID, 10, 64)
	if err != nil || userID == 0 {
		return 0, "", jwt.ErrTokenInvalidClaims
	}
	return userID, resp.Role, nil
}

// validateLocalJWT —— 使用本地密钥解析并校验 JWT 令牌
// validateLocalJWT parses the JWT locally using the shared secret.
func validateLocalJWT(tokenStr, secret string) (uint64, string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))
	if err != nil {
		return 0, "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, "", jwt.ErrTokenInvalidClaims
	}

	var userID uint64
	switch v := claims["user_id"].(type) {
	case float64:
		userID = uint64(v)
	case uint64:
		userID = v
	case int64:
		userID = uint64(v)
	case string:
		parsed, parseErr := strconv.ParseUint(v, 10, 64)
		if parseErr != nil {
			return 0, "", fmt.Errorf("invalid user_id claim: %w", parseErr)
		}
		userID = parsed
	default:
		return 0, "", jwt.ErrTokenInvalidClaims
	}
	if userID == 0 {
		return 0, "", jwt.ErrTokenInvalidClaims
	}

	role, _ := claims["role"].(string)
	return userID, role, nil
}

// GetUserID —— 从 gin 上下文中提取已认证的用户 ID
// GetUserID extracts the authenticated user_id from the gin context.
func GetUserID(c *gin.Context) uint64 {
	val, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	if id, ok := val.(uint64); ok {
		return id
	}
	return 0
}

// GetRole —— 从 gin 上下文中提取已认证的用户角色
// GetRole extracts the authenticated role from the gin context.
func GetRole(c *gin.Context) string {
	val, exists := c.Get("role")
	if !exists {
		return ""
	}
	if role, ok := val.(string); ok {
		return role
	}
	return ""
}
