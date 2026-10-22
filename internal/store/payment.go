package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"mufish/internal/model"
)

// PaymentQuery 收付款单查询条件。
type PaymentQuery struct {
	Keyword   string
	Type      string
	PartnerID int64
	AccountID int64
	Status    int
	From      string
	To        string
	Page      int
	Size      int
}

const paymentCols = `o.id, o.payment_no, o.type, o.partner_id, o.partner_name, o.account_id, o.account_name,
	o.pay_date, o.amount, o.method, o.order_type, o.order_id, o.order_no, o.status, o.remark,
	o.created_by, o.created_by_name, o.confirmed_at, o.created_at, o.updated_at`

// ListPayments 分页查询收付款单。
func (s *Store) ListPayments(ctx context.Context, q PaymentQuery) (model.Page[model.Payment], error) {
	var out model.Page[model.Payment]
	where := []string{"1=1"}
	args := []any{}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		where = append(where, "(o.payment_no LIKE ? OR o.partner_name LIKE ? OR o.order_no LIKE ? OR o.remark LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like, like)
	}
	if q.Type != "" {
		where = append(where, "o.type=?")
		args = append(args, q.Type)
	}
	if q.PartnerID > 0 {
		where = append(where, "o.partner_id=?")
		args = append(args, q.PartnerID)
	}
	if q.AccountID > 0 {
		where = append(where, "o.account_id=?")
		args = append(args, q.AccountID)
	}
	if q.Status >= 0 && q.Status <= 2 {
		where = append(where, "o.status=?")
		args = append(args, q.Status)
	}
	if q.From != "" {
		where = append(where, "o.pay_date >= ?")
		args = append(args, q.From)
	}
	if q.To != "" {
		where = append(where, "o.pay_date <= ?")
		args = append(args, q.To)
	}
	cond := strings.Join(where, " AND ")
	total, err := countOf(ctx, s.db, `SELECT COUNT(*) FROM payments o WHERE `+cond, args...)
	if err != nil {
		return out, err
	}
	page, size := normPage(q.Page, q.Size)
	args2 := append(append([]any{}, args...), size, (page-1)*size)
	list, err := query[model.Payment](ctx, s.db,
		`SELECT `+paymentCols+` FROM payments o WHERE `+cond+` ORDER BY o.pay_date DESC, o.id DESC LIMIT ? OFFSET ?`, args2...)
	if err != nil {
		return out, err
	}
	return model.Page[model.Payment]{List: list, Total: total, Page: page, Size: size}, nil
}

// GetPayment 获取收付款单。
func (s *Store) GetPayment(ctx context.Context, id int64) (model.Payment, error) {
	p, ok, err := queryRow[model.Payment](ctx, s.db,
		`SELECT `+paymentCols+` FROM payments o WHERE o.id=?`, id)
	if err != nil {
		return p, err
	}
	if !ok {
		return p, ErrNotFound
	}
	return p, nil
}

// PaymentInput 收付款单入参。
type PaymentInput struct {
	Type      string      `json:"type"`
	PartnerID int64       `json:"partner_id"`
	AccountID int64       `json:"account_id"`
	PayDate   string      `json:"pay_date"`
	Amount    model.Money `json:"amount"`
	Method    string      `json:"method"`
	OrderType string      `json:"order_type"`
	OrderID   int64       `json:"order_id"`
	Remark    string      `json:"remark"`
}

