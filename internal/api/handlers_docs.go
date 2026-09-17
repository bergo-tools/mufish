package api

import (
	"fmt"
	"net/http"

	"mufish/internal/store"
)

// ---------------------------------------------------------------------------
// 采购
// ---------------------------------------------------------------------------

func (h *Handler) listPurchases(w http.ResponseWriter, r *http.Request) error {
	page, err := h.st.ListPurchases(r.Context(), store.PurchaseQuery{
		Keyword:    qs(r, "keyword"),
		SupplierID: qi64(r, "supplier_id", 0),
		Status:     qi(r, "status", -1),
		From:       qs(r, "from"),
		To:         qs(r, "to"),
		Page:       qi(r, "page", 1),
		Size:       qi(r, "size", 20),
	})
	if err != nil {
		return err
	}
	writeOK(w, page)
	return nil
}

func (h *Handler) getPurchase(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	o, err := h.st.GetPurchase(r.Context(), id)
	if err != nil {
		return err
	}
	writeOK(w, o)
	return nil
}

func (h *Handler) savePurchase(w http.ResponseWriter, r *http.Request) error {
	var in store.PurchaseInput
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
	newID, err := h.st.SavePurchase(r.Context(), id, in, opUser(r))
	if err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "采购入库", actionText(id),
		fmt.Sprintf("采购单 #%d", newID), clientIP(r))
	writeOK(w, map[string]any{"id": newID})
	return nil
}

func (h *Handler) confirmPurchase(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	o, err := h.st.GetPurchase(r.Context(), id)
	if err != nil {
		return err
	}
	if err := h.st.ConfirmPurchase(r.Context(), id, opUser(r)); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "采购入库", "入库确认",
		fmt.Sprintf("%s（%s）", o.OrderNo, o.SupplierName), clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) voidPurchase(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	o, err := h.st.GetPurchase(r.Context(), id)
	if err != nil {
		return err
	}
	if err := h.st.VoidPurchase(r.Context(), id, opUser(r)); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "采购入库", "作废", o.OrderNo, clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) deletePurchase(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	o, err := h.st.GetPurchase(r.Context(), id)
	if err != nil {
		return err
	}
	if err := h.st.DeletePurchase(r.Context(), id); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "采购入库", "删除", o.OrderNo, clientIP(r))
	writeOK(w, nil)
	return nil
}

// ---------------------------------------------------------------------------
// 销售
// ---------------------------------------------------------------------------

func (h *Handler) listSales(w http.ResponseWriter, r *http.Request) error {
	page, err := h.st.ListSales(r.Context(), store.SalesQuery{
		Keyword:    qs(r, "keyword"),
		CustomerID: qi64(r, "customer_id", 0),
		Status:     qi(r, "status", -1),
		From:       qs(r, "from"),
		To:         qs(r, "to"),
		Page:       qi(r, "page", 1),
		Size:       qi(r, "size", 20),
	})
	if err != nil {
		return err
	}
	writeOK(w, page)
	return nil
}

func (h *Handler) getSales(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	o, err := h.st.GetSales(r.Context(), id)
	if err != nil {
		return err
	}
	writeOK(w, o)
	return nil
}

func (h *Handler) saveSales(w http.ResponseWriter, r *http.Request) error {
	var in store.SalesInput
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
	newID, err := h.st.SaveSales(r.Context(), id, in, opUser(r))
	if err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "销售出库", actionText(id),
		fmt.Sprintf("单据 #%d", newID), clientIP(r))
	writeOK(w, map[string]any{"id": newID})
	return nil
}

func (h *Handler) confirmSales(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	o, err := h.st.GetSales(r.Context(), id)
	if err != nil {
		return err
	}
	if err := h.st.ConfirmSales(r.Context(), id, opUser(r)); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "销售出库", "出库确认",
		fmt.Sprintf("%s（%s）", o.OrderNo, o.CustomerName), clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) voidSales(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	o, err := h.st.GetSales(r.Context(), id)
	if err != nil {
		return err
	}
	if err := h.st.VoidSales(r.Context(), id, opUser(r)); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "销售出库", "作废", o.OrderNo, clientIP(r))
	writeOK(w, nil)
	return nil
}

func (h *Handler) deleteSales(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	o, err := h.st.GetSales(r.Context(), id)
	if err != nil {
		return err
	}
	if err := h.st.DeleteSales(r.Context(), id); err != nil {
		return err
	}
	h.st.AddLog(r.Context(), user(r).ID, user(r).Username, "销售出库", "删除", o.OrderNo, clientIP(r))
	writeOK(w, nil)
	return nil
}
