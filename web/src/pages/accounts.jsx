// 资金账户页面。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import { ACCOUNT_TYPE_TEXT, fmtDateTime, fmtMoney } from '../format.js';
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

  // 余额调账弹窗。
  const [adjustTarget, setAdjustTarget] = useState(null);
  const [adjustBalance, setAdjustBalance] = useState('');
  const [adjustReason, setAdjustReason] = useState('');
  const [adjusting, setAdjusting] = useState(false);

  // 调账记录弹窗。
  const [historyTarget, setHistoryTarget] = useState(null);
  const [historyList, setHistoryList] = useState([]);
  const [historyLoading, setHistoryLoading] = useState(false);

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
      const payload = {
        code: String(form.code || '').trim(),
        name: String(form.name || '').trim(),
        type: form.type || 'cash',
        is_default: !!form.is_default,
        sort: Number(form.sort || 0),
        status: Number(form.status) === 0 ? 0 : 1,
        remark: String(form.remark || '').trim(),
      };
      // 只有新建账户时可以带期初余额；编辑余额必须走独立调账。
      if (!editing) payload.balance = Number(form.balance || 0);
      await Api.saveAccount(payload, editing ? editing.id : undefined);
      notifyOK(editing ? '修改成功' : '新增成功');
      setFormOpen(false);
      reload();
    } catch (e) {
      notifyError(e);
    } finally {
      setSaving(false);
    }
  };

  const openAdjust = (row) => {
    setAdjustTarget(row);
    setAdjustBalance(row.balance === undefined ? '' : String(row.balance));
    setAdjustReason('');
  };

  const closeAdjust = () => {
    setAdjustTarget(null);
    setAdjustBalance('');
    setAdjustReason('');
  };

  const currentBalance = Number(adjustTarget && adjustTarget.balance) || 0;
  const nextBalance = Number(adjustBalance);
  const adjustDiff = Number.isFinite(nextBalance) ? nextBalance - currentBalance : 0;

  const submitAdjust = async () => {
    if (!adjustTarget) return;
    const raw = String(adjustBalance === null || adjustBalance === undefined ? '' : adjustBalance).trim();
    if (raw === '') {
      notifyError(new Error('请输入调整后余额'));
      return;
    }
    const balance = Number(raw);
    if (!Number.isFinite(balance)) {
      notifyError(new Error('调整后余额格式不正确'));
      return;
    }
    if (balance === currentBalance) {
      notifyError(new Error('调整后余额与当前余额相同'));
      return;
    }
    if (!String(adjustReason || '').trim()) {
      notifyError(new Error('请填写调账原因'));
      return;
    }
    setAdjusting(true);
    try {
      await Api.adjustAccount(adjustTarget.id, balance, adjustReason.trim());
      notifyOK('调账成功');
      closeAdjust();
      reload();
    } catch (e) {
      notifyError(e);
    } finally {
      setAdjusting(false);
    }
  };

  const openHistory = async (row) => {
    setHistoryTarget(row);
    setHistoryList([]);
    setHistoryLoading(true);
    try {
      setHistoryList((await Api.accountAdjustments(row.id)) || []);
    } catch (e) {
      notifyError(e);
    } finally {
      setHistoryLoading(false);
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
      width: 250,
      align: 'center',
      render: (r) => (
        <span class="actions">
          {can('account:manage') ? (
            <Button size="sm" variant="link" onClick={() => openAdjust(r)}>
              调账
            </Button>
          ) : null}
          <Button size="sm" variant="link" onClick={() => openHistory(r)}>
            记录
          </Button>
          {can('account:manage') ? (
            <>
              <Button size="sm" variant="link" onClick={() => openForm(r)}>
                编辑
              </Button>
              <Button size="sm" variant="link" onClick={() => remove(r)}>
                删除
              </Button>
            </>
          ) : null}
        </span>
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

      {/* 新增 / 编辑账户（余额不在编辑时修改） */}
      <Modal
        open={formOpen}
        title={editing ? '编辑账户' : '新增账户'}
        onClose={() => setFormOpen(false)}
        closeOnMask={false}
        closeOnEsc={false}
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
          {editing ? (
            <Field label="当前余额" hint="余额请通过列表中的「调账」修改">
              <span class="money money-bold">{fmtMoney(form.balance)}</span>
            </Field>
          ) : (
            <Field label="期初余额" hint="新建账户时的期初余额">
              <NumberInput value={form.balance} onInput={setField('balance')} />
            </Field>
          )}
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

      {/* 余额调账 */}
      <Modal
        open={!!adjustTarget}
        title="账户余额调账"
        onClose={closeAdjust}
        closeOnMask={false}
        closeOnEsc={false}
        width={620}
        footer={
          <>
            <Button onClick={closeAdjust}>取消</Button>
            <Button variant="primary" icon="save" loading={adjusting} onClick={submitAdjust}>
              确认调账
            </Button>
          </>
        }
      >
        <div class="form-grid form-grid-2">
          <Field label="账户">
            <span>{adjustTarget ? adjustTarget.name : '-'}</span>
          </Field>
          <Field label="当前余额">
            <Money value={currentBalance} bold />
          </Field>
          <Field label="调整后余额" required>
            <NumberInput
              value={adjustBalance}
              onInput={setAdjustBalance}
              placeholder="请输入调整后余额"
            />
          </Field>
          <Field label="调整差额">
            <Money value={adjustDiff} colored bold />
          </Field>
          <Field label="调账原因" required className="span-2">
            <Textarea
              value={adjustReason}
              onInput={setAdjustReason}
              rows={3}
              placeholder="例如：银行手续费、盘点差异、期初余额修正"
            />
          </Field>
        </div>
        <p class="muted small mt-8">
          确认后会生成一条独立的调账记录，不会生成收付款单，也不会关联往来单位。账户余额不能在编辑账户时直接覆盖。
        </p>
      </Modal>

      {/* 调账记录 */}
      <Modal
        open={!!historyTarget}
        title={`调账记录 - ${historyTarget ? historyTarget.name : ''}`}
        onClose={() => setHistoryTarget(null)}
        width={900}
        footer={<Button onClick={() => setHistoryTarget(null)}>关闭</Button>}
      >
        <Table
          rowKey="id"
          loading={historyLoading}
          empty="暂无调账记录"
          data={historyList}
          columns={[
            { title: '时间', width: 165, render: (r) => fmtDateTime(r.created_at) },
            { title: '调整前', align: 'right', render: (r) => <Money value={r.before_balance} /> },
            { title: '调整后', align: 'right', render: (r) => <Money value={r.after_balance} /> },
            { title: '差额', align: 'right', render: (r) => <Money value={r.amount} colored bold /> },
            { title: '原因', key: 'reason', render: (r) => r.reason || '-' },
            { title: '操作人', width: 110, render: (r) => r.created_by_name || '-' },
          ]}
        />
      </Modal>
    </div>
  );
}
