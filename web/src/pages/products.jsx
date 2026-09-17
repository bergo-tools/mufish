// 商品管理页面。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import { fmtQty } from '../format.js';
import {
  Button,
  Card,
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
  Textarea,
  Toolbar,
  confirmDialog,
  useAsync,
  usePagedList,
} from '../ui.jsx';

// 新增 / 编辑表单初值。
const EMPTY_FORM = {
  code: '',
  name: '',
  spec: '',
  unit: '个',
  category_id: '',
  barcode: '',
  brand: '',
  purchase_price: '',
  sale_price: '',
  safety_stock: '',
  safety_stock_max: '',
  status: 1,
  remark: '',
};

const STATUS_OPTIONS = [
  { value: '1', label: '启用' },
  { value: '0', label: '停用' },
];

const FILTER_STATUS_OPTIONS = [{ value: '', label: '全部状态' }].concat(STATUS_OPTIONS);

// 工具栏里的固定宽度下拉。
function FilterSelect({ value, onInput, options, width = 140 }) {
  return (
    <div style={{ width }}>
      <Select value={value} onInput={onInput} options={options} placeholder={null} />
    </div>
  );
}

// 把扁平分类整理成带缩进的下拉选项。
function categoryOptions(list) {
  const arr = list || [];
  const byParent = {};
  arr.forEach((c) => {
    const pid = c.parent_id || 0;
    if (!byParent[pid]) byParent[pid] = [];
    byParent[pid].push(c);
  });
  const out = [];
  const used = new Set();
  const walk = (pid, depth) => {
    (byParent[pid] || []).forEach((c) => {
      used.add(c.id);
      out.push({ value: c.id, label: '　'.repeat(depth) + c.name });
      walk(c.id, depth + 1);
    });
  };
  walk(0, 0);
  // 上级已被删除的孤儿分类也一并展示。
  arr.forEach((c) => {
    if (!used.has(c.id)) out.push({ value: c.id, label: c.name });
  });
  return out;
}

// 状态标签。
function StatusTag({ status }) {
  return status === 1 ? <Tag color="green">启用</Tag> : <Tag color="gray">停用</Tag>;
}

