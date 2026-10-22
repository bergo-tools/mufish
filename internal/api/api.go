// Package api 提供木鱼进销存的后端 HTTP 接口。
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mufish/internal/model"
	"mufish/internal/store"
)

// 上下文键。
type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxPerms
)

// Handler 聚合所有 HTTP 处理逻辑。
type Handler struct {
	st      *store.Store
	assets  fs.FS
	version string
	cookie  string
	dataDir string
	started time.Time
}

// Options 构造参数。
type Options struct {
	Store   *store.Store
	Assets  fs.FS
	Version string
	DataDir string
}

// New 创建 Handler。
func New(o Options) *Handler {
	return &Handler{
		st:      o.Store,
		assets:  o.Assets,
		version: o.Version,
		cookie:  "mufish_token",
		dataDir: o.DataDir,
		started: time.Now(),
	}
}

// handlerFunc 便于统一处理错误的处理函数签名。
type handlerFunc func(w http.ResponseWriter, r *http.Request) error

// route 注册一条路由。
type route struct {
	method  string
	pattern string
	perm    string
	fn      handlerFunc
}

// Router 构建路由表。
func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()

	routes := []route{
		// --- 认证 ---
		{"POST", "/api/v1/auth/login", "", h.login},
		{"POST", "/api/v1/auth/logout", "", h.logout},
		{"GET", "/api/v1/auth/me", "", h.me},
		{"POST", "/api/v1/auth/password", "", h.changePassword},
		{"GET", "/api/v1/meta", "", h.meta},

		// --- 看板 ---
		{"GET", "/api/v1/dashboard", "dashboard:view", h.dashboard},
		{"GET", "/api/v1/dashboard/trend", "dashboard:view", h.dashboardTrend},

		// --- 商品分类 ---
		{"GET", "/api/v1/categories", "category:view", h.listCategories},
		{"GET", "/api/v1/categories/flat", "category:view", h.allCategories},
		{"POST", "/api/v1/categories", "category:manage", h.saveCategory},
		{"PUT", "/api/v1/categories/{id}", "category:manage", h.saveCategory},
		{"DELETE", "/api/v1/categories/{id}", "category:manage", h.deleteCategory},

		// --- 商品 ---
		{"GET", "/api/v1/products", "product:view", h.listProducts},
		{"GET", "/api/v1/products/options", "", h.productOptions},
		{"GET", "/api/v1/products/export", "product:view", h.exportProducts},
		{"POST", "/api/v1/products/import", "product:import", h.importProducts},
		{"GET", "/api/v1/products/{id}", "product:view", h.getProduct},
		{"POST", "/api/v1/products", "product:create", h.saveProduct},
		{"PUT", "/api/v1/products/{id}", "product:update", h.saveProduct},
		{"DELETE", "/api/v1/products/{id}", "product:delete", h.deleteProduct},
		{"GET", "/api/v1/products/{id}/stocks", "stock:view", h.productStocks},

		// --- 往来单位 ---
		{"GET", "/api/v1/partners", "partner:view", h.listPartners},
		{"GET", "/api/v1/partners/options", "", h.partnerOptions},
		{"GET", "/api/v1/partners/{id}", "partner:view", h.getPartner},
		{"POST", "/api/v1/partners", "partner:create", h.savePartner},
		{"PUT", "/api/v1/partners/{id}", "partner:update", h.savePartner},
		{"DELETE", "/api/v1/partners/{id}", "partner:delete", h.deletePartner},

		// --- 仓库 ---
		{"GET", "/api/v1/warehouses", "warehouse:view", h.listWarehouses},
		{"GET", "/api/v1/warehouses/options", "", h.warehouseOptions},
		{"POST", "/api/v1/warehouses", "warehouse:manage", h.saveWarehouse},
		{"PUT", "/api/v1/warehouses/{id}", "warehouse:manage", h.saveWarehouse},
		{"DELETE", "/api/v1/warehouses/{id}", "warehouse:manage", h.deleteWarehouse},

		// --- 资金账户 ---
		{"GET", "/api/v1/accounts", "account:view", h.listAccounts},
		{"GET", "/api/v1/accounts/options", "", h.accountOptions},
		{"POST", "/api/v1/accounts", "account:manage", h.saveAccount},
		{"PUT", "/api/v1/accounts/{id}", "account:manage", h.saveAccount},
		{"DELETE", "/api/v1/accounts/{id}", "account:manage", h.deleteAccount},

		// --- 采购 ---
		{"GET", "/api/v1/purchases", "purchase:view", h.listPurchases},
		{"GET", "/api/v1/purchases/{id}", "purchase:view", h.getPurchase},
		{"POST", "/api/v1/purchases", "purchase:create", h.savePurchase},
		{"PUT", "/api/v1/purchases/{id}", "purchase:update", h.savePurchase},
		{"POST", "/api/v1/purchases/{id}/confirm", "purchase:confirm", h.confirmPurchase},
		{"POST", "/api/v1/purchases/{id}/void", "purchase:void", h.voidPurchase},
		{"DELETE", "/api/v1/purchases/{id}", "purchase:delete", h.deletePurchase},

		// --- 销售 ---
		{"GET", "/api/v1/sales", "sales:view", h.listSales},
		{"GET", "/api/v1/sales/{id}", "sales:view", h.getSales},
		{"POST", "/api/v1/sales", "sales:create", h.saveSales},
		{"PUT", "/api/v1/sales/{id}", "sales:update", h.saveSales},
		{"POST", "/api/v1/sales/{id}/confirm", "sales:confirm", h.confirmSales},
		{"POST", "/api/v1/sales/{id}/void", "sales:void", h.voidSales},
		{"DELETE", "/api/v1/sales/{id}", "sales:delete", h.deleteSales},

		// --- 库存 ---
		{"GET", "/api/v1/stocks", "stock:view", h.listStocks},
		{"GET", "/api/v1/stocks/summary", "stock:view", h.stockSummary},
		{"GET", "/api/v1/stocks/alerts", "stock:view", h.lowStocks},
		{"GET", "/api/v1/stocks/ledger", "stock:ledger", h.stockLedger},
		{"GET", "/api/v1/stocks/ledger/export", "stock:ledger", h.exportLedger},

		// --- 盘点 ---
		{"GET", "/api/v1/stock-takes", "stock:take", h.listStockTakes},
		{"GET", "/api/v1/stock-takes/{id}", "stock:take", h.getStockTake},
		{"POST", "/api/v1/stock-takes", "stock:take", h.saveStockTake},
		{"PUT", "/api/v1/stock-takes/{id}", "stock:take", h.saveStockTake},
		{"POST", "/api/v1/stock-takes/{id}/finish", "stock:take", h.finishStockTake},
		{"POST", "/api/v1/stock-takes/{id}/void", "stock:take", h.voidStockTake},
		{"DELETE", "/api/v1/stock-takes/{id}", "stock:take", h.deleteStockTake},

		// --- 调拨 ---
		{"GET", "/api/v1/transfers", "stock:transfer", h.listTransfers},
		{"GET", "/api/v1/transfers/{id}", "stock:transfer", h.getTransfer},
		{"POST", "/api/v1/transfers", "stock:transfer", h.saveTransfer},
		{"PUT", "/api/v1/transfers/{id}", "stock:transfer", h.saveTransfer},
		{"POST", "/api/v1/transfers/{id}/confirm", "stock:transfer", h.confirmTransfer},
		{"POST", "/api/v1/transfers/{id}/void", "stock:transfer", h.voidTransfer},
		{"DELETE", "/api/v1/transfers/{id}", "stock:transfer", h.deleteTransfer},

		// --- 收付款 ---
		{"GET", "/api/v1/payments", "payment:view", h.listPayments},
		{"GET", "/api/v1/payments/{id}", "payment:view", h.getPayment},
		{"POST", "/api/v1/payments", "payment:create", h.savePayment},
		{"PUT", "/api/v1/payments/{id}", "payment:create", h.savePayment},
		{"POST", "/api/v1/payments/{id}/confirm", "payment:confirm", h.confirmPayment},
		{"POST", "/api/v1/payments/{id}/void", "payment:void", h.voidPayment},
		{"DELETE", "/api/v1/payments/{id}", "payment:create", h.deletePayment},
		{"GET", "/api/v1/payments/unpaid-orders", "", h.unpaidOrders},

		// --- 报表 ---
		{"GET", "/api/v1/reports/inout", "report:view", h.reportInout},
		{"GET", "/api/v1/reports/inout/export", "report:export", h.exportInout},
		{"GET", "/api/v1/reports/sales-rank", "report:view", h.reportSalesRank},
		{"GET", "/api/v1/reports/purchase-rank", "report:view", h.reportPurchaseRank},
		{"GET", "/api/v1/reports/customer-rank", "report:view", h.reportCustomerRank},
		{"GET", "/api/v1/reports/supplier-rank", "report:view", h.reportSupplierRank},
		{"GET", "/api/v1/reports/profit", "report:view", h.reportProfit},
		{"GET", "/api/v1/reports/ar-ap", "report:view", h.reportArAp},
		{"GET", "/api/v1/reports/ar-ap/export", "report:export", h.exportArAp},
		{"GET", "/api/v1/reports/purchase-detail", "report:view", h.reportPurchaseDetail},
		{"GET", "/api/v1/reports/sales-detail", "report:view", h.reportSalesDetail},
		{"GET", "/api/v1/reports/cash-flow", "report:view", h.reportCashFlow},

		// --- 系统 ---
		{"GET", "/api/v1/users", "user:view", h.listUsers},
		{"POST", "/api/v1/users", "user:manage", h.createUser},
		{"PUT", "/api/v1/users/{id}", "user:manage", h.updateUser},
		{"DELETE", "/api/v1/users/{id}", "user:manage", h.deleteUser},
		{"POST", "/api/v1/users/{id}/reset-password", "user:manage", h.resetPassword},

		{"GET", "/api/v1/roles", "role:view", h.listRoles},
		{"GET", "/api/v1/permissions", "role:view", h.permissions},
		{"POST", "/api/v1/roles", "role:manage", h.createRole},
		{"PUT", "/api/v1/roles/{id}", "role:manage", h.updateRole},
		{"DELETE", "/api/v1/roles/{id}", "role:manage", h.deleteRole},

		{"GET", "/api/v1/logs", "log:view", h.listLogs},
		{"GET", "/api/v1/logs/modules", "log:view", h.logModules},

		{"GET", "/api/v1/settings", "setting:view", h.getSettings},
		{"PUT", "/api/v1/settings", "setting:manage", h.saveSettings},

		{"GET", "/api/v1/system/info", "", h.systemInfo},
		{"POST", "/api/v1/system/backup", "backup:run", h.backup},
		{"GET", "/api/v1/system/backups", "backup:run", h.listBackups},
		{"POST", "/api/v1/system/restore", "backup:run", h.restore},
	}

	for _, rt := range routes {
		// Go 1.22+ ServeMux 支持 "METHOD /path" 形式
		mux.Handle(rt.method+" "+rt.pattern, h.wrap(rt))
	}

	// 静态资源与 SPA 回退
	if h.assets != nil {
		mux.Handle("/", h.spaHandler())
	}
	return h.common(mux)
}

