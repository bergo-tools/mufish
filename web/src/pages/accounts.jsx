// 资金账户页面。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import { ACCOUNT_TYPE_TEXT, fmtMoney } from '../format.js';
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
  type: 'cash',
  balance: '',
  is_default: false,
  sort: '',
  status: 1,
  remark: '',
};

const TYPE_OPTIONS = [
  { value: 'cash', label: '现金' },
  { value: 'bank', label: '银行' },
  { value: 'alipay', label: '支付宝' },
  { value: 'wechat', label: '微信' },
  { value: 'other', label: '其他' },
];

const STATUS_OPTIONS = [
  { value: '1', label: '启用' },
  { value: '0', label: '停用' },
];

// 账户类型标签颜色。
const TYPE_COLOR = { cash: 'green', bank: 'blue', alipay: 'purple', wechat: 'orange', other: 'gray' };

export default function AccountsPage({ route }) {
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState(EMPTY_FORM);
  const [saving, setSaving] = useState(false);

  // 资金账户列表。
  const { data, loading, reload } = useAsync(async () => {
    try {
      return await Api.accounts();
    } catch (e) {
      notifyError(e);
      throw e;
    }
  }, []);

  const list = data || [];
  const totalBalance = list.reduce((s, r) => s + Number(r.balance || 0), 0);
  const enabledCount = list.filter((r) => r.status === 1).length;

  const openForm = (row) => {
    setEditing(row || null);
    setForm(
      row
        ? {
            code: row.code || '',
            name: row.name || '',
            type: row.type || 'cash',
            balance: row.balance === undefined ? '' : String(row.balance),
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
      notifyError(new Error('请填写账户名称'));
      return;
    }
    setSaving(true);
    try {
      await Api.saveAccount(
        {
          code: String(form.code || '').trim(),
          name: String(form.name || '').trim(),
          type: form.type || 'cash',
          balance: Number(form.balance || 0),
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
    const ok = await confirmDialog('确定删除账户 ' + row.name + ' 吗？');
    if (!ok) return;
    try {
      await Api.deleteAccount(row.id);
      notifyOK('删除成功');
      reload();
    } catch (e) {
      notifyError(e);
    }
  };

  const columns = [
    { title: '编码', key: 'code', className: 'num', width: 110 },
    { title: '名称', key: 'name' },
    {
      title: '类型',
      key: 'type',
      width: 100,
      render: (r) => (
        <Tag color={TYPE_COLOR[r.type] || 'gray'}>{ACCOUNT_TYPE_TEXT[r.type] || r.type || '-'}</Tag>
      ),
    },
    {
      title: '余额',
      align: 'right',
      width: 130,
      render: (r) => <Money value={r.balance} colored bold />,
    },
    {
      title: '默认',
      align: 'center',
      width: 70,
      render: (r) => (r.is_default ? <Tag color="blue">默认</Tag> : <span class="muted">-</span>),
    },
    { title: '排序', align: 'right', width: 70, render: (r) => <span class="num">{r.sort || 0}</span> },
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
        can('account:manage') ? (
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
        <StatCard
          label="账户总额"
          value={fmtMoney(totalBalance, true)}
          icon="wallet"
          color="green"
          sub="全部账户余额合计"
        />
        <StatCard label="账户数量" value={String(list.length)} icon="money" color="blue" sub={`启用 ${enabledCount} 个`} />
      </div>

      <Card title="资金账户" icon="wallet" bodyClass="no-pad">
        <Toolbar
          right={
            can('account:manage') ? (
              <Button variant="primary" icon="plus" onClick={() => openForm(null)}>
                新增账户
              </Button>
            ) : null
          }
        >
          <span class="muted small">共 {list.length} 个账户</span>
        </Toolbar>
        <Table columns={columns} data={list} loading={loading} empty="暂无资金账户" rowKey="id" />
      </Card>

      <Modal
        open={formOpen}
        title={editing ? '编辑账户' : '新增账户'}
        onClose={() => setFormOpen(false)}
        width={600}
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
          <Field label="账户编码">
            <Input value={form.code} onInput={setField('code')} placeholder="留空自动生成" />
          </Field>
          <Field label="账户名称" required>
            <Input value={form.name} onInput={setField('name')} />
          </Field>
          <Field label="账户类型">
            <Select
              value={form.type}
              onInput={setField('type')}
              options={TYPE_OPTIONS}
              placeholder={null}
            />
          </Field>
          <Field label="余额" hint="新建时的期初余额">
            <NumberInput value={form.balance} onInput={setField('balance')} />
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
          <Field label="设为默认账户">
            <Switch checked={form.is_default} onInput={setField('is_default')} label="默认账户" />
          </Field>
          <Field label="备注" className="span-2">
            <Textarea value={form.remark} onInput={setField('remark')} rows={2} />
          </Field>
        </div>
      </Modal>
    </div>
  );
}
