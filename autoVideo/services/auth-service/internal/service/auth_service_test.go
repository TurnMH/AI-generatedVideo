package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/autovideo/auth-service/internal/model"
	"github.com/autovideo/auth-service/pkg/config"
	jwtpkg "github.com/autovideo/auth-service/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ── Mock implementations ──────────────────────────────────────────────────────

type mockUserRepo struct{ mock.Mock }

func (m *mockUserRepo) Create(user *model.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *mockUserRepo) FindByEmail(email string) (*model.User, error) {
	args := m.Called(email)
	u, _ := args.Get(0).(*model.User)
	return u, args.Error(1)
}

func (m *mockUserRepo) FindByPhone(phone string) (*model.User, error) {
	args := m.Called(phone)
	u, _ := args.Get(0).(*model.User)
	return u, args.Error(1)
}

func (m *mockUserRepo) FindByUsername(username string) (*model.User, error) {
	args := m.Called(username)
	u, _ := args.Get(0).(*model.User)
	return u, args.Error(1)
}

func (m *mockUserRepo) FindByID(id uint64) (*model.User, error) {
	args := m.Called(id)
	u, _ := args.Get(0).(*model.User)
	return u, args.Error(1)
}

func (m *mockUserRepo) Update(user *model.User) error {
	args := m.Called(user)
	return args.Error(0)
}

type mockTokenRepo struct{ mock.Mock }

func (m *mockTokenRepo) Create(token *model.RefreshToken) error {
	args := m.Called(token)
	return args.Error(0)
}

func (m *mockTokenRepo) FindByHash(hash string) (*model.RefreshToken, error) {
	args := m.Called(hash)
	t, _ := args.Get(0).(*model.RefreshToken)
	return t, args.Error(1)
}

func (m *mockTokenRepo) Delete(id uint64) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockTokenRepo) DeleteByUserID(userID uint64) error {
	args := m.Called(userID)
	return args.Error(0)
}

// ── Test helpers ──────────────────────────────────────────────────────────────

func newTestCfg() *config.Config {
	return &config.Config{
		JWT: config.JWTConfig{
			AccessSecret:  "test-access-secret-at-least-32ch",
			RefreshSecret: "test-refresh-secret-at-least-32c",
			AccessTTL:     15,
			RefreshTTL:    7,
		},
	}
}

