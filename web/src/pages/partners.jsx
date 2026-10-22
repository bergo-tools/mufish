// 往来单位（客户 / 供应商）页面。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import { PARTNER_TYPE_TEXT } from '../format.js';
import {
  Button,
  Card,
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
  usePagedList,
} from '../ui.jsx';

const EMPTY_FORM = {
  code: '',
  name: '',
  type: 'customer',
  contact: '',
  phone: '',
  address: '',
  tax_no: '',
  bank_name: '',
  bank_account: '',
  credit_limit: '',
  init_receivable: '',
  init_payable: '',
  status: 1,
  remark: '',
};

const TYPE_OPTIONS = [
  { value: 'customer', label: '客户' },
  { value: 'supplier', label: '供应商' },
  { value: 'both', label: '客户兼供应商' },
];

const STATUS_OPTIONS = [
  { value: '1', label: '启用' },
  { value: '0', label: '停用' },
];

// 类型颜色：客户蓝 / 供应商橙 / 客户兼供应商紫。
const TYPE_COLOR = { customer: 'blue', supplier: 'orange', both: 'purple' };

// 工具栏里的固定宽度下拉。
function FilterSelect({ value, onInput, options, width = 140 }) {
  return (
    <div style={{ width }}>
      <Select value={value} onInput={onInput} options={options} placeholder={null} />
    </div>
  );
}

export default function PartnersPage({ route }) {
  const [kw, setKw] = useState('');
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState(EMPTY_FORM);
  const [saving, setSaving] = useState(false);

  // 往来单位分页列表。
  const { list, total, page, size, loading, filters, setFilters, setPage, setSize, reload } =
    usePagedList(
      async (params) => {
        try {
          return await Api.partners(params);
        } catch (e) {
          notifyError(e);
          throw e;
        }
      },
      { keyword: '', type: '', status: '' },
      20,
    );

  const openForm = (row) => {
    setEditing(row || null);
    setForm(
      row
        ? {
            code: row.code || '',
            name: row.name || '',
            type: row.type || 'customer',
            contact: row.contact || '',
            phone: row.phone || '',
            address: row.address || '',
            tax_no: row.tax_no || '',
            bank_name: row.bank_name || '',
            bank_account: row.bank_account || '',
            credit_limit: row.credit_limit === undefined ? '' : String(row.credit_limit),
            init_receivable: row.init_receivable === undefined ? '' : String(row.init_receivable),
            init_payable: row.init_payable === undefined ? '' : String(row.init_payable),
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
      notifyError(new Error('请填写单位名称'));
      return;
    }
    setSaving(true);
    try {
      await Api.savePartner(
        {
          code: String(form.code || '').trim(),
          name: String(form.name || '').trim(),
          type: form.type || 'customer',
          contact: String(form.contact || '').trim(),
          phone: String(form.phone || '').trim(),
          address: String(form.address || '').trim(),
          tax_no: String(form.tax_no || '').trim(),
          bank_name: String(form.bank_name || '').trim(),
          bank_account: String(form.bank_account || '').trim(),
          credit_limit: Number(form.credit_limit || 0),
          init_receivable: Number(form.init_receivable || 0),
          init_payable: Number(form.init_payable || 0),
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
    const ok = await confirmDialog('确定删除往来单位 ' + row.name + ' 吗？');
    if (!ok) return;
    try {
      await Api.deletePartner(row.id);
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
      width: 120,
      render: (r) => (
        <Tag color={TYPE_COLOR[r.type] || 'gray'}>{PARTNER_TYPE_TEXT[r.type] || r.type || '-'}</Tag>
      ),
    },
    { title: '联系人', key: 'contact', render: (r) => r.contact || '-' },
    { title: '电话', key: 'phone', className: 'num', render: (r) => r.phone || '-' },
    {
      title: '应收',
      align: 'right',
      width: 110,
      render: (r) => <Money value={r.receivable} colored />,
    },
    {
      title: '应付',
      align: 'right',
      width: 110,
      render: (r) => <Money value={r.payable} colored />,
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
      render: (r) => (
        <span class="actions">
          {can('partner:update') ? (
            <Button size="sm" variant="link" onClick={() => openForm(r)}>
              编辑
            </Button>
          ) : null}
          {can('partner:delete') ? (
            <Button size="sm" variant="link" onClick={() => remove(r)}>
              删除
            </Button>
          ) : null}
        </span>
      ),
    },
  ];

  return (
    <div class="page">
      <Card title="往来单位" icon="users" bodyClass="no-pad">
        <Toolbar
          right={
            can('partner:create') ? (
              <Button variant="primary" icon="plus" onClick={() => openForm(null)}>
                新增单位
              </Button>
            ) : null
          }
        >
          <SearchBox
            value={kw}
            onInput={(v) => {
              setKw(v);
              if (!v) setFilters({ keyword: '' });
            }}
            onSearch={() => setFilters({ keyword: kw.trim() })}
            placeholder="名称 / 编码 / 联系人 / 电话"
          />
          <FilterSelect
            value={filters.type}
            onInput={(v) => setFilters({ type: v })}
            options={[{ value: '', label: '全部类型' }].concat(TYPE_OPTIONS)}
            width={150}
          />
          <FilterSelect
            value={filters.status}
            onInput={(v) => setFilters({ status: v })}
            options={[{ value: '', label: '全部状态' }].concat(STATUS_OPTIONS)}
            width={120}
          />
        </Toolbar>

        <Table columns={columns} data={list} loading={loading} empty="暂无往来单位" />
        <Pagination page={page} size={size} total={total} onPage={setPage} onSize={setSize} />
      </Card>

      <Modal
        open={formOpen}
        title={editing ? '编辑往来单位' : '新增往来单位'}
        onClose={() => setFormOpen(false)}
        width={760}
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
          <Field label="单位编码">
            <Input value={form.code} onInput={setField('code')} placeholder="留空自动生成" />
          </Field>
          <Field label="单位名称" required>
            <Input value={form.name} onInput={setField('name')} />
          </Field>
          <Field label="类型">
            <Select
              value={form.type}
              onInput={setField('type')}
              options={TYPE_OPTIONS}
              placeholder={null}
            />
          </Field>
          <Field label="联系人">
            <Input value={form.contact} onInput={setField('contact')} />
          </Field>
          <Field label="电话">
            <Input value={form.phone} onInput={setField('phone')} />
          </Field>
          <Field label="税号">
            <Input value={form.tax_no} onInput={setField('tax_no')} />
          </Field>
          <Field label="开户银行">
            <Input value={form.bank_name} onInput={setField('bank_name')} />
          </Field>
          <Field label="银行账号">
            <Input value={form.bank_account} onInput={setField('bank_account')} />
          </Field>
          <Field label="信用额度" hint="预留字段，暂不参与下单校验">
            <NumberInput value={form.credit_limit} onInput={setField('credit_limit')} min="0" />
          </Field>
          <Field label="期初应收">
            <NumberInput value={form.init_receivable} onInput={setField('init_receivable')} />
          </Field>
          <Field label="期初应付">
            <NumberInput value={form.init_payable} onInput={setField('init_payable')} />
          </Field>
          <Field label="状态">
            <Select
              value={String(form.status)}
              onInput={setField('status')}
              options={STATUS_OPTIONS}
              placeholder={null}
            />
          </Field>
          <Field label="地址" className="span-2">
            <Input value={form.address} onInput={setField('address')} />
          </Field>
          <Field label="备注">
            <Input value={form.remark} onInput={setField('remark')} />
          </Field>
        </div>
      </Modal>
    </div>
  );
}