// SavePayment 新建或修改收付款单。
func (s *Store) SavePayment(ctx context.Context, id int64, in PaymentInput, user OpUser) (int64, error) {
	switch in.Type {
	case model.PayTypeReceipt, model.PayTypePayment:
	default:
		return 0, fmt.Errorf("%w: 单据类型有误", ErrBadRequest)
	}
	if in.Amount <= 0 {
		return 0, fmt.Errorf("%w: 金额必须大于 0", ErrBadRequest)
	}
	switch in.Method {
	case "cash", "bank", "alipay", "wechat", "offset":
	default:
		in.Method = "bank"
	}
	in.PayDate = model.NormDate(in.PayDate)
	now := model.Now()

	err := s.Tx(ctx, func(tx *sql.Tx) error {
		partnerName, err := loadPartnerName(ctx, tx, in.PartnerID)
		if err != nil {
			return err
		}
		acc, ok, err := queryRow[model.Account](ctx, tx,
			`SELECT id, name FROM accounts WHERE id=?`, in.AccountID)
		if err != nil {
			return err
		}
		if !ok || acc.ID == 0 {
			return fmt.Errorf("%w: 请选择资金账户", ErrBadRequest)
		}
		orderNo := ""
		switch in.OrderType {
		case "purchase":
			if in.OrderID == 0 {
				in.OrderType = ""
				break
			}
			if in.Type != model.PayTypePayment {
				return fmt.Errorf("%w: 只有付款单才能关联采购单", ErrBadRequest)
			}
			var supplierID int64
			orderNo, supplierID, err = loadPurchaseRef(ctx, tx, in.OrderID)
			if err != nil {
				return err
			}
			if supplierID != in.PartnerID {
				return fmt.Errorf("%w: 付款单位与采购单供应商不一致", ErrBadRequest)
			}
		case "sales":
			if in.OrderID == 0 {
				in.OrderType = ""
				break
			}
			if in.Type != model.PayTypeReceipt {
				return fmt.Errorf("%w: 只有收款单才能关联销售单", ErrBadRequest)
			}
			var customerID int64
			orderNo, customerID, err = loadSalesRef(ctx, tx, in.OrderID)
			if err != nil {
				return err
			}
			if customerID != in.PartnerID {
				return fmt.Errorf("%w: 收款单位与销售单客户不一致", ErrBadRequest)
			}
		default:
			in.OrderType = ""
			in.OrderID = 0
		}

		if id > 0 {
			var status int
			if err := tx.QueryRowContext(ctx, `SELECT status FROM payments WHERE id=?`, id).Scan(&status); err != nil {
				if err == sql.ErrNoRows {
					return ErrNotFound
				}
				return err
			}
			if status != model.DocDraft {
				return fmt.Errorf("%w: 只有草稿状态的单据才能修改", ErrBadRequest)
			}
			_, err := tx.ExecContext(ctx,
				`UPDATE payments SET type=?, partner_id=?, partner_name=?, account_id=?, account_name=?,
				   pay_date=?, amount=?, method=?, order_type=?, order_id=?, order_no=?, remark=?, updated_at=?
				 WHERE id=?`,
				in.Type, in.PartnerID, partnerName, in.AccountID, acc.Name, in.PayDate, in.Amount,
				in.Method, in.OrderType, in.OrderID, orderNo, in.Remark, now, id)
			return wrapErr(err, "update payment")
		}

		prefix := "SK"
		if in.Type == model.PayTypePayment {
			prefix = "FK"
		}
		no, err := nextDocNo(ctx, tx, "payment_"+in.Type, prefix, in.PayDate)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO payments(payment_no, type, partner_id, partner_name, account_id, account_name,
			   pay_date, amount, method, order_type, order_id, order_no, status, remark,
			   created_by, created_by_name, created_at, updated_at)
			 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,0,?,?,?,?,?)`,
			no, in.Type, in.PartnerID, partnerName, in.AccountID, acc.Name, in.PayDate, in.Amount,
			in.Method, in.OrderType, in.OrderID, orderNo, in.Remark, user.ID, user.Name, now, now)
		if err != nil {
			return wrapErr(err, "insert payment")
		}
		id, _ = res.LastInsertId()
		return nil
	})
	return id, err
}

// ConfirmPayment 确认收付款：更新账户余额与关联单据已收 / 已付金额。
func (s *Store) ConfirmPayment(ctx context.Context, id int64, user OpUser) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		p, ok, err := queryRow[model.Payment](ctx, tx,
			`SELECT `+paymentCols+` FROM payments o WHERE o.id=?`, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if p.Status == model.DocConfirmed {
			return fmt.Errorf("%w: 该单据已确认", ErrBadRequest)
		}
		if p.Status == model.DocVoided {
			return fmt.Errorf("%w: 该单据已作废", ErrBadRequest)
		}

		// 关联单据校验：类型、状态和往来单位必须匹配，避免旧数据绕过保存时的校验。
		if p.OrderType != "" {
			if p.OrderID == 0 {
				return fmt.Errorf("%w: 关联单据无效", ErrBadRequest)
			}
			switch p.OrderType {
			case "sales":
				if p.Type != model.PayTypeReceipt {
					return fmt.Errorf("%w: 只有收款单才能关联销售单", ErrBadRequest)
				}
				var customerID int64
				var status int
				if err := tx.QueryRowContext(ctx,
					`SELECT customer_id, status FROM sales_orders WHERE id=?`, p.OrderID).
					Scan(&customerID, &status); err != nil {
					if err == sql.ErrNoRows {
						return fmt.Errorf("%w: 关联的销售单不存在", ErrBadRequest)
					}
					return err
				}
				if status != model.DocConfirmed {
					return fmt.Errorf("%w: 关联的销售单必须是「已出库」状态", ErrBadRequest)
				}
				if customerID != p.PartnerID {
					return fmt.Errorf("%w: 收款单位与销售单客户不一致", ErrBadRequest)
				}
			case "purchase":
				if p.Type != model.PayTypePayment {
					return fmt.Errorf("%w: 只有付款单才能关联采购单", ErrBadRequest)
				}
				var supplierID int64
				var status int
				if err := tx.QueryRowContext(ctx,
					`SELECT supplier_id, status FROM purchase_orders WHERE id=?`, p.OrderID).
					Scan(&supplierID, &status); err != nil {
					if err == sql.ErrNoRows {
						return fmt.Errorf("%w: 关联的采购单不存在", ErrBadRequest)
					}
					return err
				}
				if status != model.DocConfirmed {
					return fmt.Errorf("%w: 关联的采购单必须是「已入库」状态", ErrBadRequest)
				}
				if supplierID != p.PartnerID {
					return fmt.Errorf("%w: 付款单位与采购单供应商不一致", ErrBadRequest)
				}
			default:
				return fmt.Errorf("%w: 关联单据类型有误", ErrBadRequest)
			}
		}

		// 关联单据金额校验
		if p.OrderType == "sales" && p.OrderID > 0 && p.Type == model.PayTypeReceipt {
			var settle, received int64
			if err := tx.QueryRowContext(ctx,
				`SELECT settle_amount, received_amount FROM sales_orders WHERE id=?`, p.OrderID).Scan(&settle, &received); err != nil {
				return err
			}
			if received+int64(p.Amount) > settle {
				return fmt.Errorf("%w: 本次收款 %s 超过该销售单未收金额 %s",
					ErrBadRequest, p.Amount, model.Money(settle-received))
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE sales_orders SET received_amount = received_amount + ?, updated_at=? WHERE id=?`,
				p.Amount, model.Now(), p.OrderID); err != nil {
				return err
			}
		}
		if p.OrderType == "purchase" && p.OrderID > 0 && p.Type == model.PayTypePayment {
			var settle, paid int64
			if err := tx.QueryRowContext(ctx,
				`SELECT settle_amount, paid_amount FROM purchase_orders WHERE id=?`, p.OrderID).Scan(&settle, &paid); err != nil {
				return err
			}
			if paid+int64(p.Amount) > settle {
				return fmt.Errorf("%w: 本次付款 %s 超过该采购单未付金额 %s",
					ErrBadRequest, p.Amount, model.Money(settle-paid))
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE purchase_orders SET paid_amount = paid_amount + ?, updated_at=? WHERE id=?`,
				p.Amount, model.Now(), p.OrderID); err != nil {
				return err
			}
		}

		// 账户余额
		delta := int64(p.Amount)
		if p.Type == model.PayTypePayment {
			delta = -delta
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE accounts SET balance = balance + ?, updated_at=? WHERE id=?`,
			delta, model.Now(), p.AccountID); err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx,
			`UPDATE payments SET status=1, confirmed_at=?, updated_at=? WHERE id=?`,
			model.Now(), model.Now(), id)
		return err
	})
}

