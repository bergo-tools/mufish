package api

import (
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"mufish/internal/model"
	"mufish/internal/store"
)

// meta 返回无需登录即可获取的站点信息（登录页使用）。
func (h *Handler) meta(w http.ResponseWriter, r *http.Request) error {
	settings, err := h.st.Settings(r.Context())
	if err != nil {
		return err
	}
	u := user(r)
	perms, _ := r.Context().Value(ctxPerms).(model.PermSet)
	defaultLogin := false
	if u.ID == 0 {
		// 仅在匿名访问登录页时校验，避免每次已登录页面加载都跑一次 PBKDF2。
		defaultLogin = h.st.DefaultAdminPasswordInUse(r.Context())
	}
	data := map[string]any{
		"version":        h.version,
		"company_name":   settings["company_name"],
		"company_short":  settings["company_short"],
		"doc_print_note": settings["doc_print_note"],
		"logged_in":      u.ID > 0,
		"default_login":  defaultLogin,
	}
	if u.ID > 0 {
		data["user"] = u
		data["permissions"] = perms.List()
	}
	writeOK(w, data)
	return nil
}

// dashboard 首页看板。
func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) error {
	summary, err := h.st.Dashboard(r.Context())
	if err != nil {
		return err
	}
	canSales := hasAnyPerm(r, "sales:view", "report:view")
	canPurchase := hasAnyPerm(r, "purchase:view", "report:view")
	canStock := hasAnyPerm(r, "stock:view", "report:view")
	canFinance := hasAnyPerm(r, "payment:view", "report:view")
	canAccount := hasAnyPerm(r, "account:view", "payment:view", "report:view")
	canProduct := hasAnyPerm(r, "product:view", "stock:view", "report:view")
	canPartner := hasAnyPerm(r, "partner:view", "report:view")

	// 看板是聚合页，但仍要遵守模块权限：没有对应权限时不把数据下发给前端。
	if !canSales {
		summary.TodaySales = 0
		summary.MonthSales = 0
		summary.MonthProfit = 0
	}
	if !canPurchase {
		summary.TodayPurchase = 0
		summary.MonthPurchase = 0
	}
	if !canStock {
		summary.StockAmount = 0
		summary.StockKinds = 0
		summary.LowStockCount = 0
	}
	if !canFinance {
		summary.Receivable = 0
		summary.Payable = 0
	}
	if !canAccount {
		summary.AccountTotal = 0
	}
	if !canProduct {
		summary.ProductCount = 0
	}
	if !canPartner {
		summary.PartnerCount = 0
	}

	trend := []model.TrendPoint{}
	if canSales || canPurchase {
		trend, err = h.st.Trend(r.Context(), 14)
		if err != nil {
			return err
		}
		for i := range trend {
			if !canSales {
				trend[i].Sales = 0
				trend[i].Profit = 0
			}
			if !canPurchase {
				trend[i].Purchase = 0
			}
		}
	}
	alerts := []model.Stock{}
	if canStock {
		alerts, err = h.st.LowStockList(r.Context(), 0, 8)
		if err != nil {
			return err
		}
	}
	type recent struct {
		ID      int64       `json:"id"`
		DocType string      `json:"doc_type"`
		DocNo   string      `json:"doc_no"`
		Partner string      `json:"partner"`
		Amount  model.Money `json:"amount"`
		Status  int         `json:"status"`
		Date    string      `json:"date"`
	}
	list := make([]recent, 0, 12)
	if canSales {
		recentSales, err := h.st.ListSales(r.Context(), store.SalesQuery{Status: -1, Page: 1, Size: 6})
		if err != nil {
			return err
		}
		for _, s := range recentSales.List {
			list = append(list, recent{s.ID, "sales", s.OrderNo, s.CustomerName, s.SettleAmount, s.Status, s.OrderDate})
		}
	}
	if canPurchase {
		recentPurchase, err := h.st.ListPurchases(r.Context(), store.PurchaseQuery{Status: -1, Page: 1, Size: 6})
		if err != nil {
			return err
		}
		for _, p := range recentPurchase.List {
			list = append(list, recent{p.ID, "purchase", p.OrderNo, p.SupplierName, p.SettleAmount, p.Status, p.OrderDate})
		}
	}
	// 销售和采购分别取最近 6 条后合并，必须再按业务日期统一排序，
	// 否则「最近单据」实际上只是「6 条销售 + 6 条采购」。
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Date != list[j].Date {
			return list[i].Date > list[j].Date
		}
		if list[i].ID != list[j].ID {
			return list[i].ID > list[j].ID
		}
		return list[i].DocType > list[j].DocType
	})
	writeOK(w, map[string]any{
		"summary": summary,
		"trend":   trend,
		"alerts":  alerts,
		"recent":  list,
		"visibility": map[string]bool{
			"sales":    canSales,
			"purchase": canPurchase,
			"stock":    canStock,
			"finance":  canFinance,
			"account":  canAccount,
			"product":  canProduct,
			"partner":  canPartner,
		},
	})
	return nil
}

