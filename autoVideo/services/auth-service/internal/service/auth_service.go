// 【小白注解】本文件是认证服务的核心业务逻辑层（Service 层）。
// 它定义了 AuthService 接口和具体实现，负责：注册、登录、Token 刷新、登出、用户信息查改。
// 关键 Go 知识点：interface 接口、私有 struct、多返回值、错误处理链、方法接收器。

package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/autovideo/auth-service/internal/model"
	"github.com/autovideo/auth-service/internal/repository"
	"github.com/autovideo/auth-service/pkg/config"
	jwtpkg "github.com/autovideo/auth-service/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// 【小白注解】struct 是 Go 的自定义类型，类似其他语言的 class。
// 大写开头的字段/类型是"导出的"（公开），外部包可以访问。
type RegisterReq struct {
	Username string
	Email    string
	Phone    string
	Password string
}

type UpdateUserReq struct {
	Username  string
	AvatarURL string
	Phone     string
}

// 【小白注解】interface 是 Go 的接口类型，只定义方法签名，不包含实现。
// 任何 struct 只要实现了接口里的所有方法，就自动满足该接口（隐式实现，不需要 implements 关键字）。
// 括号里的返回值如 (*model.User, string, string, error) 就是 Go 的"多返回值"特性，
// 最后一个 error 是 Go 惯用的错误处理模式：用返回值而非异常来传递错误。
type AuthService interface {
	Register(req RegisterReq) (*model.User, string, string, error)
	LoginPassword(email, password string) (*model.User, string, string, error)
	RefreshToken(refreshToken string) (string, string, error)
	Logout(refreshToken string) error
	GetUser(userID uint64) (*model.User, error)
	UpdateUser(userID uint64, req UpdateUserReq) (*model.User, error)
}

// 【小白注解】authService 小写开头 —— 这是"未导出"（私有）struct，外部包无法直接创建它。
// 外部只能通过下面的 NewAuthService 工厂函数获取实例，并以 AuthService 接口类型来使用。
// 这是 Go 中实现"面向接口编程"的常见模式。
type authService struct {
	userRepo  repository.UserRepository
	tokenRepo repository.TokenRepository
	cfg       *config.Config
}

// 【小白注解】NewAuthService 是工厂函数，返回值类型是 AuthService 接口（而不是 *authService）。
// 这样调用方只依赖接口，不依赖具体实现，方便测试和替换。
func NewAuthService(userRepo repository.UserRepository, tokenRepo repository.TokenRepository, cfg *config.Config) AuthService {
	return &authService{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		cfg:       cfg,
	}
}

// hashToken 对 token 字符串做 SHA-256 哈希，用于数据库存储
// 【小白注解】这是一个普通函数（没有接收器），小写开头所以包外不可见。
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// 【小白注解】func (s *authService) —— 这是"方法接收器"语法。
// (s *authService) 表示 Register 是 authService 的方法，s 就是"this/self"。
// * 号表示用指针接收器，方法内可以修改 s 的字段，且避免值拷贝。
func (s *authService) Register(req RegisterReq) (*model.User, string, string, error) {
	// 验证邮箱唯一性
	// 【小白注解】Go 的错误处理链模式：先调用函数，检查 err 是否为 nil，
	// 再用 errors.Is() 判断具体错误类型，逐层处理不同情况。
	if req.Email != "" {
		_, err := s.userRepo.FindByEmail(req.Email)
		// 【小白注解】err == nil 说明查到了记录，即邮箱已注册
		if err == nil {
			return nil, "", "", errors.New("email already registered")
		}
		// 【小白注解】errors.Is 用来判断错误链中是否包含特定错误。
		// 如果不是"记录未找到"，说明是其他数据库错误，需要上报。
		// fmt.Errorf("...%w", err) 中的 %w 会"包装"原始错误，保留错误链。
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", "", fmt.Errorf("check email: %w", err)
		}
	}

	// 验证手机号唯一性
	if req.Phone != "" {
		_, err := s.userRepo.FindByPhone(req.Phone)
		if err == nil {
			return nil, "", "", errors.New("phone already registered")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", "", fmt.Errorf("check phone: %w", err)
		}
	}

	// 验证用户名唯一性
	if req.Username != "" {
		_, err := s.userRepo.FindByUsername(req.Username)
		if err == nil {
			return nil, "", "", errors.New("username already taken")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", "", fmt.Errorf("check username: %w", err)
		}
	}

	// bcrypt 哈希密码 cost=12
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, "", "", fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(hash),
		Role:         "user",
		Status:       "active",
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, "", "", fmt.Errorf("create user: %w", err)
	}

	// 【小白注解】多返回值接收：Go 函数可以返回多个值，用逗号分隔接收。
	accessToken, refreshToken, err := s.generateAndStoreTokens(user)
	if err != nil {
		return nil, "", "", err
	}

	// 【小白注解】return 4 个值对应函数签名中的 (*model.User, string, string, error)。
	return user, accessToken, refreshToken, nil
}