// wrap 组装单个路由的中间件。
func (h *Handler) wrap(rt route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 写操作要求自定义头，配合 SameSite Cookie 防跨站请求伪造
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.Header.Get("X-Requested-With") == "" && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				writeErr(w, http.StatusBadRequest, fmt.Errorf("非法的请求来源"))
				return
			}
		}
		var user model.User
		var perms model.PermSet
		if rt.perm != "" || !isPublic(rt.pattern) {
			u, p, err := h.currentUser(r)
			if err != nil {
				writeErr(w, http.StatusUnauthorized, errors.New("登录状态已失效，请重新登录"))
				return
			}
			user, perms = u, p
			if rt.perm != "" && !perms.Has(rt.perm) {
				writeErr(w, http.StatusForbidden, fmt.Errorf("没有操作权限：%s", rt.perm))
				return
			}
		} else {
			// 公开接口也尝试解析用户，便于返回个性化信息
			if u, p, err := h.currentUser(r); err == nil {
				user, perms = u, p
			}
		}
		ctx := context.WithValue(r.Context(), ctxUser, user)
		ctx = context.WithValue(ctx, ctxPerms, perms)
		if err := rt.fn(w, r.WithContext(ctx)); err != nil {
			writeErr(w, statusOf(err), err)
		}
	})
}