// dashboardTrend 走势数据。
func (h *Handler) dashboardTrend(w http.ResponseWriter, r *http.Request) error {
	days := qi(r, "days", 14)
	trend, err := h.st.Trend(r.Context(), days)
	if err != nil {
		return err
	}
	// 与看板首页保持同一脱敏口径：没有对应模块查看权限时不能通过
	// 这个独立接口拿到销售/采购/毛利数据。
	canSales := hasAnyPerm(r, "sales:view", "report:view")
	canPurchase := hasAnyPerm(r, "purchase:view", "report:view")
	if !canSales && !canPurchase {
		writeOK(w, []model.TrendPoint{})
		return nil
	}
	for i := range trend {
		if !canSales {
			trend[i].Sales = 0
			trend[i].Profit = 0
		}
		if !canPurchase {
			trend[i].Purchase = 0
		}
	}
	writeOK(w, trend)
	return nil
}

// ---------------------------------------------------------------------------
// 分类
// ---------------------------------------------------------------------------

func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.ListCategories(r.Context())
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) allCategories(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.AllCategories(r.Context())
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) saveCategory(w http.ResponseWriter, r *http.Request) error {
	var in store.CategoryInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	id := int64(0)
	if r.Method == http.MethodPut {
		var err error
		if id, err = pathID(r); err != nil {
			return err
		}
	}
	newID, err := h.st.SaveCategory(r.Context(), id, in)
	if err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "商品分类",
		actionText(id), in.Name, clientIP(r))
	writeOK(w, map[string]any{"id": newID})
	return nil
}

func (h *Handler) deleteCategory(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.st.DeleteCategory(r.Context(), id); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "商品分类", "删除", fmt.Sprintf("#%d", id), clientIP(r))
	writeOK(w, nil)
	return nil
}

// ---------------------------------------------------------------------------
// 商品
// ---------------------------------------------------------------------------

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) error {
	page, err := h.st.ListProducts(r.Context(), store.ProductQuery{
		Keyword:    qs(r, "keyword"),
		CategoryID: qi64(r, "category_id", 0),
		Status:     qi(r, "status", -1),
		Page:       qi(r, "page", 1),
		Size:       qi(r, "size", 20),
	})
	if err != nil {
		return err
	}
	writeOK(w, page)
	return nil
}

func (h *Handler) productOptions(w http.ResponseWriter, r *http.Request) error {
	keyword := qs(r, "keyword")
	warehouseID := qi64(r, "warehouse_id", 0)
	var (
		list []model.Product
		err  error
	)
	if warehouseID > 0 {
		list, err = h.st.AllProductsForWarehouse(r.Context(), keyword, warehouseID)
	} else {
		list, err = h.st.AllProducts(r.Context(), keyword)
	}
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) getProduct(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	p, err := h.st.GetProduct(r.Context(), id)
	if err != nil {
		return err
	}
	stocks, err := h.st.ProductStockByWarehouse(r.Context(), id)
	if err != nil {
		return err
	}
	writeOK(w, map[string]any{"product": p, "stocks": stocks})
	return nil
}

