// 系统参数页面：企业信息、业务开关、单据设置、安全设置。
import { useEffect, useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import {
  Button,
  Card,
  Field,
  Input,
  NumberInput,
  Spinner,
  Switch,
  Tag,
  Textarea,
  useAsync,
} from '../ui.jsx';

// 空表单（默认值，服务端返回后覆盖）。
function emptyForm() {
  return {
    company_name: '',
    company_short: '',
    company_address: '',
    company_phone: '',
    company_tax_no: '',
    allow_negative_stock: false,
    purchase_price_follow: false,
    sales_price_follow: false,
    doc_print_note: '',
    session_hours: '72',
  };
}

// 后端以 "1"/"0" 字符串保存布尔值。
const toBool = (v) => v === '1' || v === 1 || v === true;
const toFlag = (v) => (v ? '1' : '0');

export default function SettingsPage({ route }) {
  const canManage = can('setting:manage');

  const { data, loading, error, reload } = useAsync(async () => {
    try {
      return (await Api.settings()) || {};
    } catch (e) {
      notifyError(e);
      return {};
    }
  }, []);

  const [form, setForm] = useState(emptyForm());
  const [saving, setSaving] = useState(false);

  // 服务端数据到达后填充表单。
  useEffect(() => {
    if (!data) return;
    setForm({
      company_name: data.company_name || '',
      company_short: data.company_short || '',
      company_address: data.company_address || '',
      company_phone: data.company_phone || '',
      company_tax_no: data.company_tax_no || '',
      allow_negative_stock: toBool(data.allow_negative_stock),
      purchase_price_follow: toBool(data.purchase_price_follow),
      sales_price_follow: toBool(data.sales_price_follow),
      doc_print_note: data.doc_print_note || '',
      session_hours: data.session_hours || '72',
    });
  }, [data]);

  const setField = (key, value) => setForm((prev) => ({ ...prev, [key]: value }));

  // 无修改权限时用只读标签代替开关（Switch 组件不支持 disabled）。
  function boolControl(label, key) {
    if (!canManage) {
      return (
        <Field label={label}>
          <span>
            <Tag color={form[key] ? 'green' : 'gray'}>{form[key] ? '已开启' : '已关闭'}</Tag>
          </span>
        </Field>
      );
    }
    return (
      <Field label={label}>
        <Switch checked={!!form[key]} onInput={(v) => setField(key, v)} label={form[key] ? '开启' : '关闭'} />
      </Field>
    );
  }

  async function save() {
    const hours = String(form.session_hours || '').trim();
    if (!hours || Number(hours) <= 0) {
      notifyError(new Error('登录保持小时数必须是大于 0 的整数'));
      return;
    }
    const payload = {
      company_name: form.company_name || '',
      company_short: form.company_short || '',
      company_address: form.company_address || '',
      company_phone: form.company_phone || '',
      company_tax_no: form.company_tax_no || '',
      allow_negative_stock: toFlag(form.allow_negative_stock),
      purchase_price_follow: toFlag(form.purchase_price_follow),
      sales_price_follow: toFlag(form.sales_price_follow),
      doc_print_note: form.doc_print_note || '',
      session_hours: hours,
    };
    setSaving(true);
    try {
      await Api.saveSettings(payload);
      notifyOK('保存成功');
      await reload();
    } catch (e) {
      notifyError(e);
    } finally {
      setSaving(false);
    }
  }

  if (!can('setting:view')) {
    return (
      <div class="page">
        <Card title="系统参数">
          <p class="muted">当前账号没有查看系统参数的权限（setting:view）。</p>
        </Card>
      </div>
    );
  }

  if (loading) return <Spinner text="正在加载系统参数…" />;

  return (
    <div class="page">
      {error ? (
        <Card title="加载失败">
          <p class="muted">{error.message}</p>
        </Card>
      ) : null}

      <Card
        title="系统参数"
        icon="gear"
        extra={
          <Button variant="primary" icon="save" loading={saving} disabled={!canManage} onClick={save}>
            保存
          </Button>
        }
      >
        <div class="col">
          <p class="muted small">
            {canManage ? '修改后点击保存生效。' : '当前账号为只读模式（缺少 setting:manage）。'}
          </p>
          <div>
            <h4 class="card-title">企业信息</h4>
            <div class="form-grid form-grid-3 mt-8">
              <Field label="企业全称">
                <Input
                  value={form.company_name}
                  disabled={!canManage}
                  placeholder="请输入企业全称"
                  onInput={(v) => setField('company_name', v)}
                />
              </Field>
              <Field label="企业简称">
                <Input
                  value={form.company_short}
                  disabled={!canManage}
                  placeholder="请输入企业简称"
                  onInput={(v) => setField('company_short', v)}
                />
              </Field>
              <Field label="联系电话">
                <Input
                  value={form.company_phone}
                  disabled={!canManage}
                  placeholder="请输入联系电话"
                  onInput={(v) => setField('company_phone', v)}
                />
              </Field>
              <Field label="税号">
                <Input
                  value={form.company_tax_no}
                  disabled={!canManage}
                  placeholder="请输入纳税人识别号"
                  onInput={(v) => setField('company_tax_no', v)}
                />
              </Field>
              <Field label="企业地址" className="span-2">
                <Input
                  value={form.company_address}
                  disabled={!canManage}
                  placeholder="请输入企业地址"
                  onInput={(v) => setField('company_address', v)}
                />
              </Field>
            </div>
          </div>

          <div class="divider" />

          <div>
            <h4 class="card-title">业务开关</h4>
            <div class="form-grid form-grid-3 mt-8">
              {boolControl('允许负库存出库', 'allow_negative_stock')}
              {boolControl('采购入库更新参考进价', 'purchase_price_follow')}
              {boolControl('销售出库更新参考售价', 'sales_price_follow')}
            </div>
          </div>

          <div class="divider" />

          <div>
            <h4 class="card-title">单据设置</h4>
            <div class="form-grid mt-8">
              <Field label="单据打印落款" hint="打印单据时显示在页面底部的说明文字">
                {canManage ? (
                  <Textarea
                    value={form.doc_print_note}
                    rows={3}
                    placeholder="例如：感谢惠顾，欢迎再次光临！"
                    onInput={(v) => setField('doc_print_note', v)}
                  />
                ) : (
                  // Textarea 组件不支持 disabled，只读时以文本展示。
                  <span class="muted">{form.doc_print_note || '（未设置）'}</span>
                )}
              </Field>
            </div>
          </div>

          <div class="divider" />

          <div>
            <h4 class="card-title">安全设置</h4>
            <div class="form-grid form-grid-3 mt-8">
              <Field label="登录保持小时数" hint="登录状态的有效时长（小时）">
                <NumberInput
                  value={form.session_hours}
                  min="1"
                  step="1"
                  disabled={!canManage}
                  placeholder="72"
                  onInput={(v) => setField('session_hours', v)}
                />
              </Field>
            </div>
          </div>

          <div>
            <Button variant="primary" icon="save" loading={saving} disabled={!canManage} onClick={save}>
              保存
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
}