// notFound mimics the wrapped gorm.ErrRecordNotFound that the real repository returns.
func notFound() error {
	return fmt.Errorf("repo layer: %w", gorm.ErrRecordNotFound)
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	uRepo := &mockUserRepo{}
	tRepo := &mockTokenRepo{}
	svc := NewAuthService(uRepo, tRepo, newTestCfg())

	uRepo.On("FindByEmail", "alice@example.com").Return((*model.User)(nil), notFound()).Once()
	uRepo.On("FindByPhone", "13800138000").Return((*model.User)(nil), notFound()).Once()
	uRepo.On("FindByUsername", "alice").Return((*model.User)(nil), notFound()).Once()
	uRepo.On("Create", mock.Anything).Return(nil).Once()
	tRepo.On("Create", mock.Anything).Return(nil).Once()

	user, access, refresh, err := svc.Register(RegisterReq{
		Username: "alice",
		Email:    "alice@example.com",
		Phone:    "13800138000",
		Password: "secret123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "alice", user.Username)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)
	uRepo.AssertExpectations(t)
	tRepo.AssertExpectations(t)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	uRepo := &mockUserRepo{}
	svc := NewAuthService(uRepo, &mockTokenRepo{}, newTestCfg())

	uRepo.On("FindByEmail", "dupe@example.com").Return(&model.User{ID: 1}, nil).Once()

	_, _, _, err := svc.Register(RegisterReq{
		Username: "newuser",
		Email:    "dupe@example.com",
		Password: "pass",
	})

	assert.EqualError(t, err, "email already registered")
	uRepo.AssertExpectations(t)
}

func TestRegister_DuplicatePhone(t *testing.T) {
	uRepo := &mockUserRepo{}
	svc := NewAuthService(uRepo, &mockTokenRepo{}, newTestCfg())

	uRepo.On("FindByEmail", "new@example.com").Return((*model.User)(nil), notFound()).Once()
	uRepo.On("FindByPhone", "13900000000").Return(&model.User{ID: 2}, nil).Once()

	_, _, _, err := svc.Register(RegisterReq{
		Username: "newuser",
		Email:    "new@example.com",
		Phone:    "13900000000",
		Password: "pass",
	})

	assert.EqualError(t, err, "phone already registered")
	uRepo.AssertExpectations(t)
}

func TestRegister_DuplicateUsername(t *testing.T) {
	uRepo := &mockUserRepo{}
	svc := NewAuthService(uRepo, &mockTokenRepo{}, newTestCfg())

	uRepo.On("FindByEmail", "unique@example.com").Return((*model.User)(nil), notFound()).Once()
	uRepo.On("FindByPhone", "13700000001").Return((*model.User)(nil), notFound()).Once()
	uRepo.On("FindByUsername", "taken").Return(&model.User{ID: 3}, nil).Once()

	_, _, _, err := svc.Register(RegisterReq{
		Username: "taken",
		Email:    "unique@example.com",
		Phone:    "13700000001",
		Password: "pass",
	})

	assert.EqualError(t, err, "username already taken")
	uRepo.AssertExpectations(t)
}

// ── LoginPassword ─────────────────────────────────────────────────────────────

func TestLoginPassword_Success(t *testing.T) {
	uRepo := &mockUserRepo{}
	tRepo := &mockTokenRepo{}
	svc := NewAuthService(uRepo, tRepo, newTestCfg())

	hash, err := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)
	assert.NoError(t, err)

	user := &model.User{
		ID:           10,
		Email:        "dave@example.com",
		Username:     "dave",
		Role:         "user",
		Status:       "active",
		PasswordHash: string(hash),
	}
	uRepo.On("FindByEmail", "dave@example.com").Return(user, nil).Once()
	tRepo.On("Create", mock.Anything).Return(nil).Once()

	gotUser, access, refresh, err := svc.LoginPassword("dave@example.com", "correct")

	assert.NoError(t, err)
	assert.Equal(t, uint64(10), gotUser.ID)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)
	uRepo.AssertExpectations(t)
	tRepo.AssertExpectations(t)
}

func TestLoginPassword_UserNotFound(t *testing.T) {
	uRepo := &mockUserRepo{}
	svc := NewAuthService(uRepo, &mockTokenRepo{}, newTestCfg())

	uRepo.On("FindByEmail", "ghost@example.com").Return((*model.User)(nil), notFound()).Once()

	_, _, _, err := svc.LoginPassword("ghost@example.com", "anypass")

	assert.EqualError(t, err, "invalid credentials")
	uRepo.AssertExpectations(t)
}

func TestLoginPassword_WrongPassword(t *testing.T) {
	uRepo := &mockUserRepo{}
	svc := NewAuthService(uRepo, &mockTokenRepo{}, newTestCfg())

	hash, _ := bcrypt.GenerateFromPassword([]byte("realpassword"), bcrypt.MinCost)
	user := &model.User{
		ID:           11,
		Email:        "eve@example.com",
		Status:       "active",
		PasswordHash: string(hash),
	}
	uRepo.On("FindByEmail", "eve@example.com").Return(user, nil).Once()

	_, _, _, err := svc.LoginPassword("eve@example.com", "wrongpassword")

	assert.EqualError(t, err, "invalid credentials")
	uRepo.AssertExpectations(t)
}

// ── RefreshToken ──────────────────────────────────────────────────────────────