// VoidPayment 作废收付款单（已确认的回滚账户与单据金额）。
func (s *Store) VoidPayment(ctx context.Context, id int64, user OpUser) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		p, ok, err := queryRow[model.Payment](ctx, tx,
			`SELECT `+paymentCols+` FROM payments o WHERE o.id=?`, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if p.Status == model.DocVoided {
			return fmt.Errorf("%w: 该单据已作废", ErrBadRequest)
		}
		if p.Status == model.DocConfirmed {
			if p.OrderType == "sales" && p.OrderID > 0 && p.Type == model.PayTypeReceipt {
				if _, err := tx.ExecContext(ctx,
					`UPDATE sales_orders SET received_amount = MAX(received_amount - ?, 0), updated_at=? WHERE id=?`,
					p.Amount, model.Now(), p.OrderID); err != nil {
					return err
				}
			}
			if p.OrderType == "purchase" && p.OrderID > 0 && p.Type == model.PayTypePayment {
				if _, err := tx.ExecContext(ctx,
					`UPDATE purchase_orders SET paid_amount = MAX(paid_amount - ?, 0), updated_at=? WHERE id=?`,
					p.Amount, model.Now(), p.OrderID); err != nil {
					return err
				}
			}
			delta := -int64(p.Amount)
			if p.Type == model.PayTypePayment {
				delta = -delta
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE accounts SET balance = balance + ?, updated_at=? WHERE id=?`,
				delta, model.Now(), p.AccountID); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE payments SET status=2, updated_at=? WHERE id=?`, model.Now(), id)
		return err
	})
}

