package api

import (
	"fmt"
	"net/http"

	"mufish/internal/model"
	"mufish/internal/store"
)

// ---------------------------------------------------------------------------
// 库存
// ---------------------------------------------------------------------------

func (h *Handler) listStocks(w http.ResponseWriter, r *http.Request) error {
	page, err := h.st.ListStocks(r.Context(), store.StockQuery{
		Keyword:     qs(r, "keyword"),
		WarehouseID: qi64(r, "warehouse_id", 0),
		CategoryID:  qi64(r, "category_id", 0),
		OnlyLow:     qb(r, "low"),
		OnlyStock:   qb(r, "only_stock"),
		Page:        qi(r, "page", 1),
		Size:        qi(r, "size", 20),
	})
	if err != nil {
		return err
	}
	writeOK(w, page)
	return nil
}

func (h *Handler) stockSummary(w http.ResponseWriter, r *http.Request) error {
	data, err := h.st.StockSummary(r.Context(), qi64(r, "warehouse_id", 0), qi64(r, "category_id", 0))
	if err != nil {
		return err
	}
	writeOK(w, data)
	return nil
}

func (h *Handler) lowStocks(w http.ResponseWriter, r *http.Request) error {
	list, err := h.st.LowStockList(r.Context(), qi64(r, "warehouse_id", 0), qi(r, "limit", 100))
	if err != nil {
		return err
	}
	writeOK(w, list)
	return nil
}

func (h *Handler) stockLedger(w http.ResponseWriter, r *http.Request) error {
	page, err := h.st.ListStockMoves(r.Context(), store.StockLedgerQuery{
		Keyword:     qs(r, "keyword"),
		WarehouseID: qi64(r, "warehouse_id", 0),
		ProductID:   qi64(r, "product_id", 0),
		BizType:     qs(r, "biz_type"),
		From:        qs(r, "from"),
		To:          qs(r, "to"),
		Page:        qi(r, "page", 1),
		Size:        qi(r, "size", 20),
	})
	if err != nil {
		return err
	}
	writeOK(w, page)
	return nil
}

func (h *Handler) exportLedger(w http.ResponseWriter, r *http.Request) error {
	q := store.StockLedgerQuery{
		Keyword:     qs(r, "keyword"),
		WarehouseID: qi64(r, "warehouse_id", 0),
		ProductID:   qi64(r, "product_id", 0),
		BizType:     qs(r, "biz_type"),
		From:        qs(r, "from"),
		To:          qs(r, "to"),
		Size:        500,
	}
	rows := [][]string{{"业务日期", "记账时间", "仓库", "商品编码", "商品名称", "规格", "单位", "业务类型", "单据编号",
		"方向", "数量", "单价", "金额", "结存数量", "操作人", "备注"}}
	for pageNo := 1; ; pageNo++ {
		q.Page = pageNo
		page, err := h.st.ListStockMoves(r.Context(), q)
		if err != nil {
			return err
		}
		for _, m := range page.List {
			dir := "入库"
			if m.Direction < 0 {
				dir = "出库"
			}
			rows = append(rows, []string{
				m.BizDate, m.CreatedAt, m.WarehouseName, m.ProductCode, m.ProductName, m.Spec, m.Unit,
				model.BizTypeText(m.BizType), m.BizNo, dir, m.Qty.String(), m.Price.String(),
				m.Amount.String(), m.BalanceAfter.String(), m.CreatedByName, m.Remark,
			})
		}
		if len(page.List) < q.Size || int64(pageNo*q.Size) >= page.Total {
			break
		}
	}
	writeCSV(w, "库存流水", rows)
	return nil
}

// ---------------------------------------------------------------------------
// 盘点
// ---------------------------------------------------------------------------

func (h *Handler) listStockTakes(w http.ResponseWriter, r *http.Request) error {
	page, err := h.st.ListStockTakes(r.Context(), store.StockTakeQuery{
		Keyword:     qs(r, "keyword"),
		WarehouseID: qi64(r, "warehouse_id", 0),
		Status:      qi(r, "status", -1),
		From:        qs(r, "from"),
		To:          qs(r, "to"),
		Page:        qi(r, "page", 1),
		Size:        qi(r, "size", 20),
	})
	if err != nil {
		return err
	}
	writeOK(w, page)
	return nil
}

func (h *Handler) getStockTake(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	o, err := h.st.GetStockTake(r.Context(), id)
	if err != nil {
		return err
	}
	writeOK(w, o)
	return nil
}

func (h *Handler) saveStockTake(w http.ResponseWriter, r *http.Request) error {
	var in store.StockTakeInput
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
	newID, err := h.st.SaveStockTake(r.Context(), id, in, opUser(r))
	if err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "库存盘点", actionText(id),
		fmt.Sprintf("盘点单 #%d", newID), clientIP(r))
	writeOK(w, map[string]any{"id": newID})
	return nil
}

func (h *Handler) finishStockTake(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	o, err := h.st.GetStockTake(r.Context(), id)
	if err != nil {
		return err
	}
	if err := h.st.FinishStockTake(r.Context(), id, opUser(r)); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "库存盘点", "盘点完成",
		fmt.Sprintf("%s（差异 %s）", o.TakeNo, o.DiffQty), clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) voidStockTake(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.st.VoidStockTake(r.Context(), id, opUser(r)); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "库存盘点", "作废", fmt.Sprintf("#%d", id), clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) deleteStockTake(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.st.DeleteStockTake(r.Context(), id); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "库存盘点", "删除", fmt.Sprintf("#%d", id), clientIP(r))
	writeOK(w, nil)
	return nil
}

// ---------------------------------------------------------------------------
// 调拨
// ---------------------------------------------------------------------------

func (h *Handler) listTransfers(w http.ResponseWriter, r *http.Request) error {
	page, err := h.st.ListTransfers(r.Context(), store.TransferQuery{
		Keyword: qs(r, "keyword"),
		Status:  qi(r, "status", -1),
		From:    qs(r, "from"),
		To:      qs(r, "to"),
		Page:    qi(r, "page", 1),
		Size:    qi(r, "size", 20),
	})
	if err != nil {
		return err
	}
	writeOK(w, page)
	return nil
}

func (h *Handler) getTransfer(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	o, err := h.st.GetTransfer(r.Context(), id)
	if err != nil {
		return err
	}
	writeOK(w, o)
	return nil
}

func (h *Handler) saveTransfer(w http.ResponseWriter, r *http.Request) error {
	var in store.TransferInput
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
	newID, err := h.st.SaveTransfer(r.Context(), id, in, opUser(r))
	if err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "库存调拨", actionText(id),
		fmt.Sprintf("调拨单 #%d", newID), clientIP(r))
	writeOK(w, map[string]any{"id": newID})
	return nil
}

func (h *Handler) confirmTransfer(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	o, err := h.st.GetTransfer(r.Context(), id)
	if err != nil {
		return err
	}
	if err := h.st.ConfirmTransfer(r.Context(), id, opUser(r)); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "库存调拨", "确认",
		fmt.Sprintf("%s：%s → %s", o.TransferNo, o.FromName, o.ToName), clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) voidTransfer(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.st.VoidTransfer(r.Context(), id, opUser(r)); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "库存调拨", "作废", fmt.Sprintf("#%d", id), clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) deleteTransfer(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := h.st.DeleteTransfer(r.Context(), id); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "库存调拨", "删除", fmt.Sprintf("#%d", id), clientIP(r))
	writeOK(w, nil)
	return nil
}
