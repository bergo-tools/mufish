// Package auth 提供口令散列与会话令牌生成。
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	iterations = 120000
	keyLen     = 32
	saltLen    = 16
	algo       = "sha256"
)

// HashPassword 使用 PBKDF2-SHA256 生成口令散列，格式：
// pbkdf2$sha256$<iter>$<salt-b64>$<hash-b64>
func HashPassword(plain string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("生成随机盐失败: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, plain, salt, iterations, keyLen)
	if err != nil {
		return "", fmt.Errorf("计算口令散列失败: %w", err)
	}
	return fmt.Sprintf("pbkdf2$%s$%d$%s$%s", algo,
		iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword 校验口令。
func VerifyPassword(plain, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "pbkdf2" || parts[1] != algo {
		return false
	}
	iter, err := strconv.Atoi(parts[2])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, plain, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// NewToken 生成随机会话令牌。
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成会话令牌失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// NewCode 生成指定字节数的随机十六进制串（用于编号后缀等）。
func NewCode(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(b)
}

// RandomPassword 生成一个易读的初始口令。
func RandomPassword() string {
	const chars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "mufish123"
	}
	out := make([]byte, len(b))
	for i, v := range b {
		out[i] = chars[int(v)%len(chars)]
	}
	return string(out)
}
