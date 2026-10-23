// 库存盘点：盘点单查询、实盘录入、完成与作废。
import { useRef, useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import { amountCents, fmtDate, fmtDateTime, fmtQty, moneyFromCents, today } from '../format.js';
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
  Table,
  Tag,
  Toolbar,
  confirmDialog,
  useAsync,
  usePagedList,
} from '../ui.jsx';

// 盘点单状态。
const TAKE_STATUS = {
  0: { text: '草稿', color: 'gray' },
  1: { text: '已完成', color: 'green' },
  2: { text: '已作废', color: 'red' },
};

// 状态下拉选项。
const STATUS_OPTIONS = [
  { value: '0', label: '草稿' },
  { value: '1', label: '已完成' },
  { value: '2', label: '已作废' },
];

// 分页请求包装：失败时统一提示。
function wrapList(fn) {
  return async (params) => {
    try {
      return await fn(params);
    } catch (e) {
      notifyError(e);
      throw e;
    }
  };
}

function takeStatus(status) {
  return TAKE_STATUS[status] || { text: '未知', color: 'gray' };
}

// 差异数量：正数绿色、负数红色。
function DiffQty({ value }) {
  const n = Number(value || 0);
  if (n === 0) return <span class="num">0</span>;
  return (
    <span class={n > 0 ? 'money-pos' : 'money-neg'}>
      {n > 0 ? '+' : ''}
      {fmtQty(n)}
    </span>
  );
}

