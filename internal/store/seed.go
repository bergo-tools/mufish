package store

import (
	"context"
	"database/sql"
	"fmt"

	"mufish/internal/auth"
	"mufish/internal/model"
)

// 默认口令，首次登录后请及时修改。
const DefaultAdminPassword = "admin123"

// 内置角色定义。
type seedRole struct {
	Code   string
	Name   string
	Remark string
	Perms  []string
}

func defaultRoles() []seedRole {
	return []seedRole{
		{
			Code:   "admin",
			Name:   "系统管理员",
			Remark: "拥有全部权限，不可删除",
			Perms:  []string{model.SuperPerm},
		},
		{
			Code:   "manager",
			Name:   "业务经理",
			Remark: "除系统管理与备份外的全部业务权限",
			Perms: []string{
				"dashboard:view",
				"product:view", "product:create", "product:update", "product:delete", "product:import",
				"category:view", "category:manage",
				"partner:view", "partner:create", "partner:update", "partner:delete",
				"warehouse:view", "warehouse:manage",
				"purchase:view", "purchase:create", "purchase:update", "purchase:delete", "purchase:confirm", "purchase:void",
				"sales:view", "sales:create", "sales:update", "sales:delete", "sales:confirm", "sales:void",
				"stock:view", "stock:ledger", "stock:take", "stock:transfer",
				"payment:view", "payment:create", "payment:confirm", "payment:void",
				"account:view", "account:manage",
				"report:view", "report:export",
				"user:view", "role:view", "setting:view", "log:view",
			},
		},
		{
			Code:   "purchaser",
			Name:   "采购员",
			Remark: "负责采购入库与供应商维护",
			Perms: []string{
				"dashboard:view",
				"product:view", "category:view",
				"partner:view", "partner:create", "partner:update",
				"warehouse:view",
				"purchase:view", "purchase:create", "purchase:update", "purchase:confirm",
				"stock:view", "stock:ledger",
				"payment:view", "payment:create",
				"account:view",
				"report:view",
			},
		},
		{
			Code:   "salesman",
			Name:   "销售员",
			Remark: "负责销售出库与客户维护",
			Perms: []string{
				"dashboard:view",
				"product:view", "category:view",
				"partner:view", "partner:create", "partner:update",
				"warehouse:view",
				"sales:view", "sales:create", "sales:update", "sales:confirm",
				"stock:view",
				"payment:view", "payment:create",
				"account:view",
				"report:view",
			},
		},
		{
			Code:   "stockman",
			Name:   "仓管员",
			Remark: "负责出入库执行、盘点与调拨",
			Perms: []string{
				"dashboard:view",
				"product:view", "category:view",
				"partner:view",
				"warehouse:view",
				"purchase:view", "purchase:confirm",
				"sales:view", "sales:confirm",
				"stock:view", "stock:ledger", "stock:take", "stock:transfer",
				"report:view",
			},
		},
		{
			Code:   "finance",
			Name:   "财务",
			Remark: "负责收付款与应收应付",
			Perms: []string{
				"dashboard:view",
				"product:view",
				"partner:view", "partner:create", "partner:update",
				"purchase:view", "sales:view",
				"stock:view",
				"payment:view", "payment:create", "payment:confirm", "payment:void",
				"account:view", "account:manage",
				"report:view", "report:export",
			},
		},
		{
			Code:   "viewer",
			Name:   "只读用户",
			Remark: "仅可查看，不能修改任何数据",
			Perms: []string{
				"dashboard:view",
				"product:view", "category:view",
				"partner:view", "warehouse:view", "account:view",
				"purchase:view", "sales:view",
				"stock:view", "stock:ledger",
				"payment:view",
				"report:view",
			},
		},
	}
}

func defaultSettings() map[string]string {
	return map[string]string{
		"company_name":          "木鱼贸易有限公司",
		"company_short":         "木鱼",
		"company_address":       "",
		"company_phone":         "",
		"company_tax_no":        "",
		"allow_negative_stock":  "0",
		"purchase_price_follow": "1", // 采购入库是否更新商品参考进价
		"sales_price_follow":    "0", // 销售出库是否更新商品参考售价
		"purchase_cost_include": "1", // 采购优惠/运费等是否计入库存成本
		"doc_print_note":        "感谢惠顾，欢迎再次光临！",
		"session_hours":         "72",
	}
}