export default function ProductsPage({ route }) {
  const [kw, setKw] = useState('');
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState(EMPTY_FORM);
  const [saving, setSaving] = useState(false);

  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const [importOpen, setImportOpen] = useState(false);
  const [importText, setImportText] = useState('');
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState(null);

  // 分类下拉数据。
  const { data: cats } = useAsync(async () => {
    try {
      return await Api.categoriesFlat();
    } catch (e) {
      notifyError(e);
      throw e;
    }
  }, []);

  // 商品分页列表。
  const { list, total, page, size, loading, filters, setFilters, setPage, setSize, reload } =
    usePagedList(
      async (params) => {
        try {
          return await Api.products(params);
        } catch (e) {
          notifyError(e);
          throw e;
        }
      },
      { keyword: '', category_id: '', status: '' },
      20,
    );

  const catOptions = categoryOptions(cats);
  const allCatOptions = [{ value: '', label: '全部分类' }].concat(catOptions);

  const openForm = (row) => {
    setEditing(row || null);
    setForm(
      row
        ? {
            code: row.code || '',
            name: row.name || '',
            spec: row.spec || '',
            unit: row.unit || '',
            category_id: row.category_id || '',
            barcode: row.barcode || '',
            brand: row.brand || '',
            purchase_price: row.purchase_price === undefined ? '' : String(row.purchase_price),
            sale_price: row.sale_price === undefined ? '' : String(row.sale_price),
            safety_stock: row.safety_stock === undefined ? '' : String(row.safety_stock),
            safety_stock_max: row.safety_stock_max === undefined ? '' : String(row.safety_stock_max),
            status: row.status === 0 ? 0 : 1,
            remark: row.remark || '',
          }
        : { ...EMPTY_FORM },
    );
    setFormOpen(true);
  };

  const setField = (key) => (v) => setForm((prev) => ({ ...prev, [key]: v }));

  const save = async () => {
    if (!String(form.name || '').trim()) {
      notifyError(new Error('请填写商品名称'));
      return;
    }
    setSaving(true);
    try {
      const data = {
        code: String(form.code || '').trim(),
        name: String(form.name || '').trim(),
        spec: String(form.spec || '').trim(),
        unit: String(form.unit || '').trim(),
        category_id: Number(form.category_id || 0),
        barcode: String(form.barcode || '').trim(),
        brand: String(form.brand || '').trim(),
        purchase_price: Number(form.purchase_price || 0),
        sale_price: Number(form.sale_price || 0),
        safety_stock: Number(form.safety_stock || 0),
        safety_stock_max: Number(form.safety_stock_max || 0),
        status: Number(form.status) === 0 ? 0 : 1,
        remark: String(form.remark || '').trim(),
      };
      await Api.saveProduct(data, editing ? editing.id : undefined);
      notifyOK(editing ? '修改成功' : '新增成功');
      setFormOpen(false);
      reload();
    } catch (e) {
      notifyError(e);
    } finally {
      setSaving(false);
    }
  };

  const remove = async (row) => {
    const ok = await confirmDialog('确定删除商品 ' + (row.name || row.code) + ' 吗？');
    if (!ok) return;
    try {
      await Api.deleteProduct(row.id);
      notifyOK('删除成功');
      reload();
    } catch (e) {
      notifyError(e);
    }
  };

  const openDetail = async (row) => {
    setDetailOpen(true);
    setDetail(null);
    setDetailLoading(true);
    try {
      const data = await Api.product(row.id);
      setDetail(data || {});
    } catch (e) {
      notifyError(e);
    } finally {
      setDetailLoading(false);
    }
  };

  const doImport = async () => {
    if (!String(importText || '').trim()) {
      notifyError(new Error('请先粘贴 CSV 内容'));
      return;
    }
    setImporting(true);
    try {
      const res = await Api.importProducts(importText);
      setImportResult(res || {});
      notifyOK('导入完成');
      reload();
    } catch (e) {
      notifyError(e);
    } finally {
      setImporting(false);
    }
  };

  const doExport = () => {
    try {
      Api.exportProducts({
        keyword: filters.keyword,
        category_id: filters.category_id,
        status: filters.status,
      });
      notifyOK('导出已开始');
    } catch (e) {
      notifyError(e);
    }
  };

  const columns = [
    { title: '商品编码', key: 'code', className: 'num', width: 120 },
    {
      title: '名称',
      key: 'name',
      render: (r) => (
        <span>
          {r.name}
          {r.brand ? <span class="muted small"> {r.brand}</span> : null}
        </span>
      ),
    },
    { title: '规格', key: 'spec', render: (r) => r.spec || '-' },
    { title: '单位', key: 'unit', width: 60, render: (r) => r.unit || '-' },
    { title: '分类', key: 'category_name', render: (r) => r.category_name || '-' },
    { title: '条码', key: 'barcode', className: 'num', render: (r) => r.barcode || '-' },
    {
      title: '参考进价',
      align: 'right',
      width: 100,
      render: (r) => <Money value={r.purchase_price} />,
    },
    {
      title: '参考售价',
      align: 'right',
      width: 100,
      render: (r) => <Money value={r.sale_price} />,
    },
    { title: '库存', align: 'right', width: 90, render: (r) => <span class="num">{fmtQty(r.stock_qty)}</span> },
    { title: '状态', align: 'center', width: 70, render: (r) => <StatusTag status={r.status} /> },
    {
      title: '操作',
      width: 170,
      align: 'center',
      render: (r) => (
        <span class="actions">
          {can('product:update') ? (
            <Button size="sm" variant="link" onClick={() => openForm(r)}>
              编辑
            </Button>
          ) : null}
          {can('product:view') ? (
            <Button size="sm" variant="link" onClick={() => openDetail(r)}>
              查看库存
            </Button>
          ) : null}
          {can('product:delete') ? (
            <Button size="sm" variant="link" onClick={() => remove(r)}>
              删除
            </Button>
          ) : null}
        </span>
      ),
    },
  ];

  const p = (detail && detail.product) || {};
  const stocks = (detail && detail.stocks) || [];

  return (
    <div class="page">
      <Card title="商品列表" icon="box" bodyClass="no-pad">
        <Toolbar
          right={
            <>
              {can('product:create') ? (
                <Button variant="primary" icon="plus" onClick={() => openForm(null)}>
                  新增商品
                </Button>
              ) : null}
              {can('product:import') ? (
                <Button
                  icon="upload"
                  onClick={() => {
                    setImportResult(null);
                    setImportOpen(true);
                  }}
                >
                  导入
                </Button>
              ) : null}
              {can('product:view') ? (
                <Button icon="download" onClick={doExport}>
                  导出
                </Button>
              ) : null}
            </>
          }
        >
          <SearchBox
            value={kw}
            onInput={(v) => {
              setKw(v);
              if (!v) setFilters({ keyword: '' });
            }}
            onSearch={() => setFilters({ keyword: kw.trim() })}
            placeholder="商品名称 / 编码 / 条码"
          />
          <FilterSelect
            value={filters.category_id}
            onInput={(v) => setFilters({ category_id: v })}
            options={allCatOptions}
            width={160}
          />
          <FilterSelect
            value={filters.status}
            onInput={(v) => setFilters({ status: v })}
            options={FILTER_STATUS_OPTIONS}
            width={120}
          />
        </Toolbar>

        <Table columns={columns} data={list} loading={loading} empty="暂无商品" />
        <Pagination page={page} size={size} total={total} onPage={setPage} onSize={setSize} />
      </Card>

      {/* 新增 / 编辑 */}
      <Modal
        open={formOpen}
        title={editing ? '编辑商品' : '新增商品'}
        onClose={() => setFormOpen(false)}
        width={720}
        footer={
          <>
            <Button onClick={() => setFormOpen(false)}>取消</Button>
            <Button variant="primary" icon="save" loading={saving} onClick={save}>
              保存
            </Button>
          </>
        }
      >
        <div class="form-grid form-grid-3">
          <Field label="商品编码">
            <Input value={form.code} onInput={setField('code')} placeholder="留空自动生成" />
          </Field>
          <Field label="商品名称" required>
            <Input value={form.name} onInput={setField('name')} />
          </Field>
          <Field label="规格">
            <Input value={form.spec} onInput={setField('spec')} />
          </Field>
          <Field label="单位">
            <Input value={form.unit} onInput={setField('unit')} placeholder="个" />
          </Field>
          <Field label="分类">
            <Select
              value={form.category_id === '' ? '' : String(form.category_id)}
              onInput={setField('category_id')}
              options={catOptions}
              placeholder="未分类"
            />
          </Field>
          <Field label="条码">
            <Input value={form.barcode} onInput={setField('barcode')} />
          </Field>
          <Field label="品牌">
            <Input value={form.brand} onInput={setField('brand')} />
          </Field>
          <Field label="参考进价">
            <NumberInput value={form.purchase_price} onInput={setField('purchase_price')} min="0" />
          </Field>
          <Field label="参考售价">
            <NumberInput value={form.sale_price} onInput={setField('sale_price')} min="0" />
          </Field>
          <Field label="安全库存">
            <NumberInput value={form.safety_stock} onInput={setField('safety_stock')} min="0" />
          </Field>
          <Field label="库存上限">
            <NumberInput value={form.safety_stock_max} onInput={setField('safety_stock_max')} min="0" />
          </Field>
          <Field label="状态">
            <Select
              value={String(form.status)}
              onInput={setField('status')}
              options={STATUS_OPTIONS}
              placeholder={null}
            />
          </Field>
          <Field label="备注" className="span-3">
            <Textarea value={form.remark} onInput={setField('remark')} rows={2} />
          </Field>
        </div>
      </Modal>

      {/* 商品详情（含各仓库库存） */}
      <Modal
        open={detailOpen}
        title="商品详情"
        onClose={() => setDetailOpen(false)}
        width={760}
        footer={<Button onClick={() => setDetailOpen(false)}>关闭</Button>}
      >
        {detailLoading ? (
          <div class="loading-box">
            <span class="spinner" />
            <span>加载中…</span>
          </div>
        ) : (
          <div class="col">
            <Descriptions
              columns={3}
              items={[
                { label: '商品编码', value: p.code || '-' },
                { label: '商品名称', value: p.name || '-' },
                { label: '规格', value: p.spec || '-' },
                { label: '单位', value: p.unit || '-' },
                { label: '分类', value: p.category_name || '-' },
                { label: '品牌', value: p.brand || '-' },
                { label: '条码', value: p.barcode || '-' },
                { label: '参考进价', value: <Money value={p.purchase_price} /> },
                { label: '参考售价', value: <Money value={p.sale_price} /> },
                { label: '安全库存', value: fmtQty(p.safety_stock) },
                { label: '库存上限', value: fmtQty(p.safety_stock_max) },
                { label: '库存合计', value: fmtQty(p.stock_qty) },
                {
                  label: '状态',
                  value: p.status === 1 ? <Tag color="green">启用</Tag> : <Tag color="gray">停用</Tag>,
                },
                { label: '备注', value: p.remark || '-' },
              ]}
            />
            <div class="divider" />
            <Table
              compact
              empty="该商品暂无库存记录"
              rowKey="id"
              columns={[
                { title: '仓库', key: 'warehouse_name' },
                { title: '数量', align: 'right', render: (r) => <span class="num">{fmtQty(r.qty)}</span> },
                { title: '成本价', align: 'right', render: (r) => <Money value={r.cost_price} /> },
                { title: '库存金额', align: 'right', render: (r) => <Money value={r.amount} bold /> },
              ]}
              data={stocks}
            />
          </div>
        )}
      </Modal>

      {/* 批量导入 */}
      <Modal
        open={importOpen}
        title="导入商品"
        onClose={() => setImportOpen(false)}
        width={640}
        footer={
          <>
            <Button onClick={() => setImportOpen(false)}>关闭</Button>
            <Button variant="primary" icon="upload" loading={importing} onClick={doImport}>
              开始导入
            </Button>
          </>
        }
      >
        <div class="col">
          <p class="muted small">
            请粘贴 CSV 文本，列顺序：商品编码、商品名称、规格、单位、分类、条码、品牌、参考进价、参考售价、安全库存。
            首行表头可保留。
          </p>
          <Textarea
            value={importText}
            onInput={setImportText}
            rows={10}
            placeholder={'商品编码,商品名称,规格,单位,分类,条码,品牌,参考进价,参考售价,安全库存'}
          />
          {importResult ? (
            <div class="col">
              <div class="row">
                <Tag color="green">成功 {importResult.success || 0} 条</Tag>
                <Tag color="red">失败 {importResult.failed || 0} 条</Tag>
              </div>
              {(importResult.messages || []).length ? (
                <div class="col">
                  {(importResult.messages || []).map((m, i) => (
                    <span key={i} class="muted small">
                      {m}
                    </span>
                  ))}
                </div>
              ) : null}
            </div>
          ) : null}
        </div>
      </Modal>
    </div>
  );
}
