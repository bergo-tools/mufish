package model

// 权限目录：系统中所有可分配的权限点。
// 采用「模块:动作」的编码方式，超级管理员拥有通配符 "*"。

// Perm 一个权限点。
type Perm struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// PermGroup 权限分组（用于角色配置界面展示）。
type PermGroup struct {
	Module string `json:"module"`
	Name   string `json:"name"`
	Perms  []Perm `json:"perms"`
}

// SuperPerm 超级权限通配符。
const SuperPerm = "*"

// PermCatalog 全部权限点。
var PermCatalog = []PermGroup{
	{Module: "dashboard", Name: "首页看板", Perms: []Perm{
		{"dashboard:view", "查看看板"},
	}},
	{Module: "product", Name: "商品资料", Perms: []Perm{
		{"product:view", "查看商品"},
		{"product:create", "新增商品"},
		{"product:update", "修改商品"},
		{"product:delete", "删除商品"},
		{"product:import", "导入商品"},
	}},
	{Module: "category", Name: "商品分类", Perms: []Perm{
		{"category:view", "查看分类"},
		{"category:manage", "维护分类"},
	}},
	{Module: "partner", Name: "往来单位", Perms: []Perm{
		{"partner:view", "查看往来单位"},
		{"partner:create", "新增往来单位"},
		{"partner:update", "修改往来单位"},
		{"partner:delete", "删除往来单位"},
	}},
	{Module: "warehouse", Name: "仓库管理", Perms: []Perm{
		{"warehouse:view", "查看仓库"},
		{"warehouse:manage", "维护仓库"},
	}},
	{Module: "purchase", Name: "采购入库", Perms: []Perm{
		{"purchase:view", "查看采购单"},
		{"purchase:create", "新建采购单"},
		{"purchase:update", "修改采购单"},
		{"purchase:delete", "删除采购单"},
		{"purchase:confirm", "采购入库确认"},
		{"purchase:void", "作废采购单"},
	}},
	{Module: "sales", Name: "销售出库", Perms: []Perm{
		{"sales:view", "查看销售单"},
		{"sales:create", "新建销售单"},
		{"sales:update", "修改销售单"},
		{"sales:delete", "删除销售单"},
		{"sales:confirm", "销售出库确认"},
		{"sales:void", "作废销售单"},
	}},
	{Module: "stock", Name: "库存管理", Perms: []Perm{
		{"stock:view", "查看库存"},
		{"stock:ledger", "查看库存流水"},
		{"stock:take", "库存盘点"},
		{"stock:transfer", "库存调拨"},
	}},
	{Module: "payment", Name: "收付款", Perms: []Perm{
		{"payment:view", "查看收付款单"},
		{"payment:create", "新建收付款单"},
		{"payment:confirm", "收付款确认"},
		{"payment:void", "作废收付款单"},
	}},
	{Module: "account", Name: "资金账户", Perms: []Perm{
		{"account:view", "查看账户"},
		{"account:manage", "维护账户"},
	}},
	{Module: "report", Name: "统计报表", Perms: []Perm{
		{"report:view", "查看报表"},
		{"report:export", "导出报表"},
	}},
	{Module: "system", Name: "系统管理", Perms: []Perm{
		{"user:view", "查看用户"},
		{"user:manage", "维护用户"},
		{"role:view", "查看角色"},
		{"role:manage", "维护角色"},
		{"setting:view", "查看参数"},
		{"setting:manage", "修改参数"},
		{"log:view", "查看操作日志"},
		{"backup:run", "备份 / 恢复数据"},
	}},
}

// AllPermCodes 返回全部权限编码。
func AllPermCodes() []string {
	out := make([]string, 0, 64)
	for _, g := range PermCatalog {
		for _, p := range g.Perms {
			out = append(out, p.Code)
		}
	}
	return out
}

// validPerms 全部合法权限编码集合。
var validPerms = func() map[string]bool {
	m := map[string]bool{SuperPerm: true}
	for _, g := range PermCatalog {
		for _, p := range g.Perms {
			m[p.Code] = true
		}
	}
	return m
}()

// IsValidPerm 判断权限编码是否合法。
func IsValidPerm(code string) bool { return validPerms[code] }

// PermSet 权限集合，便于判断。
type PermSet map[string]bool

// NewPermSet 构造权限集合。
func NewPermSet(codes []string) PermSet {
	s := make(PermSet, len(codes)+1)
	for _, c := range codes {
		s[c] = true
	}
	return s
}

// Has 判断是否拥有某个权限（支持 "*" 通配）。
func (s PermSet) Has(code string) bool {
	if s == nil {
		return false
	}
	if s[SuperPerm] {
		return true
	}
	return s[code]
}

// HasAny 判断是否拥有其中任意一个权限。
func (s PermSet) HasAny(codes ...string) bool {
	for _, c := range codes {
		if s.Has(c) {
			return true
		}
	}
	return false
}

// List 返回排序后的权限列表。
func (s PermSet) List() []string {
	out := make([]string, 0, len(s))
	for c := range s {
		out = append(out, c)
	}
	return out
}
