// 本文件封装了 AES-256-GCM 对称加密的加密与解密功能，用于保护敏感数据。
// 核心知识点：[]byte 与 string 互转、make 创建切片（slice）、切片切割（slicing）、
// 多返回值、错误包装（%w）、len() 内置函数。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Encrypt 使用 AES-256-GCM 加密明文，返回 base64(nonce + ciphertext)
// plaintext, key string 是参数简写，两个参数都是 string 类型。
func Encrypt(plaintext, key string) (string, error) {
	// []byte(key) 将 string 转为字节切片。Go 中 string 是不可变的，[]byte 是可变的。
	// 加密库需要操作原始字节，所以必须转换。
	keyBytes := []byte(key)
	// len() 是内置函数，返回切片/字符串/map 的长度。AES-256 要求密钥正好 32 字节。
	if len(keyBytes) != 32 {
		return "", fmt.Errorf("encryption key must be 32 bytes, got %d", len(keyBytes))
	}

	// Go 的错误处理模式：几乎每个可能失败的调用都返回 (结果, error)，
	// 用 if err != nil 判断是否出错，出错就立即返回——这是 Go 最常见的代码模式。
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	// make([]byte, 长度) 创建一个指定长度的字节切片（slice），所有元素初始化为 0。
	// 切片是 Go 中最常用的动态数组，底层是 [指针, 长度, 容量] 三元组。
	nonce := make([]byte, gcm.NonceSize())
	// io.ReadFull 将随机字节填满 nonce 切片；_ 用来丢弃不需要的返回值（读取的字节数）。
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// Seal 将 nonce 和密文拼接在一起。[]byte(plaintext) 又一次 string → []byte 转换。
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	// base64 编码后返回字符串，便于在 JSON/HTTP 中传输二进制数据。
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密 Encrypt 生成的 base64 密文
func Decrypt(ciphertext, key string) (string, error) {
	keyBytes := []byte(key)
	if len(keyBytes) != 32 {
		return "", fmt.Errorf("encryption key must be 32 bytes, got %d", len(keyBytes))
	}

	// base64 解码：将字符串还原为原始字节切片 data。
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// data[:nonceSize] 和 data[nonceSize:] 是切片切割（slicing）语法。
	// s[a:b] 取索引 a 到 b-1 的元素；s[:n] 等价于 s[0:n]；s[n:] 等价于 s[n:len(s)]。
	// 这里把加密时拼在一起的 nonce 和密文重新分开。
	nonce, encryptedData := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	// string(plaintext) 将 []byte 转回 string。这和 []byte(str) 是一对互逆操作。
	return string(plaintext), nil
}
