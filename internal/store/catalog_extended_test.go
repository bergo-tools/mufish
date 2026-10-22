package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"mufish/internal/model"
)

func TestProductCRUDAndQueries(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	root, err := st.SaveCategory(ctx, 0, CategoryInput{Name: "扩展分类"})
	if err != nil {
		t.Fatalf("创建分类失败: %v", err)
	}
	child, err := st.SaveCategory(ctx, 0, CategoryInput{Name: "扩展子分类", ParentID: root})
	if err != nil {
		t.Fatalf("创建子分类失败: %v", err)
	}

	pid, err := st.SaveProduct(ctx, 0, ProductInput{
		Code: "EXT-P1", Name: "扩展测试商品", Spec: "规格A", Unit: "个",
		CategoryID: child, PurchasePrice: model.PriceFrom(10), SalePrice: model.PriceFrom(20),
		SafetyStock: model.QtyFrom(5), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}

	if _, err := st.SaveProduct(ctx, 0, ProductInput{Code: "EXT-P1", Name: "重复编码", Status: 1}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("重复商品编码应返回 ErrDuplicate，实际 %v", err)
	}
	if _, err := st.SaveProduct(ctx, 999999, ProductInput{Code: "EXT-NONE", Name: "不存在"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("修改不存在的商品应返回 ErrNotFound，实际 %v", err)
	}

	p, err := st.GetProduct(ctx, pid)
	if err != nil {
		t.Fatalf("读取商品失败: %v", err)
	}
	if p.Name != "扩展测试商品" || p.CategoryName != "扩展子分类" {
		t.Fatalf("商品读取结果不正确: %+v", p)
	}
	if _, err := st.GetProduct(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("读取不存在的商品应返回 ErrNotFound，实际 %v", err)
	}

	page, err := st.ListProducts(ctx, ProductQuery{Keyword: "扩展测试商品", Status: -1, Page: 1, Size: 10})
	if err != nil || page.Total != 1 {
		t.Fatalf("关键字查询应命中 1 个商品，实际 %d / %v", page.Total, err)
	}
	page, err = st.ListProducts(ctx, ProductQuery{CategoryID: root, Status: -1, Page: 1, Size: 10})
	if err != nil || page.Total != 1 {
		t.Fatalf("按父分类查询应包含子分类商品，实际 %d / %v", page.Total, err)
	}
	page, err = st.ListProducts(ctx, ProductQuery{Keyword: "扩展测试商品", Status: 0, Page: 1, Size: 10})
	if err != nil || page.Total != 0 {
		t.Fatalf("停用状态过滤应返回 0，实际 %d / %v", page.Total, err)
	}

	if err := st.DeleteProduct(ctx, pid); err != nil {
		t.Fatalf("删除未引用商品失败: %v", err)
	}
	if err := st.DeleteProduct(ctx, pid); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除商品应返回 ErrNotFound，实际 %v", err)
	}

	// 有库存引用的商品不可删除。
	pid2, err := st.SaveProduct(ctx, 0, ProductInput{Code: "EXT-P2", Name: "有库存商品", Status: 1})
	if err != nil {
		t.Fatalf("创建第二个商品失败: %v", err)
	}
	user := OpUser{ID: 1, Name: "测试员"}
	supplier, _ := st.SavePartner(ctx, 0, PartnerInput{Code: "EXT-S2", Name: "扩展供应商", Type: "supplier", Status: 1})
	wh, _ := st.DefaultWarehouse(ctx)
	purchase, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wh.ID, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid2, Qty: model.QtyFrom(1), Price: model.PriceFrom(1)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, purchase, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	if err := st.DeleteProduct(ctx, pid2); !errors.Is(err, ErrInUse) {
		t.Fatalf("有库存的商品删除应返回 ErrInUse，实际 %v", err)
	}
}

func TestCategoryTreeAndDeleteGuards(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	a, err := st.SaveCategory(ctx, 0, CategoryInput{Name: "扩展根"})
	if err != nil {
		t.Fatalf("创建根分类失败: %v", err)
	}
	b, err := st.SaveCategory(ctx, 0, CategoryInput{Name: "扩展子", ParentID: a})
	if err != nil {
		t.Fatalf("创建子分类失败: %v", err)
	}
	c, err := st.SaveCategory(ctx, 0, CategoryInput{Name: "扩展孙", ParentID: b})
	if err != nil {
		t.Fatalf("创建孙分类失败: %v", err)
	}

	tree, err := st.ListCategories(ctx)
	if err != nil {
		t.Fatalf("读取分类树失败: %v", err)
	}
	var found *model.Category
	for i := range tree {
		if tree[i].ID == a {
			found = &tree[i]
			break
		}
	}
	if found == nil || len(found.Children) != 1 || found.Children[0].ID != b {
		t.Fatalf("分类树结构不正确: %+v", found)
	}
	if len(found.Children[0].Children) != 1 || found.Children[0].Children[0].ID != c {
		t.Fatalf("分类树孙级结构不正确: %+v", found.Children[0])
	}

	if err := st.DeleteCategory(ctx, a); !errors.Is(err, ErrInUse) {
		t.Fatalf("删除有子分类的分类应返回 ErrInUse，实际 %v", err)
	}
	if err := st.DeleteCategory(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除不存在的分类应返回 ErrNotFound，实际 %v", err)
	}
	if _, err := st.SaveProduct(ctx, 0, ProductInput{Code: "EXT-P3", Name: "分类商品", CategoryID: c, Status: 1}); err != nil {
		t.Fatalf("创建分类商品失败: %v", err)
	}
	if err := st.DeleteCategory(ctx, c); !errors.Is(err, ErrInUse) {
		t.Fatalf("删除仍被商品使用的分类应返回 ErrInUse，实际 %v", err)
	}
	if err := st.DeleteCategory(ctx, b); !errors.Is(err, ErrInUse) {
		t.Fatalf("删除有子分类的分类应返回 ErrInUse，实际 %v", err)
	}
	empty, err := st.SaveCategory(ctx, 0, CategoryInput{Name: "空分类"})
	if err != nil {
		t.Fatalf("创建空分类失败: %v", err)
	}
	if err := st.DeleteCategory(ctx, empty); err != nil {
		t.Fatalf("删除空分类失败: %v", err)
	}
}

func TestPartnerCRUDAndDeleteGuards(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	both, err := st.SavePartner(ctx, 0, PartnerInput{Code: "EXT-B1", Name: "双向单位", Type: "both", Status: 1})
	if err != nil {
		t.Fatalf("创建双向单位失败: %v", err)
	}
	if _, err := st.SavePartner(ctx, 0, PartnerInput{Code: "EXT-B1", Name: "重复单位", Type: "customer", Status: 1}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("重复单位编码应返回 ErrDuplicate，实际 %v", err)
	}
	if _, err := st.SavePartner(ctx, 999999, PartnerInput{Code: "EXT-NONE", Name: "不存在"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("修改不存在的单位应返回 ErrNotFound，实际 %v", err)
	}
	if err := st.DeletePartner(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除不存在的单位应返回 ErrNotFound，实际 %v", err)
	}

	page, err := st.ListPartners(ctx, PartnerQuery{Type: "customer", Status: -1, Page: 1, Size: 50})
	if err != nil {
		t.Fatalf("查询客户失败: %v", err)
	}
	var hasBoth, hasSupplier bool
	for _, p := range page.List {
		if p.ID == both {
			hasBoth = true
		}
		if p.ID == supplier {
			hasSupplier = true
		}
	}
	if !hasBoth || hasSupplier {
		t.Fatalf("客户类型过滤应包含 both 且不包含 supplier，实际 both=%v supplier=%v", hasBoth, hasSupplier)
	}

	if _, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(1)}},
	}, user); err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.DeletePartner(ctx, customer); !errors.Is(err, ErrInUse) {
		t.Fatalf("被销售单引用的客户删除应返回 ErrInUse，实际 %v", err)
	}
	if _, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(1)}},
	}, user); err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.DeletePartner(ctx, supplier); !errors.Is(err, ErrInUse) {
		t.Fatalf("被采购单引用的供应商删除应返回 ErrInUse，实际 %v", err)
	}
	if err := st.DeletePartner(ctx, both); err != nil {
		t.Fatalf("删除未引用单位失败: %v", err)
	}
}