func (s *authService) LoginPassword(email, password string) (*model.User, string, string, error) {
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", "", errors.New("invalid credentials")
		}
		return nil, "", "", fmt.Errorf("find user: %w", err)
	}

	if user.Status != "active" {
		return nil, "", "", errors.New("account is not active")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", "", errors.New("invalid credentials")
	}

	accessToken, refreshToken, err := s.generateAndStoreTokens(user)
	if err != nil {
		return nil, "", "", err
	}

	return user, accessToken, refreshToken, nil
}

func (s *authService) RefreshToken(refreshToken string) (string, string, error) {
	// 验证签名
	claims, err := jwtpkg.ParseToken(refreshToken, s.cfg.JWT.RefreshSecret)
	if err != nil {
		return "", "", errors.New("invalid refresh token")
	}
	if claims.TokenType != jwtpkg.TokenTypeRefresh {
		return "", "", errors.New("invalid token type")
	}

	// 查数据库验证未被吊销
	tokenHash := hashToken(refreshToken)
	tokenRecord, err := s.tokenRepo.FindByHash(tokenHash)
	if err != nil {
		return "", "", errors.New("refresh token revoked or not found")
	}

	if time.Now().After(tokenRecord.ExpiresAt) {
		// 【小白注解】_ 是空白标识符，用来丢弃不需要的返回值（这里忽略了删除可能返回的错误）。
		_ = s.tokenRepo.Delete(tokenRecord.ID)
		return "", "", errors.New("refresh token expired")
	}

	// 查询用户
	// 【小白注解】strconv.ParseUint 把字符串转成无符号整数；参数 10 是十进制，64 是位宽。
	userIDUint, _ := strconv.ParseUint(claims.UserID, 10, 64)
	user, err := s.userRepo.FindByID(userIDUint)
	if err != nil {
		return "", "", fmt.Errorf("find user: %w", err)
	}

	// 删旧 token（轮转）
	if err := s.tokenRepo.Delete(tokenRecord.ID); err != nil {
		return "", "", fmt.Errorf("revoke old token: %w", err)
	}

	// 生成新双 Token
	newAccess, newRefresh, err := s.generateAndStoreTokens(user)
	if err != nil {
		return "", "", err
	}

	return newAccess, newRefresh, nil
}

// 【小白注解】Logout 只返回一个 error —— 当函数只需要报告成功/失败时，只返回 error 即可。
func (s *authService) Logout(refreshToken string) error {
	tokenHash := hashToken(refreshToken)
	tokenRecord, err := s.tokenRepo.FindByHash(tokenHash)
	if err != nil {
		// 已吊销或不存在，视为成功
		return nil
	}
	return s.tokenRepo.Delete(tokenRecord.ID)
}

func (s *authService) GetUser(userID uint64) (*model.User, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *authService) UpdateUser(userID uint64, req UpdateUserReq) (*model.User, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	if req.Username != "" && req.Username != user.Username {
		_, checkErr := s.userRepo.FindByUsername(req.Username)
		if checkErr == nil {
			return nil, errors.New("username already taken")
		}
		user.Username = req.Username
	}

	if req.Phone != "" && req.Phone != user.Phone {
		_, checkErr := s.userRepo.FindByPhone(req.Phone)
		if checkErr == nil {
			return nil, errors.New("phone already registered")
		}
		user.Phone = req.Phone
	}

	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}

	if err := s.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return user, nil
}

// generateAndStoreTokens 生成 access + refresh token 并将 refresh token 存库
// 【小白注解】小写开头的方法（generateAndStoreTokens）是私有方法，只能在本包内调用。
func (s *authService) generateAndStoreTokens(user *model.User) (string, string, error) {
	// 【小白注解】strconv.FormatUint 把无符号整数转成字符串；参数 10 表示十进制。
	userIDStr := strconv.FormatUint(user.ID, 10)

	accessToken, err := jwtpkg.GenerateAccessToken(userIDStr, user.Role, s.cfg.JWT.AccessSecret, s.cfg.JWT.AccessTTL)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := jwtpkg.GenerateRefreshToken(userIDStr, s.cfg.JWT.RefreshSecret, s.cfg.JWT.RefreshTTL)
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	tokenRecord := &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashToken(refreshToken),
		ExpiresAt: time.Now().Add(time.Duration(s.cfg.JWT.RefreshTTL) * 24 * time.Hour),
	}
	if err := s.tokenRepo.Create(tokenRecord); err != nil {
		return "", "", fmt.Errorf("store refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}