export default function StockTakesPage({ route }) {
  const [keyword, setKeyword] = useState('');

  const { list, total, page, size, loading, filters, setFilters, setPage, setSize, reload } =
    usePagedList(wrapList(Api.stockTakes), { keyword: '', warehouse_id: '', status: '', from: '', to: '' }, 20);

  // 仓库下拉。
  const warehouses = useAsync(async () => {
    try {
      return (await Api.warehouseOptions()) || [];
    } catch (e) {
      notifyError(e);
      throw e;
    }
  }, []);
  const warehouseOptions = (warehouses.data || []).map((w) => ({ value: String(w.id), label: w.name }));

  // 新建 / 编辑弹窗。
  const [editOpen, setEditOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ id: 0, warehouse_id: '', take_date: today(), remark: '' });
  const [rows, setRows] = useState([]);
  const [bookLoading, setBookLoading] = useState(false);
  // 盘点明细请求序号：快速切换仓库时只接受最后一次请求的结果。
  const bookReqSeq = useRef(0);
  // 盘盈/零库存商品需要单独添加，不能只依赖 stocks 表。
  const [productKw, setProductKw] = useState('');
  const [productResults, setProductResults] = useState([]);
  const [pickedProductId, setPickedProductId] = useState('');

  // 详情弹窗。
  const [viewOpen, setViewOpen] = useState(false);
  const [viewData, setViewData] = useState(null);

  // 拉取账面库存作为盘点明细。
  //
  // existingRows：同一仓库刷新时传入当前明细。账面仍有库存的商品会更新
  // 账面数量但保留实盘数/备注；账面已为 0 的手工行继续保留，避免刷新把
  // 盘盈或零库存商品丢掉。
  // clearOnError：切换仓库时必须为 true。旧仓库明细不能留在新仓库表单里，
  // 否则保存会把旧仓库商品按实盘数写入新仓库。
  const loadBook = async (warehouseId, existingRows, clearOnError) => {
    const seq = ++bookReqSeq.current;
    if (!warehouseId) {
      setRows([]);
      setBookLoading(false);
      return;
    }
    setBookLoading(true);
    try {
      // 后端单页最多 500 条，必须分页拉全，否则盘点会静默漏掉商品。
      const stockList = [];
      for (let page = 1; ; page++) {
        const res = await Api.stocks({ warehouse_id: warehouseId, page, size: 500, only_stock: 1 });
        const batch = (res && res.list) || [];
        stockList.push(...batch);
        const total = Number((res && res.total) || 0);
        if (batch.length < 500 || (total > 0 && stockList.length >= total) || page >= 100) break;
      }
      if (seq !== bookReqSeq.current) return;
      const existing = new Map(
        (existingRows || []).map((r) => [String(r.product_id), r]),
      );
      const seen = new Set();
      const next = stockList.map((s) => {
        seen.add(String(s.product_id));
        const kept = existing.get(String(s.product_id)) || null;
        const bookQty = Number(s.qty || 0);
        const hasReal =
          kept && kept.real_qty !== undefined && kept.real_qty !== null && kept.real_qty !== '';
        return {
          product_id: s.product_id,
          product_code: s.product_code,
          product_name: s.product_name,
          spec: s.spec,
          unit: s.unit,
          book_qty: bookQty,
          real_qty: hasReal ? kept.real_qty : bookQty,
          price: Number(s.cost_price || 0),
          remark: kept && kept.remark ? kept.remark : '',
        };
      });
      // 已有但当前仓库账面为 0 的明细（手工添加的盘盈/零库存商品）继续保留。
      (existingRows || []).forEach((r) => {
        if (!seen.has(String(r.product_id))) {
          next.push({ ...r, book_qty: 0 });
        }
      });
      setRows(next);
    } catch (e) {
      if (seq !== bookReqSeq.current) return;
      if (clearOnError) setRows([]);
      notifyError(e);
    } finally {
      if (seq === bookReqSeq.current) setBookLoading(false);
    }
  };

  const openCreate = () => {
    const wid = filters.warehouse_id || '';
    setForm({ id: 0, warehouse_id: wid, take_date: today(), remark: '' });
    setRows([]);
    setEditOpen(true);
    loadBook(wid, [], true);
  };

  const openEdit = async (row) => {
    try {
      const detail = await Api.stockTake(row.id);
      // 使仍在外面的账面库存请求失效，避免迟到响应覆盖编辑中的明细。
      bookReqSeq.current += 1;
      setBookLoading(false);
      setForm({
        id: detail.id,
        warehouse_id: String(detail.warehouse_id || ''),
        take_date: detail.take_date || today(),
        remark: detail.remark || '',
      });
      setRows(
        (detail.items || []).map((it) => ({
          product_id: it.product_id,
          product_code: it.product_code,
          product_name: it.product_name,
          spec: it.spec,
          unit: it.unit,
          book_qty: Number(it.book_qty || 0),
          real_qty: Number(it.real_qty || 0),
          price: Number(it.price || 0),
          remark: it.remark || '',
        })),
      );
      setEditOpen(true);
    } catch (e) {
      notifyError(e);
    }
  };

  const openView = async (row) => {
    try {
      const detail = await Api.stockTake(row.id);
      setViewData(detail);
      setViewOpen(true);
    } catch (e) {
      notifyError(e);
    }
  };

  // 切换仓库时重新拉取账面库存。旧仓库明细必须立即清空，不能等待
  // 新请求返回，也不能在失败时保留。
  const handleWarehouseChange = (v) => {
    setForm((f) => ({ ...f, warehouse_id: v }));
    setRows([]);
    loadBook(v, [], true);
  };

  const updateRow = (index, patch) => {
    setRows((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch } : r)));
  };

  const searchProducts = async () => {
    try {
      setProductResults((await Api.productOptions(productKw || '')) || []);
    } catch (e) {
      notifyError(e);
    }
  };

  const addProduct = () => {
    const p = productResults.find((x) => String(x.id) === String(pickedProductId));
    if (!p) {
      notifyError(new Error('请先搜索并选择商品'));
      return;
    }
    if (rows.some((r) => String(r.product_id) === String(p.id))) {
      notifyError(new Error('该商品已在盘点明细中'));
      return;
    }
    setRows((prev) => [
      ...prev,
      {
        product_id: p.id,
        product_code: p.code || '',
        product_name: p.name || '',
        spec: p.spec || '',
        unit: p.unit || '',
        book_qty: 0,
        real_qty: 0,
        price: Number(p.purchase_price || 0),
        remark: '',
      },
    ]);
    setPickedProductId('');
  };

  // 明细合计。
  const totalDiffQty = rows.reduce((a, r) => a + (Number(r.real_qty || 0) - Number(r.book_qty || 0)), 0);
  const totalDiffCents = rows.reduce(
    (a, r) => a + amountCents(Number(r.real_qty || 0) - Number(r.book_qty || 0), r.price),
    0,
  );
  const totalDiffAmount = moneyFromCents(totalDiffCents);

  const handleSave = async () => {
    if (!form.warehouse_id) {
      notifyError(new Error('请选择盘点仓库'));
      return;
    }
    if (bookLoading) {
      notifyError(new Error('账面库存正在加载，请稍候再保存'));
      return;
    }
    if (rows.length === 0) {
      notifyError(new Error('盘点明细为空，请先选择仓库并刷新账面库存'));
      return;
    }
    setSaving(true);
    try {
      await Api.saveStockTake(
        {
          warehouse_id: Number(form.warehouse_id),
          take_date: form.take_date || today(),
          remark: form.remark,
          items: rows.map((r) => ({
            product_id: r.product_id,
            real_qty: Number(r.real_qty || 0),
            remark: r.remark || '',
          })),
        },
        form.id || undefined,
      );
      notifyOK(form.id ? '保存成功' : '新建成功');
      setEditOpen(false);
      reload();
    } catch (e) {
      notifyError(e);
    } finally {
      setSaving(false);
    }
  };

  // 完成盘点：按实盘数调整库存。
  const handleFinish = async (row) => {
    const ok = await confirmDialog(
      `确认完成盘点单「${row.take_no}」？完成后将按实盘数量调整库存，单据不可再修改。`,
      { title: '完成盘点' },
    );
    if (!ok) return;
    try {
      await Api.finishStockTake(row.id);
      notifyOK('完成盘点成功');
      setViewOpen(false);
      reload();
    } catch (e) {
      notifyError(e);
    }
  };

  const handleVoid = async (row) => {
    const ok = await confirmDialog(`确认作废盘点单「${row.take_no}」？已完成盘点将自动回滚库存调整。`, {
      title: '作废盘点单',
    });
    if (!ok) return;
    try {
      await Api.voidStockTake(row.id);
      notifyOK('作废成功');
      setViewOpen(false);
      reload();
    } catch (e) {
      notifyError(e);
    }
  };

  const handleDelete = async (row) => {
    const ok = await confirmDialog(`确认删除盘点单「${row.take_no}」？删除后不可恢复。`, { title: '删除盘点单' });
    if (!ok) return;
    try {
      await Api.deleteStockTake(row.id);
      notifyOK('删除成功');
      reload();
    } catch (e) {
      notifyError(e);
    }
  };

  const doSearch = () => setFilters({ keyword });

  return (
    <div class="page">
      <Card
        title="盘点单列表"
        icon="clipboard"
        bodyClass="no-pad"
        extra={<span class="muted small">共 {total} 条记录</span>}
      >
        <Toolbar
          right={
            <>
              <Button icon="search" variant="primary" onClick={doSearch}>
                查询
              </Button>
              <Button
                icon="refresh"
                onClick={() => {
                  setKeyword('');
                  setFilters({ keyword: '', warehouse_id: '', status: '', from: '', to: '' });
                }}
              >
                重置
              </Button>
              {can('stock:take') ? (
                <Button icon="plus" variant="primary" onClick={openCreate}>
                  新建盘点
                </Button>
              ) : null}
            </>
          }
        >
          <SearchBox
            value={keyword}
            onInput={setKeyword}
            onSearch={doSearch}
            onClear={() => setFilters({ keyword: '' })}
            placeholder="盘点单号 / 仓库 / 备注"
            width={200}
          />
          <Select
            value={filters.warehouse_id}
            onInput={(v) => setFilters({ warehouse_id: v })}
            options={warehouseOptions}
            placeholder="全部仓库"
          />
          <Select
            value={filters.status}
            onInput={(v) => setFilters({ status: v })}
            options={STATUS_OPTIONS}
            placeholder="全部状态"
          />
          <DateRange
            from={filters.from}
            to={filters.to}
            onFrom={(v) => setFilters({ from: v })}
            onTo={(v) => setFilters({ to: v })}
          />
        </Toolbar>

        <Table
          rowKey="id"
          loading={loading}
          empty="暂无盘点单"
          columns={[
            { title: '盘点单号', key: 'take_no', className: 'num', width: 150 },
            { title: '仓库', key: 'warehouse_name', width: 120 },
            { title: '盘点日期', width: 110, render: (r) => fmtDate(r.take_date) },
            {
              title: '差异数量',
              align: 'right',
              width: 100,
              render: (r) => <DiffQty value={r.diff_qty} />,
            },
            {
              title: '差异金额',
              align: 'right',
              width: 120,
              render: (r) => <Money value={r.diff_amount} colored />,
            },
            {
              title: '状态',
              width: 90,
              align: 'center',
              render: (r) => {
                const st = takeStatus(r.status);
                return <Tag color={st.color}>{st.text}</Tag>;
              },
            },
            { title: '制单人', key: 'created_by_name', width: 90 },
            { title: '完成时间', width: 160, render: (r) => fmtDateTime(r.finished_at) },
            {
              title: '操作',
              width: 210,
              render: (r) => (
                <div class="actions">
                  {r.status === 0 ? (
                    <>
                      {can('stock:take') ? (
                        <Button size="sm" variant="link" onClick={() => openEdit(r)}>
                          编辑
                        </Button>
                      ) : null}
                      {can('stock:take') ? (
                        <Button size="sm" variant="link" onClick={() => handleFinish(r)}>
                          完成盘点
                        </Button>
                      ) : null}
                      {can('stock:take') ? (
                        <Button size="sm" variant="link" onClick={() => handleDelete(r)}>
                          删除
                        </Button>
                      ) : null}
                    </>
                  ) : (
                    <>
                      <Button size="sm" variant="link" onClick={() => openView(r)}>
                        查看
                      </Button>
                      {r.status === 1 && can('stock:take') ? (
                        <Button size="sm" variant="link" onClick={() => handleVoid(r)}>
                          作废
                        </Button>
                      ) : null}
                    </>
                  )}
                </div>
              ),
            },
          ]}
          data={list}
        />

        <Pagination page={page} size={size} total={total} onPage={setPage} onSize={setSize} />
      </Card>

      <Modal
        open={editOpen}
        title={form.id ? '编辑盘点单' : '新建盘点单'}
        width={900}
        onClose={() => setEditOpen(false)}
        closeOnMask={false}
        closeOnEsc={false}
        footer={
          <>
            <Button onClick={() => setEditOpen(false)}>取消</Button>
            <Button
              variant="primary"
              icon="save"
              loading={saving}
              disabled={bookLoading}
              onClick={handleSave}
            >
              保存
            </Button>
          </>
        }
      >
        <div class="form-grid form-grid-3">
          <Field label="盘点仓库" required>
            <Select
              value={form.warehouse_id}
              onInput={handleWarehouseChange}
              options={warehouseOptions}
              placeholder="请选择仓库"
            />
          </Field>
          <Field label="盘点日期">
            <Input
              type="date"
              value={form.take_date}
              onInput={(v) => setForm((f) => ({ ...f, take_date: v }))}
            />
          </Field>
          <Field label="备注">
            <Input
              value={form.remark}
              onInput={(v) => setForm((f) => ({ ...f, remark: v }))}
              placeholder="选填"
            />
          </Field>
        </div>

        <div class="row-between mt-12">
          <span class="muted small">实盘数量默认等于账面数量，可直接修改；差异 = 实盘 - 账面。</span>
          <Button
            size="sm"
            icon="refresh"
            loading={bookLoading}
            disabled={!form.warehouse_id}
            onClick={() => loadBook(form.warehouse_id, rows, false)}
          >
            刷新账面库存
          </Button>
        </div>

        <div class="row mt-8" style="flex-wrap:wrap">
          <SearchBox
            value={productKw}
            onInput={setProductKw}
            onSearch={searchProducts}
            placeholder="搜索商品后添加到盘点"
            width={240}
          />
          <Button size="sm" icon="search" onClick={searchProducts}>
            搜索商品
          </Button>
          <div style="min-width:220px; flex:1">
            <Select
              value={pickedProductId}
              onInput={setPickedProductId}
              options={productResults.map((p) => ({
                value: String(p.id),
                label: `${p.code ? p.code + ' ' : ''}${p.name || ''}`,
              }))}
              placeholder="选择要添加的商品"
            />
          </div>
          <Button size="sm" icon="plus" onClick={addProduct} disabled={!pickedProductId}>
            添加商品
          </Button>
        </div>
        <p class="muted small mt-8">用于盘盈或零库存商品：添加后账面数量按 0 计算，请填写实盘数量。</p>

        <div class="doc-items-table mt-8">
          <Table
            compact
            rowKey="product_id"
            loading={bookLoading}
            empty={form.warehouse_id ? '该仓库暂无可盘点的库存商品' : '请先选择盘点仓库'}
            maxHeight={360}
            columns={[
              { title: '商品编码', key: 'product_code', className: 'num', width: 110 },
              { title: '商品名称', key: 'product_name' },
              { title: '规格', key: 'spec', width: 110 },
              { title: '单位', key: 'unit', width: 60, align: 'center' },
              {
                title: '账面数量',
                align: 'right',
                width: 100,
                render: (r) => <span class="num">{fmtQty(r.book_qty)}</span>,
              },
              {
                title: '实盘数量',
                width: 120,
                render: (r, i) => (
                  <NumberInput
                    value={r.real_qty}
                    min="0"
                    onInput={(v) => updateRow(i, { real_qty: v })}
                  />
                ),
              },
              {
                title: '差异',
                align: 'right',
                width: 100,
                render: (r) => <DiffQty value={Number(r.real_qty || 0) - Number(r.book_qty || 0)} />,
              },
              {
                title: '备注',
                width: 150,
                render: (r, i) => (
                  <Input value={r.remark} onInput={(v) => updateRow(i, { remark: v })} placeholder="选填" />
                ),
              },
            ]}
            data={rows}
          />
        </div>

        <div class="doc-total">
          <div class="doc-total-item">
            <span>差异数量合计</span>
            <strong>
              <DiffQty value={totalDiffQty} />
            </strong>
          </div>
          <div class="doc-total-item">
            <span>差异金额合计</span>
            <strong>
              <Money value={totalDiffAmount} colored bold />
            </strong>
          </div>
        </div>
      </Modal>

      <Modal
        open={viewOpen}
        title="盘点单详情"
        width={900}
        onClose={() => setViewOpen(false)}
        footer={
          <>
            {viewData && viewData.status === 0 && can('stock:take') ? (
              <Button variant="primary" icon="check" onClick={() => handleFinish(viewData)}>
                完成盘点
              </Button>
            ) : null}
            {viewData && viewData.status === 1 && can('stock:take') ? (
              <Button variant="danger" icon="ban" onClick={() => handleVoid(viewData)}>
                作废
              </Button>
            ) : null}
            <Button onClick={() => setViewOpen(false)}>关闭</Button>
          </>
        }
      >
        {viewData ? (
          <>
            <Descriptions
              columns={3}
              items={[
                { label: '盘点单号', value: viewData.take_no },
                { label: '仓库', value: viewData.warehouse_name },
                { label: '盘点日期', value: fmtDate(viewData.take_date) },
                {
                  label: '状态',
                  value: (
                    <Tag color={takeStatus(viewData.status).color}>{takeStatus(viewData.status).text}</Tag>
                  ),
                },
                { label: '制单人', value: viewData.created_by_name || '-' },
                { label: '完成时间', value: fmtDateTime(viewData.finished_at) },
                { label: '差异数量', value: <DiffQty value={viewData.diff_qty} /> },
                { label: '差异金额', value: <Money value={viewData.diff_amount} colored /> },
                { label: '备注', value: viewData.remark || '-' },
              ]}
            />

            <div class="doc-items-table mt-12">
              <Table
                compact
                rowKey="id"
                empty="无盘点明细"
                maxHeight={360}
                columns={[
                  { title: '商品编码', key: 'product_code', className: 'num', width: 110 },
                  { title: '商品名称', key: 'product_name' },
                  { title: '规格', key: 'spec', width: 110 },
                  { title: '单位', key: 'unit', width: 60, align: 'center' },
                  {
                    title: '账面数量',
                    align: 'right',
                    width: 100,
                    render: (r) => <span class="num">{fmtQty(r.book_qty)}</span>,
                  },
                  {
                    title: '实盘数量',
                    align: 'right',
                    width: 100,
                    render: (r) => <span class="num">{fmtQty(r.real_qty)}</span>,
                  },
                  {
                    title: '差异',
                    align: 'right',
                    width: 100,
                    render: (r) => <DiffQty value={r.diff_qty} />,
                  },
                  { title: '单价', align: 'right', width: 100, render: (r) => <Money value={r.price} /> },
                  {
                    title: '差异金额',
                    align: 'right',
                    width: 110,
                    render: (r) => <Money value={r.diff_amount} colored />,
                  },
                  { title: '备注', key: 'remark', render: (r) => r.remark || '-' },
                ]}
                data={viewData.items || []}
              />
            </div>

            <div class="doc-total">
              <div class="doc-total-item">
                <span>差异数量合计</span>
                <strong>
                  <DiffQty value={viewData.diff_qty} />
                </strong>
              </div>
              <div class="doc-total-item">
                <span>差异金额合计</span>
                <strong>
                  <Money value={viewData.diff_amount} colored bold />
                </strong>
              </div>
            </div>
          </>
        ) : null}
      </Modal>
    </div>
  );
}