func (s *Store) seed(ctx context.Context) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		now := model.Now()

		// --- 角色 ---
		// 默认权限只在角色首次创建时写入。已存在角色的权限由管理员维护，
		// 不能在每次启动时用 INSERT OR IGNORE 重新加回来。
		for _, r := range defaultRoles() {
			var id int64
			created := false
			err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE code=?`, r.Code).Scan(&id)
			if err == sql.ErrNoRows {
				res, err := tx.ExecContext(ctx,
					`INSERT INTO roles(code, name, remark, is_system, created_at, updated_at) VALUES(?,?,?,1,?,?)`,
					r.Code, r.Name, r.Remark, now, now)
				if err != nil {
					return err
				}
				id, _ = res.LastInsertId()
				created = true
			} else if err != nil {
				return err
			}
			if !created {
				continue
			}
			for _, p := range r.Perms {
				if _, err := tx.ExecContext(ctx,
					`INSERT OR IGNORE INTO role_permissions(role_id, perm) VALUES(?,?)`, id, p); err != nil {
					return err
				}
			}
		}

		// --- 超级管理员 ---
		var userCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
			return err
		}
		if userCount == 0 {
			hash, err := auth.HashPassword(DefaultAdminPassword)
			if err != nil {
				return err
			}
			var roleID int64
			if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE code='admin'`).Scan(&roleID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO users(username, password, name, role_id, status, is_super, remark, created_at, updated_at)
				 VALUES(?,?,?,?,1,1,?,?,?)`,
				"admin", hash, "系统管理员", roleID, "内置超级管理员账号", now, now); err != nil {
				return err
			}
		}

		// --- 默认仓库 ---
		var whCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM warehouses`).Scan(&whCount); err != nil {
			return err
		}
		if whCount == 0 {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO warehouses(code, name, is_default, sort, status, remark, created_at, updated_at)
				 VALUES('WH001','主仓库',1,1,1,'系统默认仓库',?,?)`, now, now); err != nil {
				return err
			}
		}

		// --- 默认资金账户 ---
		var accCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&accCount); err != nil {
			return err
		}
		if accCount == 0 {
			defaults := []struct {
				code, name, typ string
				sort            int
			}{
				{"ACC001", "现金", "cash", 1},
				{"ACC002", "银行账户", "bank", 2},
			}
			for _, a := range defaults {
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO accounts(code, name, type, sort, status, created_at, updated_at)
					 VALUES(?,?,?,?,1,?,?)`, a.code, a.name, a.typ, a.sort, now, now); err != nil {
					return err
				}
			}
		}

		// --- 默认分类 ---
		var catCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories`).Scan(&catCount); err != nil {
			return err
		}
		if catCount == 0 {
			for i, name := range []string{"默认分类"} {
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO categories(name, parent_id, sort) VALUES(?,0,?)`, name, i+1); err != nil {
					return err
				}
			}
		}

		// --- 默认参数 ---
		for k, v := range defaultSettings() {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO settings(key, value) VALUES(?,?)`, k, v); err != nil {
				return err
			}
		}

		// --- 演示数据开关 ---
		// 用一次性标记而不是「products 表为空」判断，避免用户清空商品后
		// 重启又被写入演示数据，或与已有编码冲突导致启动失败。
		var demoSeeded string
		err := tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='demo_seeded'`).Scan(&demoSeeded)
		if err == sql.ErrNoRows {
			var prodCount int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM products`).Scan(&prodCount); err != nil {
				return err
			}
			if prodCount == 0 {
				if err := seedDemo(ctx, tx); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO settings(key, value) VALUES('demo_seeded','1')`); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		return nil
	})
}

