package model

// ---------------------------------------------------------------------------
// 系统
// ---------------------------------------------------------------------------

// User 用户。
type User struct {
	ID          int64  `json:"id" db:"id"`
	Username    string `json:"username" db:"username"`
	Password    string `json:"-" db:"password"`
	Name        string `json:"name" db:"name"`
	Phone       string `json:"phone" db:"phone"`
	Email       string `json:"email" db:"email"`
	RoleID      int64  `json:"role_id" db:"role_id"`
	RoleName    string `json:"role_name" db:"role_name"`
	Status      int    `json:"status" db:"status"` // 1 启用 0 停用
	IsSuper     bool   `json:"is_super" db:"is_super"`
	Remark      string `json:"remark" db:"remark"`
	LastLoginAt string `json:"last_login_at" db:"last_login_at"`
	CreatedAt   string `json:"created_at" db:"created_at"`
	UpdatedAt   string `json:"updated_at" db:"updated_at"`
}

// Role 角色。
type Role struct {
	ID        int64    `json:"id" db:"id"`
	Code      string   `json:"code" db:"code"`
	Name      string   `json:"name" db:"name"`
	Remark    string   `json:"remark" db:"remark"`
	IsSystem  bool     `json:"is_system" db:"is_system"`
	Perms     []string `json:"perms"`
	UserCount int      `json:"user_count"`
	CreatedAt string   `json:"created_at" db:"created_at"`
	UpdatedAt string   `json:"updated_at" db:"updated_at"`
}

// AuditLog 操作日志。
type AuditLog struct {
	ID        int64  `json:"id" db:"id"`
	UserID    int64  `json:"user_id" db:"user_id"`
	Username  string `json:"username" db:"username"`
	Module    string `json:"module" db:"module"`
	Action    string `json:"action" db:"action"`
	Detail    string `json:"detail" db:"detail"`
	IP        string `json:"ip" db:"ip"`
	CreatedAt string `json:"created_at" db:"created_at"`
}

// ---------------------------------------------------------------------------
// 基础资料
// ---------------------------------------------------------------------------

// Category 商品分类。
type Category struct {
	ID       int64       `json:"id" db:"id"`
	Name     string      `json:"name" db:"name"`
	ParentID int64       `json:"parent_id" db:"parent_id"`
	Sort     int         `json:"sort" db:"sort"`
	Remark   string      `json:"remark" db:"remark"`
	Children []*Category `json:"children,omitempty"`
}

// Product 商品。
type Product struct {
	ID             int64  `json:"id" db:"id"`
	Code           string `json:"code" db:"code"`
	Name           string `json:"name" db:"name"`
	Spec           string `json:"spec" db:"spec"`
	Unit           string `json:"unit" db:"unit"`
	CategoryID     int64  `json:"category_id" db:"category_id"`
	CategoryName   string `json:"category_name" db:"category_name"`
	Barcode        string `json:"barcode" db:"barcode"`
	Brand          string `json:"brand" db:"brand"`
	PurchasePrice  Price  `json:"purchase_price" db:"purchase_price"`
	SalePrice      Price  `json:"sale_price" db:"sale_price"`
	SafetyStock    Qty    `json:"safety_stock" db:"safety_stock"`
	SafetyStockMax Qty    `json:"safety_stock_max" db:"safety_stock_max"`
	Status         int    `json:"status" db:"status"` // 1 启用 0 停用
	Remark         string `json:"remark" db:"remark"`
	CreatedAt      string `json:"created_at" db:"created_at"`
	UpdatedAt      string `json:"updated_at" db:"updated_at"`

	// 查询附加字段
	StockQty Qty `json:"stock_qty" db:"stock_qty"`
}

// Partner 往来单位（客户 / 供应商）。
type Partner struct {
	ID             int64  `json:"id" db:"id"`
	Code           string `json:"code" db:"code"`
	Name           string `json:"name" db:"name"`
	Type           string `json:"type" db:"type"` // customer / supplier / both
	Contact        string `json:"contact" db:"contact"`
	Phone          string `json:"phone" db:"phone"`
	Address        string `json:"address" db:"address"`
	TaxNo          string `json:"tax_no" db:"tax_no"`
	BankName       string `json:"bank_name" db:"bank_name"`
	BankAccount    string `json:"bank_account" db:"bank_account"`
	CreditLimit    Money  `json:"credit_limit" db:"credit_limit"`
	InitReceivable Money  `json:"init_receivable" db:"init_receivable"` // 期初应收
	InitPayable    Money  `json:"init_payable" db:"init_payable"`       // 期初应付
	Status         int    `json:"status" db:"status"`
	Remark         string `json:"remark" db:"remark"`
	CreatedAt      string `json:"created_at" db:"created_at"`
	UpdatedAt      string `json:"updated_at" db:"updated_at"`

	// 查询附加字段
	Receivable Money `json:"receivable" db:"receivable"`
	Payable    Money `json:"payable" db:"payable"`
}

