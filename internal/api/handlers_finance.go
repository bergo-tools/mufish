package api

import (
	"fmt"
	"net/http"

	"mufish/internal/model"
	"mufish/internal/store"
)

// ---------------------------------------------------------------------------
// 收付款
// ---------------------------------------------------------------------------

func (h *Handler) listPayments(w http.ResponseWriter, r *http.Request) error {
	page, err := h.st.ListPayments(r.Context(), store.PaymentQuery{
		Keyword:   qs(r, "keyword"),
		Type:      qs(r, "type"),
		PartnerID: qi64(r, "partner_id", 0),
		AccountID: qi64(r, "account_id", 0),
		Status:    qi(r, "status", -1),
		From:      qs(r, "from"),
		To:        qs(r, "to"),
		Page:      qi(r, "page", 1),
		Size:      qi(r, "size", 20),
	})
	if err != nil {
		return err
	}
	writeOK(w, page)
	return nil
}

func (h *Handler) getPayment(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	p, err := h.st.GetPayment(r.Context(), id)
	if err != nil {
		return err
	}
	writeOK(w, p)
	return nil
}

func (h *Handler) savePayment(w http.ResponseWriter, r *http.Request) error {
	var in store.PaymentInput
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
	newID, err := h.st.SavePayment(r.Context(), id, in, opUser(r))
	if err != nil {
		return err
	}
	typeText := "收款单"
	if in.Type == model.PayTypePayment {
		typeText = "付款单"
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "收付款", actionText(id),
		fmt.Sprintf("%s #%d 金额 %s", typeText, newID, in.Amount), clientIP(r))
	writeOK(w, map[string]any{"id": newID})
	return nil
}

func (h *Handler) confirmPayment(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	p, err := h.st.GetPayment(r.Context(), id)
	if err != nil {
		return err
	}
	if err := h.st.ConfirmPayment(r.Context(), id, opUser(r)); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "收付款", "确认",
		fmt.Sprintf("%s %s", p.PaymentNo, p.Amount), clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) voidPayment(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	p, err := h.st.GetPayment(r.Context(), id)
	if err != nil {
		return err
	}
	if err := h.st.VoidPayment(r.Context(), id, opUser(r)); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "收付款", "作废", p.PaymentNo, clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) deletePayment(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	p, err := h.st.GetPayment(r.Context(), id)
	if err != nil {
		return err
	}
	if err := h.st.DeletePayment(r.Context(), id); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "收付款", "删除", p.PaymentNo, clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) unpaidOrders(w http.ResponseWriter, r *http.Request) error {
	typ := qs(r, "type")
	if typ != "purchase" {
		typ = "sales"
	}
	list, err := h.st.UnpaidOrders(r.Context(), typ, qi64(r, "partner_id", 0))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

// ---------------------------------------------------------------------------
// 报表
// ---------------------------------------------------------------------------

func (h *Handler) reportInout(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.InoutSummary(r.Context(), h.inoutQuery(r))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) inoutQuery(r *http.Request) store.InoutQuery {
	return store.InoutQuery{
		From:        qs(r, "from"),
		To:          qs(r, "to"),
		WarehouseID: qi64(r, "warehouse_id", 0),
		CategoryID:  qi64(r, "category_id", 0),
		Keyword:     qs(r, "keyword"),
	}
}

func (h *Handler) exportInout(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.InoutSummary(r.Context(), h.inoutQuery(r))
	if err != nil {
		return err
	}
	rows := [][]string{{"商品编码", "商品名称", "规格", "单位", "期初数量", "期初金额",
		"入库数量", "入库金额", "出库数量", "出库金额", "期末数量", "期末金额"}}
	for _, x := range list {
		rows = append(rows, []string{
			x.Code, x.Name, x.Spec, x.Unit,
			x.InitQty.String(), x.InitAmount.String(),
			x.InQty.String(), x.InAmount.String(),
			x.OutQty.String(), x.OutAmount.String(),
			x.EndQty.String(), x.EndAmount.String(),
		})
	}
	writeCSV(w, "进销存汇总", rows)
	return nil
}

func (h *Handler) rankQuery(r *http.Request) store.RankQuery {
	return store.RankQuery{
		From:  qs(r, "from"),
		To:    qs(r, "to"),
		Limit: qi(r, "limit", 50),
	}
}

func (h *Handler) reportSalesRank(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.SalesRank(r.Context(), h.rankQuery(r))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) reportPurchaseRank(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.PurchaseRank(r.Context(), h.rankQuery(r))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) reportCustomerRank(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.CustomerRank(r.Context(), h.rankQuery(r))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) reportSupplierRank(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.SupplierRank(r.Context(), h.rankQuery(r))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) reportProfit(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.ProfitByProduct(r.Context(), h.rankQuery(r))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) reportArAp(w http.ResponseWriter, r *http.Request) error {
	typ := qs(r, "type")
	if typ != "payable" {
		typ = "receivable"
	}
	list, err := h.st.ArApReport(r.Context(), store.ArApQuery{Type: typ, Keyword: qs(r, "keyword")})
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) exportArAp(w http.ResponseWriter, r *http.Request) error {
	typ := qs(r, "type")
	if typ != "payable" {
		typ = "receivable"
	}
	list, err := h.st.ArApReport(r.Context(), store.ArApQuery{Type: typ, Keyword: qs(r, "keyword")})
	if err != nil {
		return err
	}
	title := "应收对账表"
	if typ == "payable" {
		title = "应付对账表"
	}
	rows := [][]string{{"单位名称", "联系人", "电话", "期初金额", "本期发生", "已结算", "期末余额"}}
	for _, x := range list {
		rows = append(rows, []string{
			x.PartnerName, x.Contact, x.Phone,
			x.InitAmount.String(), x.DocAmount.String(), x.PaidAmount.String(), x.Balance.String(),
		})
	}
	writeCSV(w, title, rows)
	return nil
}

func (h *Handler) detailQuery(r *http.Request) store.DocDetailQuery {
	return store.DocDetailQuery{
		From:        qs(r, "from"),
		To:          qs(r, "to"),
		WarehouseID: qi64(r, "warehouse_id", 0),
		PartnerID:   qi64(r, "partner_id", 0),
		ProductID:   qi64(r, "product_id", 0),
		Keyword:     qs(r, "keyword"),
		Limit:       qi(r, "limit", 1000),
	}
}

func (h *Handler) reportPurchaseDetail(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.PurchaseDetail(r.Context(), h.detailQuery(r))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) reportSalesDetail(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.SalesDetail(r.Context(), h.detailQuery(r))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) reportCashFlow(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.CashFlow(r.Context(), qs(r, "from"), qs(r, "to"),
		qi64(r, "account_id", 0), qi(r, "limit", 500))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}