func TestWarehouseAndAccountDefaults(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	w1, err := st.SaveWarehouse(ctx, 0, WarehouseInput{Code: "EXT-W1", Name: "扩展仓一", IsDefault: true, Status: 1})
	if err != nil {
		t.Fatalf("创建仓库一失败: %v", err)
	}
	w2, err := st.SaveWarehouse(ctx, 0, WarehouseInput{Code: "EXT-W2", Name: "扩展仓二", IsDefault: true, Status: 1})
	if err != nil {
		t.Fatalf("创建仓库二失败: %v", err)
	}
	list, err := st.ListWarehouses(ctx, false)
	if err != nil {
		t.Fatalf("查询仓库失败: %v", err)
	}
	defaults := 0
	var defaultID int64
	for _, w := range list {
		if w.IsDefault {
			defaults++
			defaultID = w.ID
		}
	}
	if defaults != 1 || defaultID != w2 {
		t.Fatalf("默认仓库应唯一且为最后设置的仓库，实际数量 %d ID %d", defaults, defaultID)
	}
	if _, err := st.SaveWarehouse(ctx, 999999, WarehouseInput{Code: "EXT-NONE", Name: "不存在"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("修改不存在的仓库应返回 ErrNotFound，实际 %v", err)
	}
	if err := st.DeleteWarehouse(ctx, w1); err != nil {
		t.Fatalf("删除未引用仓库失败: %v", err)
	}
	if err := st.DeleteWarehouse(ctx, w1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除仓库应返回 ErrNotFound，实际 %v", err)
	}

	a1, err := st.SaveAccount(ctx, 0, AccountInput{Code: "EXT-A1", Name: "扩展账户一", IsDefault: true, Status: 1})
	if err != nil {
		t.Fatalf("创建账户一失败: %v", err)
	}
	a2, err := st.SaveAccount(ctx, 0, AccountInput{Code: "EXT-A2", Name: "扩展账户二", IsDefault: true, Status: 1})
	if err != nil {
		t.Fatalf("创建账户二失败: %v", err)
	}
	accounts, err := st.ListAccounts(ctx, false)
	if err != nil {
		t.Fatalf("查询账户失败: %v", err)
	}
	defaults = 0
	defaultID = 0
	for _, a := range accounts {
		if a.IsDefault {
			defaults++
			defaultID = a.ID
		}
	}
	if defaults != 1 || defaultID != a2 {
		t.Fatalf("默认账户应唯一且为最后设置的账户，实际数量 %d ID %d", defaults, defaultID)
	}
	if _, err := st.SaveAccount(ctx, 999999, AccountInput{Code: "EXT-NONE", Name: "不存在"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("修改不存在的账户应返回 ErrNotFound，实际 %v", err)
	}
	if err := st.DeleteAccount(ctx, a1); err != nil {
		t.Fatalf("删除未引用账户失败: %v", err)
	}
	if err := st.DeleteAccount(ctx, a1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除账户应返回 ErrNotFound，实际 %v", err)
	}
}

func TestSettingsSessionsUsersRolesAndLogs(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if st.Setting(ctx, "company_name") == "" {
		t.Fatalf("默认参数 company_name 不应为空")
	}
	if got := st.SettingInt(ctx, "session_hours", 72); got != 72 {
		t.Fatalf("默认会话时长期望 72，实际 %d", got)
	}
	if st.SettingBool(ctx, "allow_negative_stock", true) {
		t.Fatalf("默认不允许负库存")
	}
	if err := st.SaveSettings(ctx, map[string]string{"allow_negative_stock": "1", "session_hours": "12"}); err != nil {
		t.Fatalf("保存参数失败: %v", err)
	}
	if !st.SettingBool(ctx, "allow_negative_stock", false) {
		t.Fatalf("参数 allow_negative_stock 应已更新为 true")
	}
	if got := st.SettingInt(ctx, "session_hours", 72); got != 12 {
		t.Fatalf("参数 session_hours 期望 12，实际 %d", got)
	}

	roles, err := st.ListRoles(ctx)
	if err != nil {
		t.Fatalf("查询角色失败: %v", err)
	}
	var viewer, admin model.Role
	for _, r := range roles {
		if r.Code == "viewer" {
			viewer = r
		}
		if r.Code == "admin" {
			admin = r
		}
	}
	if viewer.ID == 0 || admin.ID == 0 {
		t.Fatalf("缺少内置角色")
	}
	if err := st.UpdateRole(ctx, admin.ID, RoleInput{Name: "改名"}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("内置管理员角色不可修改，实际 %v", err)
	}
	if err := st.DeleteRole(ctx, admin.ID); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("内置管理员角色不可删除，实际 %v", err)
	}
	rid, err := st.CreateRole(ctx, RoleInput{Code: "ext_role", Name: "扩展角色", Perms: []string{"stock:view", "bad:perm"}})
	if err != nil {
		t.Fatalf("创建角色失败: %v", err)
	}
	role, _ := st.GetRole(ctx, rid)
	if len(role.Perms) != 1 || role.Perms[0] != "stock:view" {
		t.Fatalf("非法权限应被过滤，实际 %v", role.Perms)
	}

	if _, err := st.CreateUser(ctx, UserInput{Username: "ext_user", Password: "123", Name: "短口令"}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("短口令应返回 ErrBadRequest，实际 %v", err)
	}
	uid, err := st.CreateUser(ctx, UserInput{Username: "ext_user", Password: "abc123456", Name: "扩展用户", RoleID: rid, Status: 1})
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	if _, err := st.CreateUser(ctx, UserInput{Username: "ext_user", Password: "abc123456"}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("重复用户名应返回 ErrDuplicate，实际 %v", err)
	}
	if err := st.DeleteRole(ctx, rid); !errors.Is(err, ErrInUse) {
		t.Fatalf("仍被用户使用的角色不可删除，实际 %v", err)
	}

	u, perms, err := st.Authenticate(ctx, "ext_user", "abc123456")
	if err != nil {
		t.Fatalf("用户登录失败: %v", err)
	}
	if !perms.Has("stock:view") || perms.Has("product:delete") {
		t.Fatalf("用户权限不正确: %v", perms.List())
	}
	if _, _, err := st.Authenticate(ctx, "ext_user", "wrong"); err == nil {
		t.Fatalf("错误口令不应登录成功")
	}
	if err := st.UpdatePassword(ctx, u.ID, "wrong", "newpass123"); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("原口令错误应返回 ErrBadRequest，实际 %v", err)
	}
	if err := st.UpdatePassword(ctx, u.ID, "abc123456", "newpass123"); err != nil {
		t.Fatalf("修改口令失败: %v", err)
	}
	if _, _, err := st.Authenticate(ctx, "ext_user", "newpass123"); err != nil {
		t.Fatalf("新口令登录失败: %v", err)
	}

	token, _, err := st.CreateSession(ctx, u.ID, "127.0.0.1", "test", time.Hour)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	if _, ok := st.SessionUser(ctx, token); !ok {
		t.Fatalf("有效会话应能取到用户")
	}
	if err := st.DeleteSession(ctx, token); err != nil {
		t.Fatalf("删除会话失败: %v", err)
	}
	if _, ok := st.SessionUser(ctx, token); ok {
		t.Fatalf("已删除会话不应有效")
	}

	expired, _, err := st.CreateSession(ctx, u.ID, "127.0.0.1", "test", -time.Hour)
	if err != nil {
		t.Fatalf("创建过期会话失败: %v", err)
	}
	if _, ok := st.SessionUser(ctx, expired); ok {
		t.Fatalf("过期会话不应有效")
	}

	// 重置口令应清除该用户的全部会话。
	token2, _, err := st.CreateSession(ctx, u.ID, "127.0.0.1", "test", time.Hour)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	if err := st.ResetPassword(ctx, u.ID, "reset123"); err != nil {
		t.Fatalf("重置口令失败: %v", err)
	}
	if _, ok := st.SessionUser(ctx, token2); ok {
		t.Fatalf("重置口令后旧会话应失效")
	}

	// 停用用户不能登录。
	if err := st.UpdateUser(ctx, u.ID, UserInput{Name: "扩展用户", RoleID: rid, Status: 0}); err != nil {
		t.Fatalf("停用用户失败: %v", err)
	}
	if _, _, err := st.Authenticate(ctx, "ext_user", "reset123"); err == nil {
		t.Fatalf("停用用户不应登录成功")
	}
	// 内置超级管理员不可停用、不可删除。
	adminUser, err := st.GetUserByName(ctx, "admin")
	if err != nil {
		t.Fatalf("读取管理员失败: %v", err)
	}
	if err := st.UpdateUser(ctx, adminUser.ID, UserInput{Name: "管理员", RoleID: admin.ID, Status: 0}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("停用超级管理员应返回 ErrBadRequest，实际 %v", err)
	}
	if err := st.DeleteUser(ctx, adminUser.ID); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("删除超级管理员应返回 ErrBadRequest，实际 %v", err)
	}

	// 操作日志查询。
	st.AddLog(ctx, u.ID, "ext_user", "扩展模块", "测试动作", "hello world", "127.0.0.1")
	logs, err := st.ListLogs(ctx, LogQuery{Keyword: "hello", Page: 1, Size: 10})
	if err != nil || logs.Total == 0 {
		t.Fatalf("日志关键字查询失败: %d / %v", logs.Total, err)
	}
	logs, err = st.ListLogs(ctx, LogQuery{Module: "扩展模块", Page: 1, Size: 10})
	if err != nil || logs.Total != 1 {
		t.Fatalf("日志模块查询期望 1 条，实际 %d / %v", logs.Total, err)
	}
	mods, err := st.LogModules(ctx)
	if err != nil {
		t.Fatalf("查询日志模块失败: %v", err)
	}
	found := false
	for _, m := range mods {
		if m == "扩展模块" {
			found = true
		}
	}
	if !found {
		t.Fatalf("日志模块列表应包含扩展模块，实际 %v", mods)
	}

	// 删除空闲用户和角色。
	if err := st.DeleteUser(ctx, uid); err != nil {
		t.Fatalf("删除用户失败: %v", err)
	}
	if err := st.DeleteRole(ctx, rid); err != nil {
		t.Fatalf("删除角色失败: %v", err)
	}
}
