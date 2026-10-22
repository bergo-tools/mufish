// 仓库管理页面。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import { fmtMoney, fmtQty } from '../format.js';
import {
  Button,
  Card,
  Field,
  Input,
  Modal,
  Money,
  NumberInput,
  Select,
  StatCard,
  Switch,
  Table,
  Tag,
  Textarea,
  Toolbar,
  confirmDialog,
  useAsync,
} from '../ui.jsx';

const EMPTY_FORM = {
  code: '',
  name: '',
  address: '',
  contact: '',
  phone: '',
  is_default: false,
  sort: '',
  status: 1,
  remark: '',
};

const STATUS_OPTIONS = [
  { value: '1', label: '启用' },
  { value: '0', label: '停用' },
];

export default function WarehousesPage({ route }) {
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState(EMPTY_FORM);
  const [saving, setSaving] = useState(false);

  // 仓库列表。
  const { data, loading, reload } = useAsync(async () => {
    try {
      return await Api.warehouses();
    } catch (e) {
      notifyError(e);
      throw e;
    }
  }, []);

  const list = data || [];
  const totalKinds = list.reduce((s, r) => s + Number(r.product_kinds || 0), 0);
  const totalQty = list.reduce((s, r) => s + Number(r.total_qty || 0), 0);
  const totalAmount = list.reduce((s, r) => s + Number(r.total_amount || 0), 0);

  const openForm = (row) => {
    setEditing(row || null);
    setForm(
      row
        ? {
            code: row.code || '',
            name: row.name || '',
            address: row.address || '',
            contact: row.contact || '',
            phone: row.phone || '',
            is_default: !!row.is_default,
            sort: row.sort === undefined ? '' : String(row.sort),
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
      notifyError(new Error('请填写仓库名称'));
      return;
    }
    setSaving(true);
    try {
      await Api.saveWarehouse(
        {
          code: String(form.code || '').trim(),
          name: String(form.name || '').trim(),
          address: String(form.address || '').trim(),
          contact: String(form.contact || '').trim(),
          phone: String(form.phone || '').trim(),
          is_default: !!form.is_default,
          sort: Number(form.sort || 0),
          status: Number(form.status) === 0 ? 0 : 1,
          remark: String(form.remark || '').trim(),
        },
        editing ? editing.id : undefined,
      );
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
    const ok = await confirmDialog('确定删除仓库 ' + row.name + ' 吗？');
    if (!ok) return;
    try {
      await Api.deleteWarehouse(row.id);
      notifyOK('删除成功');
      reload();
    } catch (e) {
      notifyError(e);
    }
  };

  const columns = [
    { title: '编码', key: 'code', className: 'num', width: 110 },
    { title: '名称', key: 'name' },
    { title: '地址', key: 'address', render: (r) => r.address || '-' },
    { title: '联系人', key: 'contact', render: (r) => r.contact || '-' },
    { title: '电话', key: 'phone', className: 'num', render: (r) => r.phone || '-' },
    {
      title: '商品种类',
      align: 'right',
      width: 90,
      render: (r) => <span class="num">{r.product_kinds || 0}</span>,
    },
    {
      title: '库存数量',
      align: 'right',
      width: 100,
      render: (r) => <span class="num">{fmtQty(r.total_qty)}</span>,
    },
    {
      title: '库存金额',
      align: 'right',
      width: 120,
      render: (r) => <Money value={r.total_amount} bold />,
    },
    {
      title: '默认',
      align: 'center',
      width: 70,
      render: (r) => (r.is_default ? <Tag color="blue">默认</Tag> : <span class="muted">-</span>),
    },
    {
      title: '状态',
      align: 'center',
      width: 70,
      render: (r) => (r.status === 1 ? <Tag color="green">启用</Tag> : <Tag color="gray">停用</Tag>),
    },
    {
      title: '操作',
      width: 130,
      align: 'center',
      render: (r) =>
        can('warehouse:manage') ? (
          <span class="actions">
            <Button size="sm" variant="link" onClick={() => openForm(r)}>
              编辑
            </Button>
            <Button size="sm" variant="link" onClick={() => remove(r)}>
              删除
            </Button>
          </span>
        ) : (
          <span class="muted">-</span>
        ),
    },
  ];

  return (
    <div class="page">
      <div class="stats">
        <StatCard label="仓库数量" value={String(list.length)} icon="warehouse" color="blue" sub="全部仓库" />
        <StatCard label="商品种类" value={String(totalKinds)} icon="box" color="purple" sub="各仓库合计" />
        <StatCard label="库存数量" value={fmtQty(totalQty)} icon="database" color="green" sub="各仓库合计" />
        <StatCard
          label="库存金额"
          value={fmtMoney(totalAmount, true)}
          icon="money"
          color="orange"
          sub="按成本价计算"
        />
      </div>

      <Card title="仓库列表" icon="warehouse" bodyClass="no-pad">
        <Toolbar
          right={
            can('warehouse:manage') ? (
              <Button variant="primary" icon="plus" onClick={() => openForm(null)}>
                新增仓库
              </Button>
            ) : null
          }
        >
          <span class="muted small">共 {list.length} 个仓库</span>
        </Toolbar>
        <Table columns={columns} data={list} loading={loading} empty="暂无仓库" rowKey="id" />
      </Card>

      <Modal
        open={formOpen}
        title={editing ? '编辑仓库' : '新增仓库'}
        onClose={() => setFormOpen(false)}
        closeOnMask={false}
        closeOnEsc={false}
        width={640}
        footer={
          <>
            <Button onClick={() => setFormOpen(false)}>取消</Button>
            <Button variant="primary" icon="save" loading={saving} onClick={save}>
              保存
            </Button>
          </>
        }
      >
        <div class="form-grid form-grid-2">
          <Field label="仓库编码">
            <Input value={form.code} onInput={setField('code')} placeholder="留空自动生成" />
          </Field>
          <Field label="仓库名称" required>
            <Input value={form.name} onInput={setField('name')} />
          </Field>
          <Field label="联系人">
            <Input value={form.contact} onInput={setField('contact')} />
          </Field>
          <Field label="电话">
            <Input value={form.phone} onInput={setField('phone')} />
          </Field>
          <Field label="地址" className="span-2">
            <Input value={form.address} onInput={setField('address')} />
          </Field>
          <Field label="排序">
            <NumberInput value={form.sort} onInput={setField('sort')} placeholder="数字越小越靠前" />
          </Field>
          <Field label="状态">
            <Select
              value={String(form.status)}
              onInput={setField('status')}
              options={STATUS_OPTIONS}
              placeholder={null}
            />
          </Field>
          <Field label="设为默认仓库">
            <Switch checked={form.is_default} onInput={setField('is_default')} label="默认仓库" />
          </Field>
          <Field label="备注" className="span-2">
            <Textarea value={form.remark} onInput={setField('remark')} rows={2} />
          </Field>
        </div>
      </Modal>
    </div>
  );
}