// seedDemo 写入少量示例基础资料，便于首次打开即可上手。
func seedDemo(ctx context.Context, tx *sql.Tx) error {
	now := model.Now()

	var catID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM categories ORDER BY sort LIMIT 1`).Scan(&catID); err != nil {
		return err
	}
	type demoProduct struct {
		code, name, spec, unit string
		pp, sp                 float64
		safety                 float64
	}
	products := []demoProduct{
		{"SP001", "联想笔记本 ThinkPad E14", "i5/16G/512G", "台", 4200, 4899, 2},
		{"SP002", "罗技无线鼠标 M185", "黑色", "个", 55, 89, 10},
		{"SP003", "A4 复印纸", "70g 500张/包", "包", 18.5, 26, 20},
		{"SP004", "佳能打印机硒鼓", "CRG-337", "支", 210, 320, 3},
		{"SP005", "网线", "超六类 3米", "根", 8.5, 15, 30},
	}
	for i, p := range products {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO products(code, name, spec, unit, category_id, purchase_price, sale_price,
			   safety_stock, status, remark, created_at, updated_at)
			 VALUES(?,?,?,?,?,?,?,?,1,'',?,?)`,
			p.code, p.name, p.spec, p.unit, catID,
			model.PriceFrom(p.pp), model.PriceFrom(p.sp), p.safety, now, now); err != nil {
			return err
		}
		_ = i
	}

	type demoPartner struct {
		code, name, typ, contact, phone string
		initReceivable, initPayable     float64
	}
	partners := []demoPartner{
		{"KH001", "华南电子科技有限公司", "customer", "张经理", "13800001111", 0, 0},
		{"KH002", "天成商贸有限公司", "customer", "李女士", "13900002222", 0, 0},
		{"GYS001", "深圳华强北数码供应商", "supplier", "王老板", "13700003333", 0, 0},
		{"GYS002", "晨光办公用品批发", "supplier", "赵先生", "13600004444", 0, 0},
	}
	for _, p := range partners {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO partners(code, name, type, contact, phone, init_receivable, init_payable,
			   status, remark, created_at, updated_at)
			 VALUES(?,?,?,?,?,?,?,1,'',?,?)`,
			p.code, p.name, p.typ, p.contact, p.phone,
			model.YuanFrom(p.initReceivable), model.YuanFrom(p.initPayable), now, now); err != nil {
			return err
		}
	}

	// 演示用户：每个内置角色一个，口令同 admin123
	hash, err := auth.HashPassword(DefaultAdminPassword)
	if err != nil {
		return err
	}
	users := []struct{ username, name, role string }{
		{"manager", "王经理", "manager"},
		{"sales", "李销售", "salesman"},
		{"stock", "赵仓管", "stockman"},
	}
	for _, u := range users {
		var roleID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE code=?`, u.role).Scan(&roleID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO users(username, password, name, role_id, status, remark, created_at, updated_at)
			 VALUES(?,?,?,?,1,'演示账号',?,?)`,
			u.username, hash, u.name, roleID, now, now); err != nil {
			return err
		}
	}

	// 期初库存
	var whID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM warehouses WHERE is_default=1 LIMIT 1`).Scan(&whID); err != nil {
		return err
	}
	initQty := map[string]float64{"SP001": 5, "SP002": 40, "SP003": 120, "SP004": 12, "SP005": 200}
	for code, q := range initQty {
		var pid int64
		var price int64
		if err := tx.QueryRowContext(ctx, `SELECT id, purchase_price FROM products WHERE code=?`, code).Scan(&pid, &price); err != nil {
			return err
		}
		amount := int64(0)
		if q > 0 {
			// price 单位为万分之一元，amount 单位为分：price * qty / 100
			amount = int64(float64(price)*q/100 + 0.5)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO stocks(warehouse_id, product_id, qty, cost_price, amount, updated_at)
			 VALUES(?,?,?,?,?,?)`, whID, pid, q, price, amount, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO stock_moves(warehouse_id, product_id, biz_type, biz_id, biz_no, biz_date, direction,
			   qty, price, amount, balance_after, remark, created_by, created_by_name, created_at)
			 VALUES(?,?,'init',0,'期初',?,1,?,?,?,?,'系统初始化期初库存',0,'系统',?)`,
			whID, pid, now[:10], q, price, amount, q, now); err != nil {
			return err
		}
	}
	return nil
}

var _ = fmt.Sprintf