func TestRefreshToken_Success(t *testing.T) {
	uRepo := &mockUserRepo{}
	tRepo := &mockTokenRepo{}
	cfg := newTestCfg()
	svc := NewAuthService(uRepo, tRepo, cfg)

	refreshTok, err := jwtpkg.GenerateRefreshToken("42", cfg.JWT.RefreshSecret, cfg.JWT.RefreshTTL)
	assert.NoError(t, err)

	tokHash := hashToken(refreshTok)
	tokenRecord := &model.RefreshToken{
		ID:        55,
		UserID:    42,
		TokenHash: tokHash,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	user := &model.User{ID: 42, Username: "frank", Role: "user", Status: "active"}

	tRepo.On("FindByHash", tokHash).Return(tokenRecord, nil).Once()
	uRepo.On("FindByID", uint64(42)).Return(user, nil).Once()
	tRepo.On("Delete", uint64(55)).Return(nil).Once()
	tRepo.On("Create", mock.Anything).Return(nil).Once()

	newAccess, newRefresh, err := svc.RefreshToken(refreshTok)

	assert.NoError(t, err)
	assert.NotEmpty(t, newAccess)
	assert.NotEmpty(t, newRefresh)
	uRepo.AssertExpectations(t)
	tRepo.AssertExpectations(t)
}

func TestRefreshToken_RevokedToken(t *testing.T) {
	tRepo := &mockTokenRepo{}
	cfg := newTestCfg()
	svc := NewAuthService(&mockUserRepo{}, tRepo, cfg)

	refreshTok, err := jwtpkg.GenerateRefreshToken("99", cfg.JWT.RefreshSecret, cfg.JWT.RefreshTTL)
	assert.NoError(t, err)

	tokHash := hashToken(refreshTok)
	tRepo.On("FindByHash", tokHash).Return((*model.RefreshToken)(nil), errors.New("not found")).Once()

	_, _, err = svc.RefreshToken(refreshTok)

	assert.EqualError(t, err, "refresh token revoked or not found")
	tRepo.AssertExpectations(t)
}

// ── Logout ────────────────────────────────────────────────────────────────────

func TestLogout_Success(t *testing.T) {
	tRepo := &mockTokenRepo{}
	svc := NewAuthService(&mockUserRepo{}, tRepo, newTestCfg())

	tok := "opaque-refresh-token-value"
	tokHash := hashToken(tok)
	tokenRecord := &model.RefreshToken{ID: 77}

	tRepo.On("FindByHash", tokHash).Return(tokenRecord, nil).Once()
	tRepo.On("Delete", uint64(77)).Return(nil).Once()

	assert.NoError(t, svc.Logout(tok))
	tRepo.AssertExpectations(t)
}

func TestLogout_AlreadyRevoked(t *testing.T) {
	tRepo := &mockTokenRepo{}
	svc := NewAuthService(&mockUserRepo{}, tRepo, newTestCfg())

	tok := "already-gone-token"
	tokHash := hashToken(tok)
	tRepo.On("FindByHash", tokHash).Return((*model.RefreshToken)(nil), errors.New("not found")).Once()

	// Token not found → treated as already logged out; must not return error.
	assert.NoError(t, svc.Logout(tok))
	tRepo.AssertExpectations(t)
}

// ── GetUser ───────────────────────────────────────────────────────────────────

func TestGetUser_Success(t *testing.T) {
	uRepo := &mockUserRepo{}
	svc := NewAuthService(uRepo, &mockTokenRepo{}, newTestCfg())

	expected := &model.User{ID: 5, Username: "grace", Role: "user"}
	uRepo.On("FindByID", uint64(5)).Return(expected, nil).Once()

	user, err := svc.GetUser(5)

	assert.NoError(t, err)
	assert.Equal(t, uint64(5), user.ID)
	assert.Equal(t, "grace", user.Username)
	uRepo.AssertExpectations(t)
}

func TestGetUser_NotFound(t *testing.T) {
	uRepo := &mockUserRepo{}
	svc := NewAuthService(uRepo, &mockTokenRepo{}, newTestCfg())

	uRepo.On("FindByID", uint64(999)).Return((*model.User)(nil), notFound()).Once()

	user, err := svc.GetUser(999)

	assert.Nil(t, user)
	assert.Error(t, err)
	// Error chain must unwrap to gorm.ErrRecordNotFound.
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
	uRepo.AssertExpectations(t)
}
