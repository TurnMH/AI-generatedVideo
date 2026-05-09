// 本文件的作用：定义数据库模型（Model）。
// 每个 struct 对应数据库中的一张表，GORM（Go 的 ORM 库）会根据这些结构体
// 自动建表、查询、插入等。字段上的 struct tag 同时控制数据库列属性和 JSON 输出。

package model

import (
	"time" // 标准库，提供 time.Time 类型用于表示时间

	"gorm.io/gorm" // GORM —— Go 最流行的 ORM 框架，用结构体操作数据库
)

// User —— 用户表模型。
// 一个 struct 里可以同时有多种 tag，用空格分隔：
//   - gorm tag：控制数据库行为（主键、索引、大小、默认值等）
//   - json tag：控制 JSON 序列化（API 输出时的字段名）
//     json:"-" 表示该字段不会出现在 JSON 输出中（隐藏敏感数据）
type User struct {
	ID           uint64         `gorm:"primaryKey;autoIncrement" json:"id"`          // uint64 = 无符号 64 位整数
	Username     string         `gorm:"uniqueIndex;size:64;not null" json:"username"` // uniqueIndex = 唯一索引
	Email        string         `gorm:"uniqueIndex;size:128" json:"email"`
	Phone        string         `gorm:"size:20;index" json:"phone"`                  // index = 普通索引，加速查询
	PasswordHash string         `gorm:"size:256" json:"-"`                           // json:"-" → 密码哈希不暴露给前端
	AvatarURL    string         `gorm:"type:text" json:"avatar_url"`
	Role         string         `gorm:"size:32;default:user;not null" json:"role"`   // default:user → 数据库默认值
	Status       string         `gorm:"size:16;default:active;not null" json:"status"`
	CreatedAt    time.Time      `json:"created_at"`                                  // time.Time → Go 中表示日期时间的类型
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`                              // gorm.DeletedAt → 软删除支持，删除只打标记不真删
}

// OAuthAccount —— 第三方登录账号表（如 Google、GitHub 登录）。
// User 字段是关联关系：gorm:"foreignKey:UserID" 告诉 GORM 用 UserID 做外键关联 User 表。
type OAuthAccount struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	UserID      uint64 `gorm:"not null;index"`
	Provider    string `gorm:"size:32;not null"`  // 第三方平台名称，如 "google"
	ProviderID  string `gorm:"size:128;not null"` // 用户在第三方平台的唯一 ID
	AccessToken string `gorm:"type:text"`
	User        User   `gorm:"foreignKey:UserID"` // 结构体嵌套 → 表示一对多关系中的"多"端
}

// RefreshToken —— 刷新令牌表，用于实现 JWT 的 token 刷新机制。
type RefreshToken struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	UserID     uint64    `gorm:"not null;index"`
	TokenHash  string    `gorm:"size:256;uniqueIndex;not null"` // 存哈希值而非原文，安全考虑
	ExpiresAt  time.Time `gorm:"not null"`
	DeviceInfo string    `gorm:"type:text"`
	CreatedAt  time.Time
	User       User `gorm:"foreignKey:UserID"`
}

// UserAPIKey —— 用户自己的 API 密钥（如 OpenAI key），加密存储。
// bool 是布尔类型，只有 true / false 两个值。
type UserAPIKey struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint64    `gorm:"not null;index" json:"user_id"`
	Provider     string    `gorm:"size:64;not null" json:"provider"`
	KeyAlias     string    `gorm:"size:128" json:"key_alias"`
	EncryptedKey string    `gorm:"type:text;not null" json:"-"` // json:"-" → 加密后的密钥也不暴露
	BaseURL      string    `gorm:"type:text;default:''" json:"base_url"`
	ModelScope   string    `gorm:"type:text;default:''" json:"model_scope"`
	IsSystem     bool      `gorm:"default:false;not null" json:"is_system"` // bool 类型，默认 false
	IsActive     bool      `gorm:"default:true;not null" json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	User         User      `gorm:"foreignKey:UserID" json:"-"`
}

// SystemAPIKey —— 系统级 API 密钥，由管理员配置，所有用户共享。
type SystemAPIKey struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Provider   string    `gorm:"size:64;not null" json:"provider"`
	KeyAlias   string    `gorm:"size:128;not null" json:"key_alias"`
	PlainKey   string    `gorm:"type:text;not null" json:"-"` // 系统密钥明文存储但不返回前端
	BaseURL    string    `gorm:"type:text;default:''" json:"base_url"`
	ModelScope string    `gorm:"type:text;default:''" json:"model_scope"`
	IsActive   bool      `gorm:"default:true;not null" json:"is_active"`
	Status     string    `gorm:"size:16;default:'active'" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// Permission —— 权限表，每条记录代表一个可授予的权限。
type Permission struct {
	ID   uint64 `gorm:"primaryKey;autoIncrement"`
	Code string `gorm:"size:128;uniqueIndex;not null"` // 权限标识码，如 "user:create"
	Desc string `gorm:"type:text"`                     // 权限描述
}

// RolePermission —— 角色-权限关联表（多对多关系）。
// 两个字段都标记为 primaryKey → 组合主键（一个角色+一个权限唯一确定一条记录）。
type RolePermission struct {
	Role       string `gorm:"size:32;not null;primaryKey"`
	Permission string `gorm:"size:128;not null;primaryKey"`
}
