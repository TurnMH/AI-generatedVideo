package grpcauth

import (
	"context"
	"strconv"

	"github.com/autovideo/auth-service/internal/service"
	jwtpkg "github.com/autovideo/auth-service/pkg/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type VerifyTokenRequest struct {
	Token string `json:"token"`
}

type VerifyTokenResponse struct {
	Valid   bool   `json:"valid"`
	UserID  string `json:"user_id"`
	Role    string `json:"role"`
	Message string `json:"message"`
}

type AuthServiceServer interface {
	VerifyToken(context.Context, *VerifyTokenRequest) (*VerifyTokenResponse, error)
}

type Handler struct {
	authSvc      service.AuthService
	accessSecret string
}

func NewHandler(authSvc service.AuthService, accessSecret string) *Handler {
	return &Handler{
		authSvc:      authSvc,
		accessSecret: accessSecret,
	}
}

func (h *Handler) VerifyToken(ctx context.Context, req *VerifyTokenRequest) (*VerifyTokenResponse, error) {
	if req == nil || req.Token == "" {
		return &VerifyTokenResponse{
			Valid:   false,
			Message: "token required",
		}, nil
	}

	claims, err := jwtpkg.ParseToken(req.Token, h.accessSecret)
	if err != nil {
		return &VerifyTokenResponse{
			Valid:   false,
			Message: "invalid token",
		}, nil
	}
	if claims.TokenType != jwtpkg.TokenTypeAccess {
		return &VerifyTokenResponse{
			Valid:   false,
			Message: "invalid token type",
		}, nil
	}

	userID, err := strconv.ParseUint(claims.UserID, 10, 64)
	if err != nil || userID == 0 {
		return &VerifyTokenResponse{
			Valid:   false,
			Message: "invalid user id",
		}, nil
	}

	user, err := h.authSvc.GetUser(userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user: %v", err)
	}
	if user.Status != "active" {
		return &VerifyTokenResponse{
			Valid:   false,
			Message: "user inactive",
		}, nil
	}

	return &VerifyTokenResponse{
		Valid:   true,
		UserID:  claims.UserID,
		Role:    claims.Role,
		Message: "ok",
	}, nil
}

func RegisterAuthServiceServer(s grpc.ServiceRegistrar, srv AuthServiceServer) {
	s.RegisterService(&AuthService_ServiceDesc, srv)
}

func _AuthService_VerifyToken_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(VerifyTokenRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).VerifyToken(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/auth.AuthService/VerifyToken",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).VerifyToken(ctx, req.(*VerifyTokenRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var AuthService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "auth.AuthService",
	HandlerType: (*AuthServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "VerifyToken",
			Handler:    _AuthService_VerifyToken_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/auth.proto",
}
