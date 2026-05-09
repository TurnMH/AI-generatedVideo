// 本文件实现请求日志中间件：记录每次 HTTP 请求的耗时、状态码、IP 等信息。
// 涉及 Go 基础知识：time 包（计时）、切片（slice）、append、可变参数（...）、闭包。
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Logger 返回基于 Zap 的请求日志中间件
//
// 【闭包】Logger 接收一个 *zap.Logger 指针，返回 gin.HandlerFunc。
// 返回的匿名函数捕获了外部变量 logger，每次请求都复用同一个 logger 实例。
func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 【time.Now()】获取当前时刻，返回 time.Time 类型（精确到纳秒）
		start := time.Now()
		// c.Request 是标准库 *http.Request；URL.Path 获取请求路径，如 "/api/login"
		path := c.Request.URL.Path
		// RawQuery 获取 URL 中 ? 后面的查询参数原始字符串，如 "page=1&size=10"
		query := c.Request.URL.RawQuery

		// c.Next() 先让后续处理函数执行完毕，再回到这里继续——这就是"后置中间件"的写法
		c.Next()

		// 【time.Since(start)】计算从 start 到现在经过的时间，返回 time.Duration 类型
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		errMsg := c.Errors.ByType(gin.ErrorTypePrivate).String()

		// 【切片字面量】[]zap.Field{...} 创建一个 zap.Field 类型的切片（slice）。
		// 切片是 Go 中最常用的动态数组，长度可变。
		fields := []zap.Field{
			zap.Int("status", statusCode),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", clientIP),
			// zap.Duration 专门用于记录时间间隔，会自动格式化为人类可读的字符串
			zap.Duration("latency", latency),
			zap.String("user_agent", c.Request.UserAgent()),
		}

		if errMsg != "" {
			// 【append】内置函数，向切片末尾追加元素，返回新切片。
			// 注意：append 可能在底层扩容，因此必须用返回值重新赋值。
			fields = append(fields, zap.String("error", errMsg))
		}

		// 【可变参数 ...】fields... 把切片展开为多个独立参数传入函数。
		// 等价于 logger.Error("request", fields[0], fields[1], fields[2], ...)
		if statusCode >= 500 {
			logger.Error("request", fields...)
		} else if statusCode >= 400 {
			logger.Warn("request", fields...)
		} else {
			logger.Info("request", fields...)
		}
	}
}
