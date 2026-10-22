// 收付款单：收款单 / 付款单的查询、新建、确认、作废。
import { useEffect, useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import { PAY_TYPE_TEXT, METHOD_TEXT, docStatus, fmtDate, fmtMoney, fmtQty, today } from '../format.js';
import {
  Button,
  Card,
  DateRange,
  Descriptions,
  Field,
  Modal,
  Money,
  NumberInput,
  Pagination,
  SearchBox,
  Select,
  Table,
  Tabs,
  Tag,
  Textarea,
  Toolbar,
  confirmDialog,
  useAsync,
  usePagedList,
} from '../ui.jsx';

// 收付款方式选项。
const METHOD_OPTIONS = [
  { value: 'cash', label: '现金' },
  { value: 'bank', label: '银行转账' },
  { value: 'alipay', label: '支付宝' },
  { value: 'wechat', label: '微信' },
  { value: 'offset', label: '往来冲抵' },
];

const STATUS_OPTIONS = [
  { value: '0', label: '草稿' },
  { value: '1', label: '已确认' },
  { value: '2', label: '已作废' },
];

const TYPE_OPTIONS = [
  { value: 'receipt', label: '收款单' },
  { value: 'payment', label: '付款单' },
];

// 单据类型 → 关联单据类型。
const orderTypeOf = (type) => (type === 'payment' ? 'purchase' : 'sales');
// 关联单据类型 → 往来单位类型。
const partnerKindOf = (type) => (type === 'payment' ? 'supplier' : 'customer');

function toastError(e) {
  notifyError(e);
}

// 编辑草稿时保留原关联单据：即使该单据已结清、不在未结列表中也可继续展示与保存。
function buildOrderOptions(unpaid, form) {
  const opts = (unpaid || []).map((o) => ({
    value: String(o.id),
    label: `${o.no}（未结 ${fmtMoney(o.balance)}）`,
  }));
  if (form && form.order_id && !opts.some((o) => o.value === String(form.order_id))) {
    opts.unshift({
      value: String(form.order_id),
      label: `${form.order_no || '原关联单据'}（已关联）`,
    });
  }
  return opts;
}

// 允许 route.query.type 带入默认类型。
function initTab(route) {
  const t = route && route.query ? route.query.type : '';
  return t === 'payment' ? 'payment' : 'receipt';
}

export default function PaymentsPage({ route }) {
  const tab = initTab(route);
  const [active, setActive] = useState(tab);

  const list = usePagedList(
    (params) => Api.payments(params),
    { keyword: '', type: tab, status: '', from: '', to: '' },
    20,
  );

  const [form, setForm] = useState(null); // 新建 / 编辑表单
  const [saving, setSaving] = useState(false);
  const [detail, setDetail] = useState(null); // 详情弹窗数据
  const [keyword, setKeyword] = useState('');
  const [partnerKw, setPartnerKw] = useState('');

  // 往来单位 / 资金账户下拉数据（按表单类型区分客户或供应商）。
  const formType = form ? form.type : active;
  const options = useAsync(async () => {
    const [partners, accounts] = await Promise.all([
      Api.partnerOptions(partnerKindOf(formType)),
      Api.accountOptions(),
    ]);
    return {
      partners: partners || [],
      accounts: accounts || [],
    };
  }, [formType]);

  // 关联单据：选择往来单位后加载未结单据。
  const [unpaid, setUnpaid] = useState([]);
  useEffect(() => {
    let alive = true;
    if (!form || !form.partner_id) {
      setUnpaid([]);
      return () => {
        alive = false;
      };
    }
    (async () => {
      try {
        const rows = await Api.unpaidOrders(orderTypeOf(form.type), form.partner_id);
        if (alive) setUnpaid(rows || []);
      } catch (e) {
        if (alive) {
          setUnpaid([]);
          toastError(e);
        }
      }
    })();
    return () => {
      alive = false;
    };
  }, [form && form.type, form && form.partner_id]);

  const partnerList = (options.data && options.data.partners) || [];
  const accountList = (options.data && options.data.accounts) || [];

  // 往来单位默认最多 300 条，搜索后把结果合并进下拉，已选单位不会因搜索消失。
  const searchPartners = async () => {
    try {
      const partners = await Api.partnerOptions(partnerKindOf(formType), partnerKw);
      options.setData((prev) => {
        const merged = new Map();
        ((prev && prev.partners) || []).forEach((p) => merged.set(p.id, p));
        (partners || []).forEach((p) => merged.set(p.id, p));
        return { ...(prev || {}), partners: Array.from(merged.values()) };
      });
    } catch (e) {
      toastError(e);
    }
  };

  const partnerOptions = partnerList.map((p) => ({ value: String(p.id), label: p.name }));
  if (form && form.partner_id && !partnerList.some((p) => String(p.id) === String(form.partner_id))) {
    partnerOptions.unshift({
      value: String(form.partner_id),
      label: form.partner_name || '已选单位',
    });
  }

  // 切换单据类型页签。
  const switchTab = (key) => {
    setActive(key);
    setUnpaid([]);
    list.setFilters({ type: key });
  };

  // 打开新建 / 编辑弹窗。
  const openForm = (row) => {
    setPartnerKw('');
    if (row) {
      setForm({
        id: row.id,
        type: row.type || active,
        partner_id: row.partner_id ? String(row.partner_id) : '',
        partner_name: row.partner_name || '',
        account_id: row.account_id ? String(row.account_id) : '',
        pay_date: row.pay_date ? fmtDate(row.pay_date) : today(),
        amount: row.amount === undefined || row.amount === null ? '' : String(row.amount),
        method: row.method || 'bank',
        order_id: row.order_id ? String(row.order_id) : '',
        order_no: row.order_no || '',
        remark: row.remark || '',
      });
      return;
    }
    const def = accountList.find((a) => a.is_default) || accountList[0] || null;
    setForm({
      id: 0,
      type: active,
      partner_id: '',
      partner_name: '',
      account_id: def ? String(def.id) : '',
      pay_date: today(),
      amount: '',
      method: 'bank',
      order_id: '',
      order_no: '',
      remark: '',
    });
  };

  const closeForm = () => setForm(null);

  const patchForm = (key, value) => setForm((prev) => (prev ? { ...prev, [key]: value } : prev));

  // 切换表单单据类型：往来单位与关联单据需要重选。
  const changeFormType = (type) => {
    setUnpaid([]);
    setPartnerKw('');
    setForm((prev) =>
      prev
        ? { ...prev, type, partner_id: '', partner_name: '', order_id: '', order_no: '' }
        : prev,
    );
  };

  // 选择往来单位：清空关联单据。
  const changePartner = (partner_id) => {
    const p = partnerList.find((x) => String(x.id) === String(partner_id));
    setForm((prev) =>
      prev
        ? {
            ...prev,
            partner_id,
            partner_name: p ? p.name : prev.partner_name || '',
            order_id: '',
            order_no: '',
          }
        : prev,
    );
  };

  // 选择关联单据：自动带入未结金额。
  const changeOrder = (order_id) => {
    const hit = unpaid.find((o) => String(o.id) === String(order_id));
    setForm((prev) => {
      if (!prev) return prev;
      const next = { ...prev, order_id };
      if (hit && Number(hit.balance) > 0) next.amount = String(hit.balance);
      return next;
    });
  };

  // 保存收付款单。
  const save = async () => {
    if (!form) return;
    if (!form.partner_id) {
      notifyError(new Error('请选择往来单位'));
      return;
    }
    if (form.method !== 'offset' && !form.account_id) {
      notifyError(new Error('请选择资金账户'));
      return;
    }
    const amount = Number(form.amount || 0);
    if (!(amount > 0)) {
      notifyError(new Error('金额必须大于 0'));
      return;
    }
    setSaving(true);
    try {
      await Api.savePayment(
        {
          type: form.type,
          partner_id: Number(form.partner_id),
          account_id: Number(form.account_id) || 0,
          pay_date: form.pay_date || today(),
          amount,
          method: form.method,
          order_type: form.order_id ? orderTypeOf(form.type) : '',
          order_id: form.order_id ? Number(form.order_id) : 0,
          remark: form.remark || '',
        },
        form.id || undefined,
      );
      notifyOK('保存成功');
      closeForm();
      list.reload();
    } catch (e) {
      toastError(e);
    } finally {
      setSaving(false);
    }
  };

  // 确认单据。
  const confirm = async (row) => {
    const effect = row.method === 'offset'
      ? '确认后将核销往来账款，不改变资金余额。'
      : '确认后将影响往来账款与资金余额。';
    const ok = await confirmDialog(`确认单据「${row.payment_no}」？${effect}`);
    if (!ok) return;
    try {
      await Api.confirmPayment(row.id);
      notifyOK('确认成功');
      list.reload();
    } catch (e) {
      toastError(e);
    }
  };

  // 作废单据。
  const voidDoc = async (row) => {
    const ok = await confirmDialog(`作废单据「${row.payment_no}」？作废后不可恢复。`, { okText: '作废' });
    if (!ok) return;
    try {
      await Api.voidPayment(row.id);
      notifyOK('作废成功');
      list.reload();
    } catch (e) {
      toastError(e);
    }
  };

  // 删除草稿单据。
  const remove = async (row) => {
    const ok = await confirmDialog(`删除草稿单据「${row.payment_no}」？`);
    if (!ok) return;
    try {
      await Api.deletePayment(row.id);
      notifyOK('删除成功');
      list.reload();
    } catch (e) {
      toastError(e);
    }
  };

  // 查看详情。
  const openDetail = async (row) => {
    try {
      const data = await Api.payment(row.id);
      setDetail(data || row);
    } catch (e) {
      toastError(e);
    }
  };

  const columns = [
    { title: '单据编号', key: 'payment_no', width: 160, render: (r) => <span class="num">{r.payment_no}</span> },
    { title: '往来单位', key: 'partner_name' },
    { title: '资金账户', key: 'account_name' },
    { title: '日期', width: 110, render: (r) => fmtDate(r.pay_date) },
    {
      title: '金额',
      align: 'right',
      width: 130,
      render: (r) => <Money value={r.amount} bold />,
    },
    {
      title: '收款/付款方式',
      width: 110,
      render: (r) => METHOD_TEXT[r.method] || r.method || '-',
    },
    {
      title: '关联单据',
      width: 160,
      render: (r) => (r.order_no ? <span class="num">{r.order_no}</span> : <span class="muted">—</span>),
    },
    {
      title: '状态',
      align: 'center',
      width: 84,
      render: (r) => {
        const st = docStatus(r.status);
        return <Tag color={st.color}>{st.text}</Tag>;
      },
    },
    { title: '制单人', key: 'created_by_name', width: 100 },
    {
      title: '操作',
      align: 'center',
      width: 190,
      render: (r) => (
        <div class="row" style="gap:2px; justify-content:center">
          {r.status === 0 && can('payment:create') ? (
            <Button size="sm" variant="link" onClick={() => openForm(r)}>
              编辑
            </Button>
          ) : null}
          {r.status === 0 && can('payment:confirm') ? (
            <Button size="sm" variant="link" onClick={() => confirm(r)}>
              确认
            </Button>
          ) : null}
          {r.status === 0 && can('payment:create') ? (
            <Button size="sm" variant="link" onClick={() => remove(r)}>
              删除
            </Button>
          ) : null}
          {r.status === 1 && can('payment:void') ? (
            <Button size="sm" variant="link" onClick={() => voidDoc(r)}>
              作废
            </Button>
          ) : null}
          <Button size="sm" variant="link" onClick={() => openDetail(r)}>
            查看
          </Button>
        </div>
      ),
    },
  ];

  const detailItems = detail
    ? [
        { label: '单据编号', value: detail.payment_no || '-' },
        { label: '单据类型', value: PAY_TYPE_TEXT[detail.type] || '-' },
        {
          label: '状态',
          value: (
            <Tag color={docStatus(detail.status).color}>{docStatus(detail.status).text}</Tag>
          ),
        },
        { label: '往来单位', value: detail.partner_name || '-' },
        { label: '资金账户', value: detail.method === 'offset' ? '无需资金账户' : detail.account_name || '-' },
        { label: '单据日期', value: fmtDate(detail.pay_date) },
        { label: '金额', value: <Money value={detail.amount} bold /> },
        { label: '结算方式', value: METHOD_TEXT[detail.method] || detail.method || '-' },
        { label: '关联单据', value: detail.order_no || '未关联' },
        { label: '制单人', value: detail.created_by_name || '-' },
        { label: '创建时间', value: detail.created_at || '-' },
        { label: '确认时间', value: detail.confirmed_at || '-' },
        { label: '备注', value: detail.remark || '-', span: 3 },
      ]
    : [];

  return (
    <div class="page">
      <Card bodyClass="no-pad">
        <Tabs
          tabs={[
            { key: 'receipt', label: '收款单' },
            { key: 'payment', label: '付款单' },
          ]}
          active={active}
          onChange={switchTab}
        />

        <Toolbar
          right={
            can('payment:create') ? (
              <Button variant="primary" icon="plus" onClick={() => openForm(null)}>
                新建{PAY_TYPE_TEXT[active]}单
              </Button>
            ) : null
          }
        >
          <SearchBox
            value={keyword}
            onInput={setKeyword}
            onSearch={() => list.setFilters({ keyword })}
            onClear={() => list.setFilters({ keyword: '' })}
            placeholder="单据号 / 往来单位 / 关联单据"
            width={240}
          />
          <Select
            value={list.filters.status === undefined ? '' : String(list.filters.status)}
            onInput={(v) => list.setFilters({ status: v })}
            options={STATUS_OPTIONS}
            placeholder="全部状态"
          />
          <DateRange
            from={list.filters.from}
            to={list.filters.to}
            onFrom={(v) => list.setFilters({ from: v })}
            onTo={(v) => list.setFilters({ to: v })}
          />
          <Button
            variant="ghost"
            onClick={() => {
              setKeyword('');
              list.setFilters({ keyword: '', status: '', from: '', to: '' });
            }}
          >
            重置
          </Button>
        </Toolbar>

        <Table columns={columns} data={list.list} loading={list.loading} empty="暂无收付款单" />

        <Pagination
          page={list.page}
          size={list.size}
          total={list.total}
          onPage={list.setPage}
          onSize={list.setSize}
        />
      </Card>

      {form ? (
        <Modal
          open
          width={720}
          title={form.id ? '编辑收付款单' : '新建收付款单'}
          onClose={closeForm}
          closeOnMask={false}
          closeOnEsc={false}
          footer={
            <>
              <Button onClick={closeForm}>取消</Button>
              <Button variant="primary" icon="save" loading={saving} onClick={save}>
                保存
              </Button>
            </>
          }
        >
          <div class="form-grid form-grid-2">
            <Field label="单据类型" required>
              <Select value={form.type} onInput={changeFormType} options={TYPE_OPTIONS} />
            </Field>
            <Field label="日期" required>
              <input
                class="input"
                type="date"
                value={form.pay_date || ''}
                onInput={(e) => patchForm('pay_date', e.target.value)}
              />
            </Field>
            <Field
              label="往来单位"
              required
              hint={options.loading ? '加载中…' : '单位较多时可先搜索，再选择'}
            >
              <div class="col">
                <div class="row" style="gap:6px; margin-bottom:6px">
                  <SearchBox
                    value={partnerKw}
                    onInput={setPartnerKw}
                    onSearch={searchPartners}
                    placeholder="搜索单位名称/编码"
                    width={180}
                  />
                  <Button size="sm" icon="search" onClick={searchPartners}>
                    搜索
                  </Button>
                </div>
                <Select
                  value={form.partner_id}
                  onInput={changePartner}
                  options={partnerOptions}
                  placeholder="请选择往来单位"
                />
              </div>
            </Field>
            {form.method === 'offset' ? (
              <Field label="资金账户" hint="往来冲抵不产生现金流动，不需要选择账户">
                <span class="muted">无需资金账户</span>
              </Field>
            ) : (
              <Field label="资金账户" required hint={options.loading ? '加载中…' : ''}>
              <Select
                value={form.account_id}
                onInput={(v) => patchForm('account_id', v)}
                options={accountList.map((a) => ({
                  value: String(a.id),
                  label: a.name + (Number(a.balance) ? `（余额 ${fmtMoney(a.balance)}）` : ''),
                }))}
                placeholder="请选择资金账户"
              />
              </Field>
            )}
            <Field label="金额" required>
              <NumberInput
                value={form.amount}
                min="0"
                placeholder="0.00"
                onInput={(v) => patchForm('amount', v)}
              />
            </Field>
            <Field label="结算方式" required>
              <Select
                value={form.method}
                onInput={(v) =>
                  setForm((prev) =>
                    prev
                      ? { ...prev, method: v, account_id: v === 'offset' ? '' : prev.account_id }
                      : prev,
                  )
                }
                options={METHOD_OPTIONS}
                placeholder={null}
              />
            </Field>
            <Field
              label="关联单据"
              className="span-2"
              hint={form.partner_id ? '' : '请先选择往来单位'}
            >
              <Select
                value={form.order_id}
                onInput={changeOrder}
                disabled={!form.partner_id}
                options={buildOrderOptions(unpaid, form)}
                placeholder="不关联单据"
              />
            </Field>
            <Field label="备注" className="span-2">
              <Textarea
                value={form.remark}
                rows={3}
                placeholder="可填写备注信息"
                onInput={(v) => patchForm('remark', v)}
              />
            </Field>
          </div>
        </Modal>
      ) : null}

      <Modal
        open={!!detail}
        width={720}
        title="收付款单详情"
        onClose={() => setDetail(null)}
        footer={<Button onClick={() => setDetail(null)}>关闭</Button>}
      >
        {detail ? <Descriptions items={detailItems} columns={3} /> : null}
      </Modal>
    </div>
  );
}