func isPublic(pattern string) bool {
	switch pattern {
	case "/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/meta":
		return true
	}
	return false
}

// common 全局中间件。
func (h *Handler) common(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				log.Printf("[panic] %s %s: %v", r.Method, r.URL.Path, p)
				writeErr(w, http.StatusInternalServerError, fmt.Errorf("服务器内部错误"))
			}
		}()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// currentUser 从 Cookie 或 Authorization 头解析登录用户。
func (h *Handler) currentUser(r *http.Request) (model.User, model.PermSet, error) {
	token := ""
	if c, err := r.Cookie(h.cookie); err == nil {
		token = c.Value
	}
	if token == "" {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if token == "" {
		return model.User{}, nil, errors.New("no token")
	}
	u, ok := h.st.SessionUser(r.Context(), token)
	if !ok {
		return model.User{}, nil, errors.New("invalid session")
	}
	perms, err := h.st.UserPerms(r.Context(), u)
	if err != nil {
		return model.User{}, nil, err
	}
	return u, perms, nil
}

// user 取出当前用户。
func user(r *http.Request) model.User {
	u, _ := r.Context().Value(ctxUser).(model.User)
	return u
}

// opUser 构造操作人。
func opUser(r *http.Request) store.OpUser {
	u := user(r)
	return store.OpUser{ID: u.ID, Name: displayName(u)}
}

func displayName(u model.User) string {
	if u.Name != "" {
		return u.Name
	}
	return u.Username
}

// ---------------------------------------------------------------------------
// 响应
// ---------------------------------------------------------------------------

type apiResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		log.Printf("[json] 编码失败: %v", err)
	}
}

func writeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, apiResponse{Code: 0, Msg: "ok", Data: data})
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, apiResponse{Code: status, Msg: err.Error()})
}

func statusOf(err error) int {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, store.ErrBadRequest):
		return http.StatusBadRequest
	case errors.Is(err, store.ErrDuplicate):
		return http.StatusConflict
	case errors.Is(err, store.ErrInUse):
		return http.StatusConflict
	}
	var se *store.Error
	if errors.As(err, &se) {
		return se.Status
	}
	return http.StatusInternalServerError
}

// ---------------------------------------------------------------------------
// 请求助手
// ---------------------------------------------------------------------------

// decodeJSON 解析请求体。
func decodeJSON(r *http.Request, dst any) error {
	defer io.Copy(io.Discard, r.Body)
	dec := json.NewDecoder(io.LimitReader(r.Body, 8<<20))
	if err := dec.Decode(dst); err != nil {
		if err == io.EOF {
			return fmt.Errorf("%w: 请求体为空", store.ErrBadRequest)
		}
		return fmt.Errorf("%w: 请求数据格式有误（%v）", store.ErrBadRequest, err)
	}
	return nil
}

func qs(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

func qi(r *http.Request, key string, def int) int {
	v := qs(r, key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func qi64(r *http.Request, key string, def int64) int64 {
	v := qs(r, key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func qb(r *http.Request, key string) bool {
	v := qs(r, key)
	return v == "1" || v == "true"
}

// pathID 读取路径中的 {id}。
func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%w: 无效的记录编号", store.ErrBadRequest)
	}
	return id, nil
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i > 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func nowStamp() string { return time.Now().Format("20060102-150405") }
