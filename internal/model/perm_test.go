package model

import (
	"strings"
	"testing"
)

func TestPermCatalog(t *testing.T) {
	codes := AllPermCodes()
	if len(codes) != 46 {
		t.Fatalf("权限点数量期望 46，实际 %d", len(codes))
	}
	seen := map[string]bool{}
	for _, g := range PermCatalog {
		if g.Module == "" || g.Name == "" || len(g.Perms) == 0 {
			t.Fatalf("权限分组不完整: %+v", g)
		}
		for _, p := range g.Perms {
			if seen[p.Code] {
				t.Fatalf("权限编码重复: %s", p.Code)
			}
			seen[p.Code] = true
			if !strings.Contains(p.Code, ":") {
				t.Fatalf("权限编码应包含模块前缀: %s", p.Code)
			}
			if !IsValidPerm(p.Code) {
				t.Fatalf("目录中的权限应合法: %s", p.Code)
			}
		}
	}
	if len(seen) != len(codes) {
		t.Fatalf("AllPermCodes 与目录不一致: %d / %d", len(codes), len(seen))
	}
	if !IsValidPerm(SuperPerm) {
		t.Fatalf("超级权限通配符应合法")
	}
	if IsValidPerm("not:a:perm") {
		t.Fatalf("不存在的权限不应合法")
	}
}

func TestPermSet(t *testing.T) {
	var nilSet PermSet
	if nilSet.Has("product:view") {
		t.Fatalf("nil 权限集合不应拥有任何权限")
	}
	s := NewPermSet([]string{"product:view", "stock:view"})
	if !s.Has("product:view") || !s.Has("stock:view") {
		t.Fatalf("权限集合缺少已添加权限")
	}
	if s.Has("product:delete") {
		t.Fatalf("权限集合不应拥有未添加权限")
	}
	if !s.HasAny("product:delete", "stock:view") {
		t.Fatalf("HasAny 应命中已有权限")
	}
	if s.HasAny("product:delete", "sales:view") {
		t.Fatalf("HasAny 不应命中不存在的权限")
	}
	if len(s.List()) != 2 {
		t.Fatalf("权限列表长度期望 2，实际 %d", len(s.List()))
	}
	super := NewPermSet([]string{SuperPerm})
	if !super.Has("任意权限") {
		t.Fatalf("超级权限应通配所有权限")
	}
}