func (h *Handler) saveProduct(w http.ResponseWriter, r *http.Request) error {
	var in store.ProductInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	id := int64(0)
	if r.Method == http.MethodPut {
		var err error
		if id, err = pathID(r); err != nil {
			return err
		}
	}
	newID, err := h.st.SaveProduct(r.Context(), id, in)
	if err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "商品资料", actionText(id),
		fmt.Sprintf("%s %s", in.Code, in.Name), clientIP(r))
	writeOK(w, map[string]any{"id": newID})
	return nil
}

func (h *Handler) deleteProduct(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	p, _ := h.st.GetProduct(r.Context(), id)
	if err := h.st.DeleteProduct(r.Context(), id); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "商品资料", "删除",
		fmt.Sprintf("%s %s", p.Code, p.Name), clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) productStocks(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	list, err := h.st.ProductStockByWarehouse(r.Context(), id)
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

// exportProducts 导出商品 CSV。
func (h *Handler) exportProducts(w http.ResponseWriter, r *http.Request) error {
	q := store.ProductQuery{
		Keyword:    qs(r, "keyword"),
		CategoryID: qi64(r, "category_id", 0),
		Status:     qi(r, "status", -1),
		Size:       500,
	}
	rows := [][]string{{"商品编码", "商品名称", "规格", "单位", "分类", "条码", "品牌", "参考进价", "参考售价", "安全库存", "当前库存", "状态"}}
	for pageNo := 1; ; pageNo++ {
		q.Page = pageNo
		page, err := h.st.ListProducts(r.Context(), q)
		if err != nil {
			return err
		}
		for _, p := range page.List {
			status := "启用"
			if p.Status != 1 {
				status = "停用"
			}
			rows = append(rows, []string{
				p.Code, p.Name, p.Spec, p.Unit, p.CategoryName, p.Barcode, p.Brand,
				p.PurchasePrice.String(), p.SalePrice.String(), p.SafetyStock.String(),
				p.StockQty.String(), status,
			})
		}
		if len(page.List) < q.Size || int64(pageNo*q.Size) >= page.Total {
			break
		}
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "商品资料", "导出", "导出商品清单", clientIP(r))
	writeCSV(w, "商品清单", rows)
	return nil
}

// importProducts 批量导入商品（CSV 文本）。
func (h *Handler) importProducts(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	text := strings.TrimPrefix(in.Text, "\ufeff")
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return store.NewError(http.StatusBadRequest, "CSV 解析失败："+err.Error())
	}
	if len(records) == 0 {
		return store.NewError(http.StatusBadRequest, "导入内容为空")
	}
	// 跳过表头
	if len(records) > 0 && len(records[0]) > 0 && strings.Contains(records[0][0], "编码") {
		records = records[1:]
	}
	cats, err := h.st.AllCategories(r.Context())
	if err != nil {
		return err
	}
	catIDs := make(map[string]int64, len(cats))
	for _, c := range cats {
		catIDs[strings.TrimSpace(c.Name)] = c.ID
	}
	okCount, failCount := 0, 0
	var messages []string
	fail := func(line int, format string, args ...any) {
		failCount++
		if len(messages) < 8 {
			messages = append(messages, fmt.Sprintf(format, args...))
		}
	}
	for i, rec := range records {
		if len(rec) < 2 || strings.TrimSpace(rec[1]) == "" {
			fail(i+2, "第 %d 行：商品名称不能为空", i+2)
			continue
		}
		get := func(idx int) string {
			if idx < len(rec) {
				return strings.TrimSpace(rec[idx])
			}
			return ""
		}
		purchasePrice, err := parsePrice(get(7))
		if err != nil {
			fail(i+2, "第 %d 行：参考进价%v", i+2, err)
			continue
		}
		salePrice, err := parsePrice(get(8))
		if err != nil {
			fail(i+2, "第 %d 行：参考售价%v", i+2, err)
			continue
		}
		safetyStock, err := parseQty(get(9))
		if err != nil {
			fail(i+2, "第 %d 行：安全库存%v", i+2, err)
			continue
		}
		input := store.ProductInput{
			Code:          get(0),
			Name:          get(1),
			Spec:          get(2),
			Unit:          defaultStr(get(3), "个"),
			Barcode:       get(5),
			Brand:         get(6),
			PurchasePrice: purchasePrice,
			SalePrice:     salePrice,
			SafetyStock:   safetyStock,
			Status:        1,
		}
		if input.Unit == "" {
			input.Unit = "个"
		}
		if categoryName := get(4); categoryName != "" {
			categoryID, ok := catIDs[categoryName]
			if !ok {
				fail(i+2, "第 %d 行：分类 %q 不存在", i+2, categoryName)
				continue
			}
			input.CategoryID = categoryID
		}
		if _, err := h.st.SaveProduct(r.Context(), 0, input); err != nil {
			fail(i+2, "第 %d 行：%v", i+2, err)
			continue
		}
		okCount++
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "商品资料", "导入",
		fmt.Sprintf("成功 %d 条，失败 %d 条", okCount, failCount), clientIP(r))
	writeOK(w, map[string]any{"success": okCount, "failed": failCount, "messages": messages})
	return nil
}