// Warehouse 仓库。
type Warehouse struct {
	ID        int64  `json:"id" db:"id"`
	Code      string `json:"code" db:"code"`
	Name      string `json:"name" db:"name"`
	Address   string `json:"address" db:"address"`
	Contact   string `json:"contact" db:"contact"`
	Phone     string `json:"phone" db:"phone"`
	IsDefault bool   `json:"is_default" db:"is_default"`
	Sort      int    `json:"sort" db:"sort"`
	Status    int    `json:"status" db:"status"`
	Remark    string `json:"remark" db:"remark"`
	CreatedAt string `json:"created_at" db:"created_at"`
	UpdatedAt string `json:"updated_at" db:"updated_at"`

	// 查询附加字段
	ProductKinds int   `json:"product_kinds" db:"product_kinds"`
	TotalQty     Qty   `json:"total_qty" db:"total_qty"`
	TotalAmount  Money `json:"total_amount" db:"total_amount"`
}

// Account 资金账户（现金 / 银行 / 支付宝 …）。
type Account struct {
	ID        int64  `json:"id" db:"id"`
	Code      string `json:"code" db:"code"`
	Name      string `json:"name" db:"name"`
	Type      string `json:"type" db:"type"` // cash / bank / alipay / wechat / other
	Balance   Money  `json:"balance" db:"balance"`
	IsDefault bool   `json:"is_default" db:"is_default"`
	Sort      int    `json:"sort" db:"sort"`
	Status    int    `json:"status" db:"status"`
	Remark    string `json:"remark" db:"remark"`
	CreatedAt string `json:"created_at" db:"created_at"`
	UpdatedAt string `json:"updated_at" db:"updated_at"`
}

// ---------------------------------------------------------------------------
// 单据
// ---------------------------------------------------------------------------

// 单据状态。
const (
	DocDraft     = 0 // 草稿
	DocConfirmed = 1 // 已确认（入库 / 出库 / 收付款完成）
	DocVoided    = 2 // 已作废
)

// DocStatusText 单据状态中文名。
func DocStatusText(s int) string {
	switch s {
	case DocDraft:
		return "草稿"
	case DocConfirmed:
		return "已确认"
	case DocVoided:
		return "已作废"
	}
	return "未知"
}

// OrderItem 单据明细行（采购 / 销售 / 调拨通用）。
type OrderItem struct {
	ID          int64  `json:"id" db:"id"`
	DocID       int64  `json:"doc_id" db:"doc_id"`
	ProductID   int64  `json:"product_id" db:"product_id"`
	ProductCode string `json:"product_code" db:"product_code"`
	ProductName string `json:"product_name" db:"product_name"`
	Spec        string `json:"spec" db:"spec"`
	Unit        string `json:"unit" db:"unit"`
	Qty         Qty    `json:"qty" db:"qty"`
	Price       Price  `json:"price" db:"price"`
	Amount      Money  `json:"amount" db:"amount"`
	Remark      string `json:"remark" db:"remark"`
	Sort        int    `json:"sort" db:"sort"`
}

// PurchaseOrder 采购入库单。
type PurchaseOrder struct {
	ID             int64       `json:"id" db:"id"`
	OrderNo        string      `json:"order_no" db:"order_no"`
	SupplierID     int64       `json:"supplier_id" db:"supplier_id"`
	SupplierName   string      `json:"supplier_name" db:"supplier_name"`
	WarehouseID    int64       `json:"warehouse_id" db:"warehouse_id"`
	WarehouseName  string      `json:"warehouse_name" db:"warehouse_name"`
	OrderDate      string      `json:"order_date" db:"order_date"`
	Status         int         `json:"status" db:"status"`
	TotalQty       Qty         `json:"total_qty" db:"total_qty"`
	TotalAmount    Money       `json:"total_amount" db:"total_amount"`
	DiscountAmount Money       `json:"discount_amount" db:"discount_amount"`
	OtherFee       Money       `json:"other_fee" db:"other_fee"`
	SettleAmount   Money       `json:"settle_amount" db:"settle_amount"`
	PaidAmount     Money       `json:"paid_amount" db:"paid_amount"`
	Remark         string      `json:"remark" db:"remark"`
	CreatedBy      int64       `json:"created_by" db:"created_by"`
	CreatedByName  string      `json:"created_by_name" db:"created_by_name"`
	ConfirmedAt    string      `json:"confirmed_at" db:"confirmed_at"`
	ConfirmedBy    int64       `json:"confirmed_by" db:"confirmed_by"`
	CreatedAt      string      `json:"created_at" db:"created_at"`
	UpdatedAt      string      `json:"updated_at" db:"updated_at"`
	Items          []OrderItem `json:"items"`
}

