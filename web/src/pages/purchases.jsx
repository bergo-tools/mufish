// 采购入库页面：列表查询 + 新建/编辑 + 详情查看。
import { useEffect, useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import {
  amountCents,
  daysAgo,
  docStatus,
  fmtDate,
  fmtDateTime,
  fmtMoney,
  fmtQty,
  moneyFromCents,
  moneyToCents,
  sumAmountCents,
  today,
} from '../format.js';
import {
  Button,
  Card,
  DateRange,
  Descriptions,
  Field,
  Input,
  Modal,
  Money,
  NumberInput,
  Pagination,
  SearchBox,
  Select,
  Spinner,
  Table,
  Tag,
  Textarea,
  Toolbar,
  confirmDialog,
  useAsync,
  usePagedList,
} from '../ui.jsx';

// 状态下拉选项。
const STATUS_OPTIONS = [
  { value: '0', label: '草稿' },
  { value: '1', label: '已确认' },
  { value: '2', label: '已作废' },
];

// 行自增 key，避免 React/Preact 复用错行。
let rowSeq = 0;

// 新建一行明细。
function newRow() {
  return {
    key: ++rowSeq,
    product_id: '',
    product_code: '',
    product_name: '',
    spec: '',
    unit: '',
    qty: '1',
    price: '',
    remark: '',
  };
}

// 安全转数字。
function num(v) {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
}

// 单据状态标签。
function StatusTag({ status }) {
  const st = docStatus(status);
  return <Tag color={st.color}>{st.text}</Tag>;
}

export default function PurchasesPage({ route }) {
  const routeId = (route && route.query && route.query.id) || '';

  // 列表查询状态。
  const [keyword, setKeyword] = useState('');
  const { list, total, page, size, loading, filters, setFilters, setPage, setSize, reload } = usePagedList(
    (params) => Api.purchases(params),
    { keyword: '', status: '', from: daysAgo(30), to: today() },
    20,
  );

  // 新建 / 编辑弹窗。
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState(null);
  const [saving, setSaving] = useState(false);

  // 下拉选项。
  const [suppliers, setSuppliers] = useState([]);
  const [warehouses, setWarehouses] = useState([]);
  const [products, setProducts] = useState([]);
  const [productKw, setProductKw] = useState('');

  // 详情弹窗。
  const [detailId, setDetailId] = useState(null);

  // 加载供应商 / 仓库选项。
  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const [ps, ws] = await Promise.all([Api.partnerOptions('supplier'), Api.warehouseOptions()]);
        if (!alive) return;
        setSuppliers(ps || []);
        setWarehouses(ws || []);
      } catch (e) {
        if (alive) notifyError(e);
      }
    })();
    return () => {
      alive = false;
    };
  }, []);

  // 按关键字加载商品选项。
  const loadProducts = async (kw) => {
    try {
      const res = await Api.productOptions(kw || '');
      setProducts(res || []);
    } catch (e) {
      notifyError(e);
    }
  };

  // 打开新建/编辑弹窗时刷新一次商品选项。
  useEffect(() => {
    if (formOpen) loadProducts(productKw);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [formOpen]);

  // 详情数据（详情统一走 useAsync）。
  const detail = useAsync(() => (detailId ? Api.purchase(detailId) : Promise.resolve(null)), [detailId]);
  useEffect(() => {
    if (detail.error) notifyError(detail.error);
  }, [detail.error]);

  // 支持 #/purchases?id=3 直接打开单据详情。
  useEffect(() => {
    if (routeId) setDetailId(Number(routeId));
  }, [routeId]);

  // 明细行里出现过的商品（含后端返回但不在选项里的历史商品）。
  const productMap = new Map();
  products.forEach((p) => productMap.set(String(p.id), p));
  (form ? form.items : []).forEach((it) => {
    if (it.product_id && !productMap.has(String(it.product_id))) {
      productMap.set(String(it.product_id), {
        id: it.product_id,
        code: it.product_code,
        name: it.product_name,
        spec: it.spec,
        unit: it.unit,
      });
    }
  });
  const productOptions = Array.from(productMap.values()).map((p) => ({
    value: String(p.id),
    label: (p.code ? p.code + ' ' : '') + (p.name || ''),
  }));
  const supplierOptions = suppliers.map((p) => ({ value: String(p.id), label: (p.code ? p.code + ' ' : '') + p.name }));
  const warehouseOptions = warehouses.map((w) => ({ value: String(w.id), label: w.name }));

  // 打开新建。
  const openCreate = () => {
    const def = warehouses.find((w) => w.is_default === true || Number(w.is_default) === 1);
    setEditing(null);
    setProductKw('');
    setForm({
      supplier_id: '',
      warehouse_id: def ? String(def.id) : '',
      order_date: today(),
      discount_amount: '0',
      other_fee: '0',
      remark: '',
      items: [newRow()],
    });
    setFormOpen(true);
  };

  // 打开编辑：先拉取单据明细。
  const openEdit = async (row) => {
    try {
      const doc = await Api.purchase(row.id);
      setEditing(doc);
      setProductKw('');
      setForm({
        supplier_id: doc.supplier_id ? String(doc.supplier_id) : '',
        warehouse_id: doc.warehouse_id ? String(doc.warehouse_id) : '',
        order_date: doc.order_date ? fmtDate(doc.order_date) : today(),
        discount_amount: String(num(doc.discount_amount)),
        other_fee: String(num(doc.other_fee)),
        remark: doc.remark || '',
        items: (doc.items || []).length
          ? doc.items.map((it) => ({
              key: ++rowSeq,
              product_id: String(it.product_id),
              product_code: it.product_code || '',
              product_name: it.product_name || '',
              spec: it.spec || '',
              unit: it.unit || '',
              qty: String(num(it.qty)),
              price: String(num(it.price)),
              remark: it.remark || '',
            }))
          : [newRow()],
      });
      setDetailId(null);
      setFormOpen(true);
    } catch (e) {
      notifyError(e);
    }
  };

  const closeForm = () => {
    setFormOpen(false);
    setEditing(null);
    setForm(null);
  };

  // 明细操作。
  const updateItem = (idx, patch) => {
    setForm((f) => ({ ...f, items: f.items.map((it, i) => (i === idx ? { ...it, ...patch } : it)) }));
  };
  const addItem = () => setForm((f) => ({ ...f, items: [...f.items, newRow()] }));
  const removeItem = (idx) => setForm((f) => ({ ...f, items: f.items.filter((_, i) => i !== idx) }));
  const pickProduct = (idx, pid) => {
    const p = productMap.get(String(pid));
    const cur = form.items[idx] || {};
    const patch = { product_id: pid };
    if (p) {
      patch.product_code = p.code || '';
      patch.product_name = p.name || '';
      patch.spec = p.spec || '';
      patch.unit = p.unit || '';
      // 单价为空时自动带出参考进价。
      if (!String(cur.price === undefined || cur.price === null ? '' : cur.price).trim()) {
        patch.price = String(num(p.purchase_price));
      }
    }
    updateItem(idx, patch);
  };

  // 合计。
  const items = form ? form.items : [];
  const totalQty = items.reduce((s, it) => s + num(it.qty), 0);
  const totalCents = sumAmountCents(items);
  const totalAmount = moneyFromCents(totalCents);
  const settleCents = Math.max(
    0,
    totalCents - moneyToCents(form && form.discount_amount) + moneyToCents(form && form.other_fee),
  );
  const settleAmount = moneyFromCents(settleCents);

  // 保存。
  const submit = async () => {
    if (!form) return;
    if (!Number(form.supplier_id)) {
      notifyError('请选择供应商');
      return;
    }
    if (!Number(form.warehouse_id)) {
      notifyError('请选择仓库');
      return;
    }
    const rows = form.items.filter((it) => Number(it.product_id) > 0);
    if (rows.length === 0) {
      notifyError('请至少录入一行有效商品明细');
      return;
    }
    if (rows.some((it) => num(it.qty) <= 0)) {
      notifyError('明细行数量必须大于 0');
      return;
    }
    if (rows.some((it) => num(it.price) < 0)) {
      notifyError('明细行单价不能为负数');
      return;
    }
    if (num(form.discount_amount) < 0) {
      notifyError('优惠金额不能为负数');
      return;
    }
    if (num(form.other_fee) < 0) {
      notifyError('其他费用不能为负数');
      return;
    }
    const submittedCents = sumAmountCents(rows);
    if (moneyToCents(form.discount_amount) > submittedCents) {
      notifyError('优惠金额不能超过明细金额');
      return;
    }
    const payload = {
      supplier_id: Number(form.supplier_id),
      warehouse_id: Number(form.warehouse_id),
      order_date: form.order_date || today(),
      discount_amount: num(form.discount_amount),
      other_fee: num(form.other_fee),
      remark: form.remark || '',
      items: rows.map((it) => ({
        product_id: Number(it.product_id),
        qty: num(it.qty),
        price: num(it.price),
        remark: it.remark || '',
      })),
    };
    setSaving(true);
    try {
      await Api.savePurchase(payload, editing ? editing.id : undefined);
      notifyOK(editing ? '修改成功' : '新建成功');
      closeForm();
      reload();
    } catch (e) {
      notifyError(e);
    } finally {
      setSaving(false);
    }
  };

  // 确认入库。
  const doConfirm = async (row) => {
    const ok = await confirmDialog('确定要确认入库单据「' + row.order_no + '」吗？确认后将增加该仓库库存。');
    if (!ok) return;
    try {
      await Api.confirmPurchase(row.id);
      notifyOK('入库确认成功');
      reload();
      if (detailId) detail.reload();
    } catch (e) {
      notifyError(e);
    }
  };

  // 作废。
  const doVoid = async (row) => {
    const ok = await confirmDialog('确定要作废单据「' + row.order_no + '」吗？作废后将回退已入库数量。');
    if (!ok) return;
    try {
      await Api.voidPurchase(row.id);
      notifyOK('作废成功');
      reload();
      if (detailId) detail.reload();
    } catch (e) {
      notifyError(e);
    }
  };

  // 删除草稿。
  const doDelete = async (row) => {
    const ok = await confirmDialog('确定要删除草稿单据「' + row.order_no + '」吗？删除后不可恢复。');
    if (!ok) return;
    try {
      await Api.deletePurchase(row.id);
      notifyOK('删除成功');
      if (detailId === row.id) setDetailId(null);
      reload();
    } catch (e) {
      notifyError(e);
    }
  };

  // 列表操作列。
  const renderActions = (row) => {
    const btns = [];
    if (row.status === 0) {
      if (can('purchase:update')) {
        btns.push(
          <Button key="edit" size="sm" variant="link" onClick={() => openEdit(row)}>
            编辑
          </Button>,
        );
      }
      if (can('purchase:confirm')) {
        btns.push(
          <Button key="confirm" size="sm" variant="link" onClick={() => doConfirm(row)}>
            确认入库
          </Button>,
        );
      }
      if (can('purchase:delete')) {
        btns.push(
          <Button key="del" size="sm" variant="link" onClick={() => doDelete(row)}>
            删除
          </Button>,
        );
      }
    } else if (row.status === 1) {
      if (can('purchase:view')) {
        btns.push(
          <Button key="view" size="sm" variant="link" onClick={() => setDetailId(row.id)}>
            查看
          </Button>,
        );
      }
      if (can('purchase:void')) {
        btns.push(
          <Button key="void" size="sm" variant="link" onClick={() => doVoid(row)}>
            作废
          </Button>,
        );
      }
    } else if (can('purchase:view')) {
      btns.push(
        <Button key="view" size="sm" variant="link" onClick={() => setDetailId(row.id)}>
          查看
        </Button>,
      );
    }
    return <div class="row" style="gap:2px; flex-wrap:wrap; justify-content:center">{btns}</div>;
  };

  const detailData = detail.data;

  return (
    <div class="page">
      <Card
        title="采购入库单"
        icon="cart"
        bodyClass="no-pad"
        extra={
          can('purchase:create') ? (
            <Button variant="primary" size="sm" icon="plus" onClick={openCreate}>
              新建采购单
            </Button>
          ) : null
        }
      >
        <Toolbar
          right={
            <DateRange
              from={filters.from}
              to={filters.to}
              onFrom={(v) => setFilters({ from: v })}
              onTo={(v) => setFilters({ to: v })}
            />
          }
        >
          <SearchBox
            value={keyword}
            onInput={setKeyword}
            onSearch={() => setFilters({ keyword: keyword.trim() })}
            onClear={() => setFilters({ keyword: '' })}
            placeholder="单据号 / 供应商 / 仓库"
            width={230}
          />
          <div style="width:130px">
            <Select
              value={filters.status}
              onInput={(v) => setFilters({ status: v })}
              options={STATUS_OPTIONS}
              placeholder="全部状态"
            />
          </div>
          <Button size="sm" icon="search" onClick={() => setFilters({ keyword: keyword.trim() })}>
            查询
          </Button>
        </Toolbar>

        <Table
          columns={[
            { title: '单据编号', key: 'order_no', className: 'num nowrap' },
            { title: '供应商', render: (r) => r.supplier_name || '-' },
            { title: '仓库', render: (r) => r.warehouse_name || '-' },
            { title: '单据日期', width: 104, render: (r) => fmtDate(r.order_date) },
            { title: '数量', align: 'right', render: (r) => <span class="num">{fmtQty(r.total_qty)}</span> },
            { title: '金额', align: 'right', render: (r) => <Money value={r.total_amount} /> },
            { title: '已付', align: 'right', render: (r) => <Money value={r.paid_amount} /> },
            { title: '状态', align: 'center', width: 76, render: (r) => <StatusTag status={r.status} /> },
            { title: '制单人', render: (r) => r.created_by_name || '-' },
            { title: '操作', align: 'center', width: 190, render: renderActions },
          ]}
          data={list}
          loading={loading}
          rowKey="id"
          empty="暂无采购单"
        />

        <Pagination page={page} size={size} total={total} onPage={setPage} onSize={setSize} />
      </Card>

      {formOpen && form ? (
        <Modal
          open
          title={editing ? '编辑采购单 ' + editing.order_no : '新建采购单'}
          onClose={closeForm}
          closeOnMask={false}
          closeOnEsc={false}
          width={1000}
          bodyClass="no-pad"
          footer={
            <>
              <Button onClick={closeForm}>取消</Button>
              <Button variant="primary" icon="save" loading={saving} onClick={submit}>
                保存
              </Button>
            </>
          }
        >
          <div style="padding:14px">
            <div class="form-grid form-grid-3">
              <Field label="供应商" required>
                <Select
                  value={form.supplier_id}
                  onInput={(v) => setForm({ ...form, supplier_id: v })}
                  options={supplierOptions}
                  placeholder="请选择供应商"
                />
              </Field>
              <Field label="仓库" required>
                <Select
                  value={form.warehouse_id}
                  onInput={(v) => setForm({ ...form, warehouse_id: v })}
                  options={warehouseOptions}
                  placeholder="请选择仓库"
                />
              </Field>
              <Field label="单据日期">
                <Input
                  type="date"
                  value={form.order_date}
                  onInput={(v) => setForm({ ...form, order_date: v })}
                />
              </Field>
            </div>
          </div>

          <div class="row-between" style="padding:0 14px 10px">
            <div class="row">
              <SearchBox
                value={productKw}
                onInput={setProductKw}
                onSearch={() => loadProducts(productKw)}
                placeholder="搜索商品，回车刷新下拉"
                width={240}
              />
              <span class="muted small">商品下拉最多显示 300 条</span>
            </div>
            <Button size="sm" icon="plus" onClick={addItem}>
              添加一行
            </Button>
          </div>

          <div class="doc-items-table">
            <Table
              compact
              rowKey="key"
              empty="暂无明细，请点击「添加一行」"
              columns={[
                {
                  title: '商品',
                  width: 260,
                  render: (r, i) => (
                    <Select
                      value={r.product_id}
                      onInput={(v) => pickProduct(i, v)}
                      options={productOptions}
                      placeholder="选择商品"
                    />
                  ),
                },
                {
                  title: '数量',
                  width: 96,
                  render: (r, i) => (
                    <NumberInput value={r.qty} min="0" onInput={(v) => updateItem(i, { qty: v })} />
                  ),
                },
                {
                  title: '单价',
                  width: 104,
                  render: (r, i) => (
                    <NumberInput value={r.price} min="0" onInput={(v) => updateItem(i, { price: v })} />
                  ),
                },
                {
                  title: '金额',
                  width: 104,
                  align: 'right',
                  render: (r) => (
                    <span class="num">{fmtMoney(moneyFromCents(amountCents(r.qty, r.price)))}</span>
                  ),
                },
                {
                  title: '备注',
                  render: (r, i) => (
                    <Input value={r.remark} placeholder="备注" onInput={(v) => updateItem(i, { remark: v })} />
                  ),
                },
                {
                  title: '操作',
                  width: 58,
                  align: 'center',
                  render: (r, i) => (
                    <Button size="sm" variant="link" icon="trash" title="删除本行" onClick={() => removeItem(i)} />
                  ),
                },
              ]}
              data={form.items}
            />
          </div>

          <p class="muted small" style="padding:0 14px; margin-top:6px">
            采购优惠/费用是否计入库存成本由「系统参数 → 业务开关」控制，默认计入。
          </p>
          <div class="doc-total">
            <div class="doc-total-item">
              <span>合计数量</span>
              <strong>{fmtQty(totalQty)}</strong>
            </div>
            <div class="doc-total-item">
              <span>合计金额</span>
              <strong>{fmtMoney(totalAmount)}</strong>
            </div>
            <div class="doc-total-item">
              <span>优惠金额</span>
              <div style="width:110px">
                <NumberInput
                  value={form.discount_amount}
                  min="0"
                  onInput={(v) => setForm({ ...form, discount_amount: v })}
                />
              </div>
            </div>
            <div class="doc-total-item">
              <span>其他费用</span>
              <div style="width:110px">
                <NumberInput
                  value={form.other_fee}
                  min="0"
                  onInput={(v) => setForm({ ...form, other_fee: v })}
                />
              </div>
            </div>
            <div class="doc-total-item">
              <span>结算金额</span>
              <strong>{fmtMoney(settleAmount)}</strong>
            </div>
          </div>

          <div style="padding:14px">
            <Field label="备注">
              <Textarea value={form.remark} rows={2} onInput={(v) => setForm({ ...form, remark: v })} />
            </Field>
          </div>
        </Modal>
      ) : null}

      <Modal
        open={detailId !== null}
        title={detailData ? '采购单详情 ' + detailData.order_no : '采购单详情'}
        onClose={() => setDetailId(null)}
        width={1000}
        bodyClass="no-pad"
        footer={
          <>
            {detailData && detailData.status === 0 && can('purchase:update') ? (
              <Button icon="edit" onClick={() => openEdit(detailData)}>
                编辑
              </Button>
            ) : null}
            {detailData && detailData.status === 0 && can('purchase:confirm') ? (
              <Button variant="primary" icon="check" onClick={() => doConfirm(detailData)}>
                确认入库
              </Button>
            ) : null}
            {detailData && detailData.status === 1 && can('purchase:void') ? (
              <Button variant="danger" icon="ban" onClick={() => doVoid(detailData)}>
                作废
              </Button>
            ) : null}
            <Button onClick={() => setDetailId(null)}>关闭</Button>
          </>
        }
      >
        {detail.loading ? (
          <Spinner text="正在加载单据…" />
        ) : !detailData ? (
          <div style="padding:16px">
            <p class="muted">单据不存在或已被删除。</p>
          </div>
        ) : (
          <>
            <div style="padding:14px">
              <Descriptions
                columns={3}
                items={[
                  { label: '单据编号', value: <span class="num">{detailData.order_no}</span> },
                  { label: '供应商', value: detailData.supplier_name || '-' },
                  { label: '仓库', value: detailData.warehouse_name || '-' },
                  { label: '单据日期', value: fmtDate(detailData.order_date) },
                  { label: '状态', value: <StatusTag status={detailData.status} /> },
                  { label: '制单人', value: detailData.created_by_name || '-' },
                  { label: '制单时间', value: fmtDateTime(detailData.created_at) },
                  { label: '确认时间', value: fmtDateTime(detailData.confirmed_at) },
                  { label: '已付金额', value: <Money value={detailData.paid_amount} /> },
                ]}
              />
            </div>

            <div class="doc-items-table">
              <Table
                compact
                rowKey="id"
                empty="无明细"
                columns={[
                  { title: '商品编码', key: 'product_code', className: 'num nowrap' },
                  { title: '商品名称', render: (r) => r.product_name || '-' },
                  { title: '规格', render: (r) => r.spec || '-' },
                  { title: '单位', width: 66, render: (r) => r.unit || '-' },
                  { title: '数量', align: 'right', render: (r) => <span class="num">{fmtQty(r.qty)}</span> },
                  { title: '单价', align: 'right', render: (r) => <span class="num">{fmtMoney(r.price)}</span> },
                  { title: '金额', align: 'right', render: (r) => <Money value={r.amount} /> },
                  { title: '备注', render: (r) => r.remark || '-' },
                ]}
                data={detailData.items || []}
              />
            </div>

            <div class="doc-total">
              <div class="doc-total-item">
                <span>合计数量</span>
                <strong>{fmtQty(detailData.total_qty)}</strong>
              </div>
              <div class="doc-total-item">
                <span>合计金额</span>
                <strong>{fmtMoney(detailData.total_amount)}</strong>
              </div>
              <div class="doc-total-item">
                <span>优惠金额</span>
                <strong>{fmtMoney(detailData.discount_amount)}</strong>
              </div>
              <div class="doc-total-item">
                <span>其他费用</span>
                <strong>{fmtMoney(detailData.other_fee)}</strong>
              </div>
              <div class="doc-total-item">
                <span>结算金额</span>
                <strong>{fmtMoney(detailData.settle_amount)}</strong>
              </div>
              <div class="doc-total-item">
                <span>已付金额</span>
                <strong>{fmtMoney(detailData.paid_amount)}</strong>
              </div>
            </div>
          </>
        )}
      </Modal>
    </div>
  );
}