// ---------------------------------------------------------------------------
// 往来单位
// ---------------------------------------------------------------------------

func (h *Handler) listPartners(w http.ResponseWriter, r *http.Request) error {
	page, err := h.st.ListPartners(r.Context(), store.PartnerQuery{
		Keyword: qs(r, "keyword"),
		Type:    qs(r, "type"),
		Status:  qi(r, "status", -1),
		Page:    qi(r, "page", 1),
		Size:    qi(r, "size", 20),
	})
	if err != nil {
		return err
	}
	writeOK(w, page)
	return nil
}

func (h *Handler) partnerOptions(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.AllPartners(r.Context(), qs(r, "type"), qs(r, "keyword"))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) getPartner(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	p, err := h.st.GetPartner(r.Context(), id)
	if err != nil {
		return err
	}
	writeOK(w, p)
	return nil
}

func (h *Handler) savePartner(w http.ResponseWriter, r *http.Request) error {
	var in store.PartnerInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	id := int64(0)
	var before model.Partner
	if r.Method == http.MethodPut {
		var err error
		if id, err = pathID(r); err != nil {
			return err
		}
		if before, err = h.st.GetPartner(r.Context(), id); err != nil {
			return err
		}
	}
	newID, err := h.st.SavePartner(r.Context(), id, in)
	if err != nil {
		return err
	}
	detail := fmt.Sprintf("%s %s", in.Code, in.Name)
	if id > 0 {
		var changes []string
		if before.InitReceivable != in.InitReceivable {
			changes = append(changes, fmt.Sprintf("期初应收 %s → %s", before.InitReceivable, in.InitReceivable))
		}
		if before.InitPayable != in.InitPayable {
			changes = append(changes, fmt.Sprintf("期初应付 %s → %s", before.InitPayable, in.InitPayable))
		}
		if len(changes) > 0 {
			detail += "；" + strings.Join(changes, "；")
		}
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "往来单位", actionText(id),
		detail, clientIP(r))
	writeOK(w, map[string]any{"id": newID})
	return nil
}

func (h *Handler) deletePartner(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	p, _ := h.st.GetPartner(r.Context(), id)
	if err := h.st.DeletePartner(r.Context(), id); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "往来单位", "删除", p.Name, clientIP(r))
	writeOK(w, nil)
	return nil
}

// ---------------------------------------------------------------------------
// 仓库
// ---------------------------------------------------------------------------

func (h *Handler) listWarehouses(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.ListWarehouses(r.Context(), false)
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) warehouseOptions(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.ListWarehouses(r.Context(), true)
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) saveWarehouse(w http.ResponseWriter, r *http.Request) error {
	var in store.WarehouseInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	id := int64(0)
	if r.Method == http.MethodPut {
		var err error
		if id, err = pathID(r); err != nil {
			return err
		}
	}
	newID, err := h.st.SaveWarehouse(r.Context(), id, in)
	if err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "仓库管理", actionText(id), in.Name, clientIP(r))
	writeOK(w, map[string]any{"id": newID})
	return nil
}