// SalesOrder 销售出库单。
type SalesOrder struct {
	ID             int64       `json:"id" db:"id"`
	OrderNo        string      `json:"order_no" db:"order_no"`
	CustomerID     int64       `json:"customer_id" db:"customer_id"`
	CustomerName   string      `json:"customer_name" db:"customer_name"`
	WarehouseID    int64       `json:"warehouse_id" db:"warehouse_id"`
	WarehouseName  string      `json:"warehouse_name" db:"warehouse_name"`
	OrderDate      string      `json:"order_date" db:"order_date"`
	Status         int         `json:"status" db:"status"`
	TotalQty       Qty         `json:"total_qty" db:"total_qty"`
	TotalAmount    Money       `json:"total_amount" db:"total_amount"`
	CostAmount     Money       `json:"cost_amount" db:"cost_amount"`
	DiscountAmount Money       `json:"discount_amount" db:"discount_amount"`
	OtherFee       Money       `json:"other_fee" db:"other_fee"`
	SettleAmount   Money       `json:"settle_amount" db:"settle_amount"`
	ReceivedAmount Money       `json:"received_amount" db:"received_amount"`
	Remark         string      `json:"remark" db:"remark"`
	CreatedBy      int64       `json:"created_by" db:"created_by"`
	CreatedByName  string      `json:"created_by_name" db:"created_by_name"`
	ConfirmedAt    string      `json:"confirmed_at" db:"confirmed_at"`
	ConfirmedBy    int64       `json:"confirmed_by" db:"confirmed_by"`
	CreatedAt      string      `json:"created_at" db:"created_at"`
	UpdatedAt      string      `json:"updated_at" db:"updated_at"`
	Items          []OrderItem `json:"items"`
}

// Stock 库存余额。
type Stock struct {
	ID            int64  `json:"id" db:"id"`
	WarehouseID   int64  `json:"warehouse_id" db:"warehouse_id"`
	WarehouseName string `json:"warehouse_name" db:"warehouse_name"`
	ProductID     int64  `json:"product_id" db:"product_id"`
	ProductCode   string `json:"product_code" db:"product_code"`
	ProductName   string `json:"product_name" db:"product_name"`
	Spec          string `json:"spec" db:"spec"`
	Unit          string `json:"unit" db:"unit"`
	CategoryName  string `json:"category_name" db:"category_name"`
	Qty           Qty    `json:"qty" db:"qty"`
	CostPrice     Price  `json:"cost_price" db:"cost_price"`
	Amount        Money  `json:"amount" db:"amount"`
	SafetyStock   Qty    `json:"safety_stock" db:"safety_stock"`
	UpdatedAt     string `json:"updated_at" db:"updated_at"`
}

// StockMove 库存流水。
type StockMove struct {
	ID            int64  `json:"id" db:"id"`
	WarehouseID   int64  `json:"warehouse_id" db:"warehouse_id"`
	WarehouseName string `json:"warehouse_name" db:"warehouse_name"`
	ProductID     int64  `json:"product_id" db:"product_id"`
	ProductCode   string `json:"product_code" db:"product_code"`
	ProductName   string `json:"product_name" db:"product_name"`
	Spec          string `json:"spec" db:"spec"`
	Unit          string `json:"unit" db:"unit"`
	BizType       string `json:"biz_type" db:"biz_type"` // purchase/sales/take/transfer_in/transfer_out/init
	BizID         int64  `json:"biz_id" db:"biz_id"`
	BizNo         string `json:"biz_no" db:"biz_no"`
	BizDate       string `json:"biz_date" db:"biz_date"`   // 业务日期（单据日期）
	Direction     int    `json:"direction" db:"direction"` // 1 入库 -1 出库
	Qty           Qty    `json:"qty" db:"qty"`             // 带符号数量
	Price         Price  `json:"price" db:"price"`
	Amount        Money  `json:"amount" db:"amount"`
	BalanceAfter  Qty    `json:"balance_after" db:"balance_after"`
	Remark        string `json:"remark" db:"remark"`
	CreatedBy     int64  `json:"created_by" db:"created_by"`
	CreatedByName string `json:"created_by_name" db:"created_by_name"`
	CreatedAt     string `json:"created_at" db:"created_at"`
}

