// 本文件实现基于 Redis 的请求限流中间件：在固定时间窗口内限制每个客户端的请求次数。
// 涉及 Go 基础知识：闭包、time.Duration 类型、fmt.Sprintf 格式化、int64 类型转换、
// 函数作为参数（高阶函数）、context 包。
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/autovideo/auth-service/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimit 基于 Redis 的令牌桶限流中间件，按客户端 IP 限流
// requests: 窗口期内允许的最大请求数
// window: 时间窗口
//
// 【闭包 & 多参数】函数接收 Redis 客户端、请求上限、时间窗口三个参数，
// 返回 gin.HandlerFunc；内层匿名函数通过闭包捕获这三个外部变量。
func RateLimit(rdb *redis.Client, requests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		// 【fmt.Sprintf】格式化字符串，%s 是字符串占位符，%d 是整数占位符。
		key := fmt.Sprintf("rate_limit:%s", ip)

		// 【context.Background()】创建一个空的上下文（Context），通常用于没有超时/取消需求的场景。
		// Context 是 Go 中管理请求生命周期（超时、取消、传值）的标准机制。
		ctx := context.Background()

		// 使用 Redis INCR + EXPIRE 实现固定窗口计数器
		// .Incr() 让 key 的值 +1 并返回新值；.Result() 拆出 (int64, error) 两个返回值。
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Redis 不可用时放行，避免影响业务
			c.Next()
			return
		}

		// 第一次请求时设置过期时间
		if count == 1 {
			// 【time.Duration】表示一段时间长度（如 1*time.Minute）。
			// Expire 设置 key 的 TTL（生存时间），到期后 Redis 自动删除该 key。
			rdb.Expire(ctx, key, window)
		}

		// 【int64(requests)】类型转换：Go 是强类型语言，int 和 int64 不能直接比较，
		// 需要显式转换后才能使用 > 运算符。
		if count > int64(requests) {
			// 【fmt.Sprintf("%d", ...)】将整数格式化为字符串
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requests))
			c.Header("X-RateLimit-Remaining", "0")
			response.TooManyRequests(c)
			return
		}

		remaining := int64(requests) - count
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		// 【window.String()】time.Duration 自带 String() 方法，输出人类可读格式如 "1m0s"
		c.Header("X-RateLimit-Window", window.String())

		c.Next()
	}
}

// RateLimitByKey 按自定义 key 限流（如 user_id、api 路径等）
//
// 【函数作为参数（高阶函数）】keyFunc 的类型是 func(c *gin.Context) string，
// 即"接收 *gin.Context、返回 string 的函数"。调用者可以传入任意符合该签名的函数，
// 从而灵活决定限流维度（按用户、按路径等）——这就是 Go 的"函数是一等公民"特性。
func RateLimitByKey(rdb *redis.Client, keyFunc func(c *gin.Context) string, requests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// keyFunc(c) 调用传入的函数，动态生成限流 key
		key := fmt.Sprintf("rate_limit:%s", keyFunc(c))
		ctx := context.Background()

		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if count == 1 {
			rdb.Expire(ctx, key, window)
		}
		if count > int64(requests) {
			// 【http.StatusTooManyRequests】标准库定义的 HTTP 状态码常量，值为 429
			// c.AbortWithStatus 终止请求链并返回指定状态码
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	}
}
