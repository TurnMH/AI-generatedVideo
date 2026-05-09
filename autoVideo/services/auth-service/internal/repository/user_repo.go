// 本文件是用户数据仓库层（Repository），负责对数据库中的用户表进行增删改查。
// 核心知识点：interface 接口定义、指针接收器（pointer receiver）、错误包装（%w）、
// 结构体指针（*model.User）、工厂函数模式（NewXxx）。
package repository

import (
	"fmt"

	"github.com/autovideo/auth-service/internal/model"
	"gorm.io/gorm"
)

// UserRepository 是一个接口（interface）。
// 接口定义了一组方法签名，任何结构体只要实现了这些方法，就自动满足该接口——不需要显式声明。
// 这叫做 Go 的"隐式接口实现"，是 Go 与 Java/C# 最大的区别之一。
// *model.User 表示"指向 model.User 结构体的指针"，传指针可以避免拷贝整个结构体，提高性能。
// 返回值 error 是 Go 内置的错误接口，函数通过返回 nil 表示成功、非 nil 表示失败。
type UserRepository interface {
	Create(user *model.User) error
	FindByEmail(email string) (*model.User, error)
	FindByPhone(phone string) (*model.User, error)
	FindByUsername(username string) (*model.User, error)
	FindByID(id uint64) (*model.User, error)
	Update(user *model.User) error
}

// userRepository 是接口的具体实现，注意首字母小写——Go 中小写开头表示包内私有，外部无法直接使用。
// 字段 db 的类型是 *gorm.DB（指针），这样所有方法共享同一个数据库连接。
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 是"工厂函数"，返回值类型是接口 UserRepository 而非具体结构体。
// &userRepository{} 用 & 取地址，返回结构体指针；这样才能匹配下面的指针接收器方法。
// 这种"返回接口"的写法是 Go 的惯用模式，方便以后替换实现（比如换成 mock 做测试）。
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// (r *userRepository) 叫做"指针接收器"（pointer receiver）。
// r 就像其他语言的 this/self，*userRepository 表示 r 是指针，可以读写结构体字段。
// 如果用值接收器 (r userRepository)，r 会是副本，修改不会影响原对象。
// fmt.Errorf("...: %w", err) 中的 %w 是"错误包装"，把原始 err 嵌套进新错误中，
// 调用方可以用 errors.Is / errors.As 解包查看底层错误，方便调试。
func (r *userRepository) Create(user *model.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// var user model.User 声明一个零值结构体变量（所有字段为默认值）。
// &user 取地址传给 GORM，让 GORM 把查询结果直接写入 user 的内存地址。
// 返回值 (*model.User, error) 是 Go 的多返回值特性，常见的"值+错误"模式。
func (r *userRepository) FindByEmail(email string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return &user, nil
}

func (r *userRepository) FindByPhone(phone string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("phone = ?", phone).First(&user).Error; err != nil {
		return nil, fmt.Errorf("find user by phone: %w", err)
	}
	return &user, nil
}

func (r *userRepository) FindByUsername(username string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, fmt.Errorf("find user by username: %w", err)
	}
	return &user, nil
}

// uint64 是无符号 64 位整数，适合做数据库主键 ID（不会为负数）。
func (r *userRepository) FindByID(id uint64) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return &user, nil
}

func (r *userRepository) Update(user *model.User) error {
	if err := r.db.Save(user).Error; err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}