// BizTypeText 业务类型中文名。
func BizTypeText(t string) string {
	switch t {
	case "purchase":
		return "采购入库"
	case "sales":
		return "销售出库"
	case "take":
		return "库存盘点"
	case "transfer_in":
		return "调拨入库"
	case "transfer_out":
		return "调拨出库"
	case "init":
		return "期初库存"
	case "adjust":
		return "手工调整"
	}
	return t
}

// StockTake 盘点单。
type StockTake struct {
	ID            int64           `json:"id" db:"id"`
	TakeNo        string          `json:"take_no" db:"take_no"`
	WarehouseID   int64           `json:"warehouse_id" db:"warehouse_id"`
	WarehouseName string          `json:"warehouse_name" db:"warehouse_name"`
	TakeDate      string          `json:"take_date" db:"take_date"`
	Status        int             `json:"status" db:"status"`
	DiffQty       Qty             `json:"diff_qty" db:"diff_qty"`
	DiffAmount    Money           `json:"diff_amount" db:"diff_amount"`
	Remark        string          `json:"remark" db:"remark"`
	CreatedBy     int64           `json:"created_by" db:"created_by"`
	CreatedByName string          `json:"created_by_name" db:"created_by_name"`
	FinishedAt    string          `json:"finished_at" db:"finished_at"`
	CreatedAt     string          `json:"created_at" db:"created_at"`
	Items         []StockTakeItem `json:"items"`
}

// StockTakeItem 盘点明细。
type StockTakeItem struct {
	ID          int64  `json:"id" db:"id"`
	TakeID      int64  `json:"take_id" db:"take_id"`
	ProductID   int64  `json:"product_id" db:"product_id"`
	ProductCode string `json:"product_code" db:"product_code"`
	ProductName string `json:"product_name" db:"product_name"`
	Spec        string `json:"spec" db:"spec"`
	Unit        string `json:"unit" db:"unit"`
	BookQty     Qty    `json:"book_qty" db:"book_qty"`
	RealQty     Qty    `json:"real_qty" db:"real_qty"`
	DiffQty     Qty    `json:"diff_qty" db:"diff_qty"`
	Price       Price  `json:"price" db:"price"`
	DiffAmount  Money  `json:"diff_amount" db:"diff_amount"`
	Remark      string `json:"remark" db:"remark"`
}

// Transfer 调拨单。
type Transfer struct {
	ID              int64          `json:"id" db:"id"`
	TransferNo      string         `json:"transfer_no" db:"transfer_no"`
	FromWarehouseID int64          `json:"from_warehouse_id" db:"from_warehouse_id"`
	FromName        string         `json:"from_name" db:"from_name"`
	ToWarehouseID   int64          `json:"to_warehouse_id" db:"to_warehouse_id"`
	ToName          string         `json:"to_name" db:"to_name"`
	TransferDate    string         `json:"transfer_date" db:"transfer_date"`
	Status          int            `json:"status" db:"status"`
	TotalQty        Qty            `json:"total_qty" db:"total_qty"`
	TotalAmount     Money          `json:"total_amount" db:"total_amount"`
	Remark          string         `json:"remark" db:"remark"`
	CreatedBy       int64          `json:"created_by" db:"created_by"`
	CreatedByName   string         `json:"created_by_name" db:"created_by_name"`
	ConfirmedAt     string         `json:"confirmed_at" db:"confirmed_at"`
	CreatedAt       string         `json:"created_at" db:"created_at"`
	Items           []TransferItem `json:"items"`
}

// TransferItem 调拨明细。
type TransferItem struct {
	ID          int64  `json:"id" db:"id"`
	TransferID  int64  `json:"transfer_id" db:"transfer_id"`
	ProductID   int64  `json:"product_id" db:"product_id"`
	ProductCode string `json:"product_code" db:"product_code"`
	ProductName string `json:"product_name" db:"product_name"`
	Spec        string `json:"spec" db:"spec"`
	Unit        string `json:"unit" db:"unit"`
	Qty         Qty    `json:"qty" db:"qty"`
	Price       Price  `json:"price" db:"price"`
	Amount      Money  `json:"amount" db:"amount"`
}

// 收付款类型。
const (
	PayTypeReceipt = "receipt" // 收款
	PayTypePayment = "payment" // 付款
)