func (h *Handler) deleteWarehouse(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.st.DeleteWarehouse(r.Context(), id); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "仓库管理", "删除", fmt.Sprintf("#%d", id), clientIP(r))
	writeOK(w, nil)
	return nil
}

// ---------------------------------------------------------------------------
// 资金账户
// ---------------------------------------------------------------------------

func (h *Handler) listAccounts(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.ListAccounts(r.Context(), false)
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) accountOptions(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.ListAccounts(r.Context(), true)
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) saveAccount(w http.ResponseWriter, r *http.Request) error {
	var in store.AccountInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	id := int64(0)
	if r.Method == http.MethodPut {
		var err error
		if id, err = pathID(r); err != nil {
			return err
		}
	}
	newID, err := h.st.SaveAccount(r.Context(), id, in)
	if err != nil {
		return err
	}
	// 余额变化不在这里处理：必须走 adjustAccount 生成独立调账记录。
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "资金账户", actionText(id), in.Name, clientIP(r))
	writeOK(w, map[string]any{"id": newID})
	return nil
}

// adjustAccount 调整账户余额并生成调账记录。
func (h *Handler) adjustAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in struct {
		Balance *model.Money `json:"balance"`
		Reason  string       `json:"reason"`
	}
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	if in.Balance == nil {
		return store.NewError(http.StatusBadRequest, "请填写调整后余额")
	}
	adj, err := h.st.AdjustAccount(r.Context(), id, *in.Balance, in.Reason, opUser(r))
	if err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "资金账户", "余额调账",
		fmt.Sprintf("%s：余额 %s → %s（原因：%s）",
			adj.AccountName, adj.BeforeBalance, adj.AfterBalance, adj.Reason),
		clientIP(r))
	writeOK(w, adj)
	return nil
}

// listAccountAdjustments 查询某账户的调账记录。
func (h *Handler) listAccountAdjustments(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	list, err := h.st.ListAccountAdjustments(r.Context(), id, qi(r, "limit", 100))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) deleteAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.st.DeleteAccount(r.Context(), id); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "资金账户", "删除", fmt.Sprintf("#%d", id), clientIP(r))
	writeOK(w, nil)
	return nil
}

// ---------------------------------------------------------------------------
// 助手
// ---------------------------------------------------------------------------

func actionText(id int64) string {
	if id > 0 {
		return "修改"
	}
	return "新增"
}

func defaultStr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}

func parsePrice(s string) (model.Price, error) {
	f, err := parseCSVNumber(s)
	if err != nil {
		return 0, err
	}
	return model.PriceFrom(f), nil
}

func parseQty(s string) (model.Qty, error) {
	f, err := parseCSVNumber(s)
	if err != nil {
		return 0, err
	}
	return model.QtyFrom(f), nil
}

// parseCSVNumber 解析 CSV 中的数字；空单元格视为 0，其余非法值必须报错，
// 不能静默变成 0，否则价格列写错会以 0 成本入库。
func parseCSVNumber(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("不是有效数字（%q）", s)
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("不是有效数字（%q）", s)
	}
	if f < 0 {
		return 0, fmt.Errorf("不能为负数（%q）", s)
	}
	return f, nil
}

// writeCSV 输出带 BOM 的 CSV（Excel 友好）。
func writeCSV(w http.ResponseWriter, filename string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		"attachment; filename="+filename+".csv; filename*=UTF-8''"+urlEncode(filename)+".csv")
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(w)
	for _, row := range rows {
		safe := make([]string, len(row))
		for i, cell := range row {
			safe[i] = csvSafe(cell)
		}
		if err := cw.Write(safe); err != nil {
			return
		}
	}
	cw.Flush()
}

// csvSafe 防止 Excel 把用户输入（商品名、备注等）当成公式执行；
// 纯负数金额不转义，避免报表里的数字变成文本。
func csvSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=':
		return "'" + s
	case '+':
		return "'" + s
	case '@':
		return "'" + s
	case '-':
		if _, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return s
		}
		return "'" + s
	case '\t', '\r', '\n':
		return "'" + s
	}
	return s
}

func urlEncode(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-' || c == '_' || c == '.' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}
