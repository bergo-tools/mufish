package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
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
	data := map[string]any{
		"version":        h.version,
		"company_name":   settings["company_name"],
		"company_short":  settings["company_short"],
		"doc_print_note": settings["doc_print_note"],
		"logged_in":      u.ID > 0,
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
	trend, err := h.st.Trend(r.Context(), 14)
	if err != nil {
		return err
	}
	alerts, err := h.st.LowStockList(r.Context(), 0, 8)
	if err != nil {
		return err
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
	recentSales, err := h.st.ListSales(r.Context(), store.SalesQuery{Status: -1, Page: 1, Size: 6})
	if err != nil {
		return err
	}
	recentPurchase, err := h.st.ListPurchases(r.Context(), store.PurchaseQuery{Status: -1, Page: 1, Size: 6})
	if err != nil {
		return err
	}
	list := make([]recent, 0, 12)
	for _, s := range recentSales.List {
		list = append(list, recent{s.ID, "sales", s.OrderNo, s.CustomerName, s.SettleAmount, s.Status, s.OrderDate})
	}
	for _, p := range recentPurchase.List {
		list = append(list, recent{p.ID, "purchase", p.OrderNo, p.SupplierName, p.SettleAmount, p.Status, p.OrderDate})
	}
	writeOK(w, map[string]any{
		"summary": summary,
		"trend":   trend,
		"alerts":  alerts,
		"recent":  list,
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
	list, err := h.st.AllProducts(r.Context(), qs(r, "keyword"))
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
	page, err := h.st.ListProducts(r.Context(), store.ProductQuery{
		Keyword:    qs(r, "keyword"),
		CategoryID: qi64(r, "category_id", 0),
		Status:     qi(r, "status", -1),
		Page:       1,
		Size:       500,
	})
	if err != nil {
		return err
	}
	rows := [][]string{{"商品编码", "商品名称", "规格", "单位", "分类", "条码", "品牌", "参考进价", "参考售价", "安全库存", "当前库存", "状态"}}
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
	okCount, failCount := 0, 0
	var messages []string
	for i, rec := range records {
		if len(rec) < 2 || strings.TrimSpace(rec[1]) == "" {
			continue
		}
		get := func(idx int) string {
			if idx < len(rec) {
				return strings.TrimSpace(rec[idx])
			}
			return ""
		}
		input := store.ProductInput{
			Code:          get(0),
			Name:          get(1),
			Spec:          get(2),
			Unit:          defaultStr(get(3), "个"),
			Barcode:       get(5),
			Brand:         get(6),
			PurchasePrice: parsePrice(get(7)),
			SalePrice:     parsePrice(get(8)),
			SafetyStock:   parseQty(get(9)),
			Status:        1,
		}
		if input.Unit == "" {
			input.Unit = "个"
		}
		if _, err := h.st.SaveProduct(r.Context(), 0, input); err != nil {
			failCount++
			if len(messages) < 8 {
				messages = append(messages, fmt.Sprintf("第 %d 行：%v", i+2, err))
			}
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
	if r.Method == http.MethodPut {
		var err error
		if id, err = pathID(r); err != nil {
			return err
		}
	}
	newID, err := h.st.SavePartner(r.Context(), id, in)
	if err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "往来单位", actionText(id),
		fmt.Sprintf("%s %s", in.Code, in.Name), clientIP(r))
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
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "资金账户", actionText(id), in.Name, clientIP(r))
	writeOK(w, map[string]any{"id": newID})
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

func parsePrice(s string) model.Price {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return model.PriceFrom(f)
}

func parseQty(s string) model.Qty {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return model.QtyFrom(f)
}

// writeCSV 输出带 BOM 的 CSV（Excel 友好）。
func writeCSV(w http.ResponseWriter, filename string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		"attachment; filename="+filename+".csv; filename*=UTF-8''"+urlEncode(filename)+".csv")
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(w)
	_ = cw.WriteAll(rows)
	cw.Flush()
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