// Payment 收款单 / 付款单。
type Payment struct {
	ID            int64  `json:"id" db:"id"`
	PaymentNo     string `json:"payment_no" db:"payment_no"`
	Type          string `json:"type" db:"type"`
	PartnerID     int64  `json:"partner_id" db:"partner_id"`
	PartnerName   string `json:"partner_name" db:"partner_name"`
	AccountID     int64  `json:"account_id" db:"account_id"`
	AccountName   string `json:"account_name" db:"account_name"`
	PayDate       string `json:"pay_date" db:"pay_date"`
	Amount        Money  `json:"amount" db:"amount"`
	Method        string `json:"method" db:"method"` // cash/bank/alipay/wechat/offset
	OrderType     string `json:"order_type" db:"order_type"`
	OrderID       int64  `json:"order_id" db:"order_id"`
	OrderNo       string `json:"order_no" db:"order_no"`
	Status        int    `json:"status" db:"status"`
	Remark        string `json:"remark" db:"remark"`
	CreatedBy     int64  `json:"created_by" db:"created_by"`
	CreatedByName string `json:"created_by_name" db:"created_by_name"`
	ConfirmedAt   string `json:"confirmed_at" db:"confirmed_at"`
	CreatedAt     string `json:"created_at" db:"created_at"`
	UpdatedAt     string `json:"updated_at" db:"updated_at"`
}

// ---------------------------------------------------------------------------
// 报表
// ---------------------------------------------------------------------------

// InoutRow 进销存汇总行。
type InoutRow struct {
	Key        string `json:"key" db:"key"`
	Code       string `json:"code" db:"code"`
	Name       string `json:"name" db:"name"`
	Spec       string `json:"spec" db:"spec"`
	Unit       string `json:"unit" db:"unit"`
	InitQty    Qty    `json:"init_qty" db:"init_qty"`
	InitAmount Money  `json:"init_amount" db:"init_amount"`
	InQty      Qty    `json:"in_qty" db:"in_qty"`
	InAmount   Money  `json:"in_amount" db:"in_amount"`
	OutQty     Qty    `json:"out_qty" db:"out_qty"`
	OutAmount  Money  `json:"out_amount" db:"out_amount"`
	EndQty     Qty    `json:"end_qty" db:"end_qty"`
	EndAmount  Money  `json:"end_amount" db:"end_amount"`
}

// RankRow 排行行。
type RankRow struct {
	Key      string  `json:"key" db:"key"`
	Code     string  `json:"code" db:"code"`
	Name     string  `json:"name" db:"name"`
	Spec     string  `json:"spec" db:"spec"`
	Unit     string  `json:"unit" db:"unit"`
	Qty      Qty     `json:"qty" db:"qty"`
	Amount   Money   `json:"amount" db:"amount"`
	Cost     Money   `json:"cost" db:"cost"`
	Profit   Money   `json:"profit" db:"profit"`
	Rate     float64 `json:"rate" db:"rate"`
	DocCount int     `json:"doc_count" db:"doc_count"`
}

// ArApRow 应收应付行。
type ArApRow struct {
	PartnerID   int64  `json:"partner_id" db:"partner_id"`
	PartnerName string `json:"partner_name" db:"partner_name"`
	Contact     string `json:"contact" db:"contact"`
	Phone       string `json:"phone" db:"phone"`
	InitAmount  Money  `json:"init_amount" db:"init_amount"`
	DocAmount   Money  `json:"doc_amount" db:"doc_amount"`
	PaidAmount  Money  `json:"paid_amount" db:"paid_amount"`
	Balance     Money  `json:"balance" db:"balance"`
}

// DashboardSummary 首页看板数据。
type DashboardSummary struct {
	TodaySales    Money `json:"today_sales"`
	TodayPurchase Money `json:"today_purchase"`
	MonthSales    Money `json:"month_sales"`
	MonthPurchase Money `json:"month_purchase"`
	MonthProfit   Money `json:"month_profit"`
	Receivable    Money `json:"receivable"`
	Payable       Money `json:"payable"`
	StockAmount   Money `json:"stock_amount"`
	StockKinds    int   `json:"stock_kinds"`
	LowStockCount int   `json:"low_stock_count"`
	ProductCount  int   `json:"product_count"`
	PartnerCount  int   `json:"partner_count"`
	AccountTotal  Money `json:"account_total"`
}

// TrendPoint 走势点。
type TrendPoint struct {
	Date     string `json:"date"`
	Sales    Money  `json:"sales"`
	Purchase Money  `json:"purchase"`
	Profit   Money  `json:"profit"`
}

// Page 通用分页结果。
type Page[T any] struct {
	List  []T   `json:"list"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}
