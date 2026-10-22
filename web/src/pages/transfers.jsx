// 库存调拨：调拨单查询、录入、确认与作废。
import { useEffect, useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import {
  amountCents,
  docStatus,
  fmtDate,
  fmtDateTime,
  fmtQty,
  moneyFromCents,
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
  Table,
  Tag,
  Toolbar,
  confirmDialog,
  useAsync,
  usePagedList,
} from '../ui.jsx';

// 明细行自增键（允许同一商品出现多行）。
let rowSeq = 0;
function newRow() {
  return { _key: ++rowSeq, product_id: '', product_code: '', product_name: '', qty: 1, price: 0 };
}

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

export default function TransfersPage({ route }) {
  const [keyword, setKeyword] = useState('');

  const { list, total, page, size, loading, filters, setFilters, setPage, setSize, reload } =
    usePagedList(wrapList(Api.transfers), { keyword: '', status: '', from: '', to: '' }, 20);

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

  // 商品下拉：默认最多 300 条，支持关键字搜索，避免超过 300 个商品后
  // 排在后面的商品永远无法选择。
  const [productKw, setProductKw] = useState('');
  const [productList, setProductList] = useState([]);
  const loadProducts = async (kw) => {
    try {
      setProductList((await Api.productOptions(kw || '')) || []);
    } catch (e) {
      notifyError(e);
    }
  };
  useEffect(() => {
    loadProducts('');
  }, []);
  const productOptions = productList.map((p) => ({
    value: String(p.id),
    label: `${p.code} ${p.name}${p.spec ? ' / ' + p.spec : ''}`,
  }));

  // 新建 / 编辑弹窗。
  const [editOpen, setEditOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({
    id: 0,
    from_warehouse_id: '',
    to_warehouse_id: '',
    transfer_date: today(),
    remark: '',
  });
  const [rows, setRows] = useState([]);

  // 详情弹窗。
  const [viewOpen, setViewOpen] = useState(false);
  const [viewData, setViewData] = useState(null);

  const openCreate = () => {
    setForm({ id: 0, from_warehouse_id: '', to_warehouse_id: '', transfer_date: today(), remark: '' });
    setRows([newRow()]);
    setEditOpen(true);
  };

  const openEdit = async (row) => {
    try {
      const detail = await Api.transfer(row.id);
      setForm({
        id: detail.id,
        from_warehouse_id: String(detail.from_warehouse_id || ''),
        to_warehouse_id: String(detail.to_warehouse_id || ''),
        transfer_date: detail.transfer_date || today(),
        remark: detail.remark || '',
      });
      setRows(
        (detail.items || []).map((it) => ({
          _key: ++rowSeq,
          product_id: String(it.product_id || ''),
          product_code: it.product_code || '',
          product_name: it.product_name || '',
          qty: Number(it.qty || 0),
          price: Number(it.price || 0),
        })),
      );
      setEditOpen(true);
    } catch (e) {
      notifyError(e);
    }
  };

  const openView = async (row) => {
    try {
      const detail = await Api.transfer(row.id);
      setViewData(detail);
      setViewOpen(true);
    } catch (e) {
      notifyError(e);
    }
  };

  const updateRow = (index, patch) => {
    setRows((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch } : r)));
  };

  const pickProduct = (index, productId) => {
    const p = productList.find((x) => String(x.id) === String(productId));
    updateRow(index, {
      product_id: productId,
      product_code: p ? p.code || '' : '',
      product_name: p ? p.name || '' : '',
      spec: p ? p.spec || '' : '',
      unit: p ? p.unit || '' : '',
    });
  };

  // 单行商品下拉：编辑旧单时若商品已停用或超出下拉范围，补一条显示项。
  const rowProductOptions = (r) => {
    if (!r.product_id) return productOptions;
    if (productOptions.some((o) => o.value === String(r.product_id))) return productOptions;
    const label = `${r.product_code || ''} ${r.product_name || ''}`.trim() || '#' + r.product_id;
    return [{ value: String(r.product_id), label }, ...productOptions];
  };

  const removeRow = (index) => {
    setRows((prev) => prev.filter((r, i) => i !== index));
  };

  const addRow = () => setRows((prev) => [...prev, newRow()]);

  // 明细合计。
  const totalQty = rows.reduce((a, r) => a + Number(r.qty || 0), 0);
  const totalAmount = moneyFromCents(sumAmountCents(rows));

  const handleSave = async () => {
    if (!form.from_warehouse_id || !form.to_warehouse_id) {
      notifyError(new Error('请选择调出仓库与调入仓库'));
      return;
    }
    if (form.from_warehouse_id === form.to_warehouse_id) {
      notifyError(new Error('调出仓库与调入仓库不能相同'));
      return;
    }
    if (rows.length === 0) {
      notifyError(new Error('请至少录入一行调拨明细'));
      return;
    }
    for (let i = 0; i < rows.length; i++) {
      if (!rows[i].product_id) {
        notifyError(new Error(`第 ${i + 1} 行未选择商品`));
        return;
      }
      if (Number(rows[i].qty || 0) <= 0) {
        notifyError(new Error(`第 ${i + 1} 行数量必须大于 0`));
        return;
      }
    }
    setSaving(true);
    try {
      await Api.saveTransfer(
        {
          from_warehouse_id: Number(form.from_warehouse_id),
          to_warehouse_id: Number(form.to_warehouse_id),
          transfer_date: form.transfer_date || today(),
          remark: form.remark,
          items: rows.map((r) => ({
            product_id: Number(r.product_id),
            qty: Number(r.qty || 0),
            price: Number(r.price || 0),
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

  // 确认调拨：调出仓减库存、调入仓加库存。
  const handleConfirm = async (row) => {
    const ok = await confirmDialog(
      `确认调拨单「${row.transfer_no}」？确认后将扣减「${row.from_name}」库存并增加「${row.to_name}」库存。`,
      { title: '确认调拨' },
    );
    if (!ok) return;
    try {
      await Api.confirmTransfer(row.id);
      notifyOK('确认调拨成功');
      setViewOpen(false);
      reload();
    } catch (e) {
      notifyError(e);
    }
  };

  const handleVoid = async (row) => {
    const ok = await confirmDialog(`确认作废调拨单「${row.transfer_no}」？已确认的调拨将自动回滚库存。`, {
      title: '作废调拨单',
    });
    if (!ok) return;
    try {
      await Api.voidTransfer(row.id);
      notifyOK('作废成功');
      setViewOpen(false);
      reload();
    } catch (e) {
      notifyError(e);
    }
  };

  const handleDelete = async (row) => {
    const ok = await confirmDialog(`确认删除调拨单「${row.transfer_no}」？删除后不可恢复。`, { title: '删除调拨单' });
    if (!ok) return;
    try {
      await Api.deleteTransfer(row.id);
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
        title="调拨单列表"
        icon="transfer"
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
                  setFilters({ keyword: '', status: '', from: '', to: '' });
                }}
              >
                重置
              </Button>
              {can('stock:transfer') ? (
                <Button icon="plus" variant="primary" onClick={openCreate}>
                  新建调拨
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
            placeholder="调拨单号 / 仓库 / 备注"
            width={200}
          />
          <Select
            value={filters.status}
            onInput={(v) => setFilters({ status: v })}
            options={[
              { value: '0', label: '草稿' },
              { value: '1', label: '已确认' },
              { value: '2', label: '已作废' },
            ]}
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
          empty="暂无调拨单"
          columns={[
            { title: '调拨单号', key: 'transfer_no', className: 'num', width: 150 },
            { title: '调出仓库', key: 'from_name', width: 120 },
            { title: '调入仓库', key: 'to_name', width: 120 },
            { title: '日期', width: 110, render: (r) => fmtDate(r.transfer_date) },
            {
              title: '数量',
              align: 'right',
              width: 100,
              render: (r) => <span class="num">{fmtQty(r.total_qty)}</span>,
            },
            { title: '金额', align: 'right', width: 120, render: (r) => <Money value={r.total_amount} bold /> },
            {
              title: '状态',
              width: 90,
              align: 'center',
              render: (r) => {
                const st = docStatus(r.status);
                return <Tag color={st.color}>{st.text}</Tag>;
              },
            },
            { title: '制单人', key: 'created_by_name', width: 90 },
            { title: '确认时间', width: 160, render: (r) => fmtDateTime(r.confirmed_at) },
            {
              title: '操作',
              width: 200,
              render: (r) => (
                <div class="actions">
                  {r.status === 0 ? (
                    <>
                      {can('stock:transfer') ? (
                        <Button size="sm" variant="link" onClick={() => openEdit(r)}>
                          编辑
                        </Button>
                      ) : null}
                      {can('stock:transfer') ? (
                        <Button size="sm" variant="link" onClick={() => handleConfirm(r)}>
                          确认调拨
                        </Button>
                      ) : null}
                      {can('stock:transfer') ? (
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
                      {r.status === 1 && can('stock:transfer') ? (
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
        title={form.id ? '编辑调拨单' : '新建调拨单'}
        width={900}
        onClose={() => setEditOpen(false)}
        closeOnMask={false}
        closeOnEsc={false}
        footer={
          <>
            <Button onClick={() => setEditOpen(false)}>取消</Button>
            <Button variant="primary" icon="save" loading={saving} onClick={handleSave}>
              保存
            </Button>
          </>
        }
      >
        <div class="form-grid form-grid-4">
          <Field label="调出仓库" required>
            <Select
              value={form.from_warehouse_id}
              onInput={(v) => setForm((f) => ({ ...f, from_warehouse_id: v }))}
              options={warehouseOptions}
              placeholder="请选择调出仓库"
            />
          </Field>
          <Field label="调入仓库" required>
            <Select
              value={form.to_warehouse_id}
              onInput={(v) => setForm((f) => ({ ...f, to_warehouse_id: v }))}
              options={warehouseOptions}
              placeholder="请选择调入仓库"
            />
          </Field>
          <Field label="调拨日期">
            <Input
              type="date"
              value={form.transfer_date}
              onInput={(v) => setForm((f) => ({ ...f, transfer_date: v }))}
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

        {form.from_warehouse_id && form.to_warehouse_id && form.from_warehouse_id === form.to_warehouse_id ? (
          <p class="mt-8">
            <Tag color="red">调出仓库与调入仓库不能相同，请重新选择</Tag>
          </p>
        ) : null}

        <div class="row-between mt-12">
          <div class="row">
            <SearchBox
              value={productKw}
              onInput={setProductKw}
              onSearch={() => loadProducts(productKw)}
              placeholder="搜索商品，回车刷新下拉"
              width={240}
            />
            <Button size="sm" icon="search" onClick={() => loadProducts(productKw)}>
              搜索商品
            </Button>
          </div>
          <Button size="sm" icon="plus" onClick={addRow}>
            添加一行
          </Button>
        </div>
        <p class="muted small mt-8">单价可留 0，确认调拨时按调出仓库的库存成本自动结转。</p>

        <div class="doc-items-table mt-8">
          <Table
            compact
            rowKey="_key"
            empty="请添加调拨明细"
            maxHeight={340}
            columns={[
              {
                title: '商品',
                render: (r, i) => (
                  <Select
                    value={r.product_id}
                    onInput={(v) => pickProduct(i, v)}
                    options={rowProductOptions(r)}
                    placeholder="请选择商品"
                  />
                ),
              },
              {
                title: '数量',
                width: 110,
                render: (r, i) => (
                  <NumberInput value={r.qty} min="0" onInput={(v) => updateRow(i, { qty: v })} />
                ),
              },
              {
                title: '单价',
                width: 120,
                render: (r, i) => (
                  <NumberInput value={r.price} min="0" onInput={(v) => updateRow(i, { price: v })} />
                ),
              },
              {
                title: '金额',
                align: 'right',
                width: 120,
                render: (r) => <Money value={moneyFromCents(amountCents(r.qty, r.price))} />,
              },
              {
                title: '操作',
                width: 70,
                align: 'center',
                render: (r, i) => (
                  <Button size="sm" variant="link" onClick={() => removeRow(i)}>
                    删除
                  </Button>
                ),
              },
            ]}
            data={rows}
          />
        </div>

        <div class="doc-total">
          <div class="doc-total-item">
            <span>数量合计</span>
            <strong>{fmtQty(totalQty)}</strong>
          </div>
          <div class="doc-total-item">
            <span>金额合计</span>
            <strong>
              <Money value={totalAmount} bold />
            </strong>
          </div>
        </div>
      </Modal>

      <Modal
        open={viewOpen}
        title="调拨单详情"
        width={900}
        onClose={() => setViewOpen(false)}
        footer={
          <>
            {viewData && viewData.status === 0 && can('stock:transfer') ? (
              <Button variant="primary" icon="check" onClick={() => handleConfirm(viewData)}>
                确认调拨
              </Button>
            ) : null}
            {viewData && viewData.status === 1 && can('stock:transfer') ? (
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
                { label: '调拨单号', value: viewData.transfer_no },
                { label: '调出仓库', value: viewData.from_name },
                { label: '调入仓库', value: viewData.to_name },
                { label: '调拨日期', value: fmtDate(viewData.transfer_date) },
                {
                  label: '状态',
                  value: (
                    <Tag color={docStatus(viewData.status).color}>{docStatus(viewData.status).text}</Tag>
                  ),
                },
                { label: '制单人', value: viewData.created_by_name || '-' },
                { label: '确认时间', value: fmtDateTime(viewData.confirmed_at) },
                { label: '数量合计', value: fmtQty(viewData.total_qty) },
                { label: '金额合计', value: <Money value={viewData.total_amount} bold /> },
                { label: '备注', value: viewData.remark || '-' },
              ]}
            />

            <div class="doc-items-table mt-12">
              <Table
                compact
                rowKey="id"
                empty="无调拨明细"
                maxHeight={340}
                columns={[
                  { title: '商品编码', key: 'product_code', className: 'num', width: 110 },
                  { title: '商品名称', key: 'product_name' },
                  { title: '规格', key: 'spec', width: 120 },
                  { title: '单位', key: 'unit', width: 60, align: 'center' },
                  {
                    title: '数量',
                    align: 'right',
                    width: 100,
                    render: (r) => <span class="num">{fmtQty(r.qty)}</span>,
                  },
                  { title: '单价', align: 'right', width: 110, render: (r) => <Money value={r.price} /> },
                  { title: '金额', align: 'right', width: 120, render: (r) => <Money value={r.amount} bold /> },
                ]}
                data={viewData.items || []}
              />
            </div>

            <div class="doc-total">
              <div class="doc-total-item">
                <span>数量合计</span>
                <strong>{fmtQty(viewData.total_qty)}</strong>
              </div>
              <div class="doc-total-item">
                <span>金额合计</span>
                <strong>
                  <Money value={viewData.total_amount} bold />
                </strong>
              </div>
            </div>
          </>
        ) : null}
      </Modal>
    </div>
  );
}
