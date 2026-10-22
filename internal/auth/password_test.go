package auth

import (
	"regexp"
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	const plain = "S3cret-口令"
	hash, err := HashPassword(plain)
	if err != nil {
		t.Fatalf("生成口令散列失败: %v", err)
	}
	if strings.Contains(hash, plain) {
		t.Fatalf("散列中不应包含明文口令")
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 5 || parts[0] != "pbkdf2" || parts[1] != "sha256" {
		t.Fatalf("散列格式异常: %s", hash)
	}
	if !VerifyPassword(plain, hash) {
		t.Fatalf("正确口令应校验通过")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatalf("错误口令不应校验通过")
	}

	hash2, err := HashPassword(plain)
	if err != nil {
		t.Fatalf("第二次生成口令散列失败: %v", err)
	}
	if hash == hash2 {
		t.Fatalf("相同口令应使用不同盐生成不同散列")
	}
	if !VerifyPassword(plain, hash2) {
		t.Fatalf("第二个散列也应校验通过")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	cases := []string{
		"",
		"plaintext",
		"pbkdf2$sha256$120000$salt",
		"pbkdf2$sha512$120000$c2FsdA$aGFzaA",
		"pbkdf2$sha256$abc$c2FsdA$aGFzaA",
		"pbkdf2$sha256$0$c2FsdA$aGFzaA",
		"pbkdf2$sha256$120000$!!!$aGFzaA",
		"pbkdf2$sha256$120000$c2FsdA$!!!",
	}
	for _, h := range cases {
		if VerifyPassword("whatever", h) {
			t.Fatalf("非法散列不应校验通过: %q", h)
		}
	}
}

func TestNewTokenAndCode(t *testing.T) {
	t1, err := NewToken()
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}
	t2, err := NewToken()
	if err != nil {
		t.Fatalf("生成第二个令牌失败: %v", err)
	}
	if len(t1) != 64 || !regexp.MustCompile("^[0-9a-f]+$").MatchString(t1) {
		t.Fatalf("令牌应为 64 位十六进制字符串，实际 %q", t1)
	}
	if t1 == t2 {
		t.Fatalf("两次生成的令牌不应相同")
	}
	if got := NewCode(4); len(got) != 8 {
		t.Fatalf("NewCode(4) 应返回 8 位十六进制字符串，实际 %q", got)
	}
	if got := NewCode(0); got != "" {
		t.Fatalf("NewCode(0) 应返回空字符串，实际 %q", got)
	}
}

func TestRandomPassword(t *testing.T) {
	p := RandomPassword()
	if len(p) != 10 {
		t.Fatalf("随机口令长度期望 10，实际 %d（%q）", len(p), p)
	}
	const allowed = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"
	for _, r := range p {
		if !strings.ContainsRune(allowed, r) {
			t.Fatalf("随机口令包含非法字符 %q", r)
		}
	}
	if RandomPassword() == p {
		t.Fatalf("两次生成的随机口令不应相同")
	}
}