// DeletePayment 删除收付款单（仅草稿）。
func (s *Store) DeletePayment(ctx context.Context, id int64) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		var status int
		if err := tx.QueryRowContext(ctx, `SELECT status FROM payments WHERE id=?`, id).Scan(&status); err != nil {
			if err == sql.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		if status != model.DocDraft {
			return fmt.Errorf("%w: 只有草稿状态的单据才能删除", ErrBadRequest)
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM payments WHERE id=?`, id)
		return err
	})
}

// UnpaidOrders 查询某往来单位未结清的单据（用于收款 / 付款时选择）。
func (s *Store) UnpaidOrders(ctx context.Context, typ string, partnerID int64) ([]map[string]any, error) {
	out := []map[string]any{}
	if typ == "purchase" {
		rows, err := query[unpaidRow](ctx, s.db,
			`SELECT id, order_no AS no, order_date AS date, settle_amount, settle_amount - paid_amount AS balance
			 FROM purchase_orders WHERE supplier_id=? AND status=1 AND settle_amount > paid_amount
			 ORDER BY order_date, id`, partnerID)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, map[string]any{
				"id": r.ID, "no": r.No, "date": r.Date,
				"settle_amount": r.SettleAmount, "balance": r.Balance,
			})
		}
		return out, nil
	}
	rows, err := query[unpaidRow](ctx, s.db,
		`SELECT id, order_no AS no, order_date AS date, settle_amount, settle_amount - received_amount AS balance
		 FROM sales_orders WHERE customer_id=? AND status=1 AND settle_amount > received_amount
		 ORDER BY order_date, id`, partnerID)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "no": r.No, "date": r.Date,
			"settle_amount": r.SettleAmount, "balance": r.Balance,
		})
	}
	return out, nil
}

type unpaidRow struct {
	ID           int64       `db:"id"`
	No           string      `db:"no"`
	Date         string      `db:"date"`
	SettleAmount model.Money `db:"settle_amount"`
	Balance      model.Money `db:"balance"`
}

// loadPurchaseRef 读取采购单编号与供应商，只允许已入库单据被付款单关联。
func loadPurchaseRef(ctx context.Context, tx *sql.Tx, id int64) (string, int64, error) {
	var no string
	var status int
	var supplierID int64
	err := tx.QueryRowContext(ctx,
		`SELECT order_no, status, supplier_id FROM purchase_orders WHERE id=?`, id).
		Scan(&no, &status, &supplierID)
	if err == sql.ErrNoRows {
		return "", 0, fmt.Errorf("%w: 关联的采购单不存在", ErrBadRequest)
	}
	if err != nil {
		return "", 0, err
	}
	if status != model.DocConfirmed {
		return "", 0, fmt.Errorf("%w: 关联的采购单必须是「已入库」状态", ErrBadRequest)
	}
	return no, supplierID, nil
}

// loadSalesRef 读取销售单编号与客户，只允许已出库单据被收款单关联。
func loadSalesRef(ctx context.Context, tx *sql.Tx, id int64) (string, int64, error) {
	var no string
	var status int
	var customerID int64
	err := tx.QueryRowContext(ctx,
		`SELECT order_no, status, customer_id FROM sales_orders WHERE id=?`, id).
		Scan(&no, &status, &customerID)
	if err == sql.ErrNoRows {
		return "", 0, fmt.Errorf("%w: 关联的销售单不存在", ErrBadRequest)
	}
	if err != nil {
		return "", 0, err
	}
	if status != model.DocConfirmed {
		return "", 0, fmt.Errorf("%w: 关联的销售单必须是「已出库」状态", ErrBadRequest)
	}
	return no, customerID, nil
}
