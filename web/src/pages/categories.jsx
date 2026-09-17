// 商品分类页面（树形展示）。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import {
  Button,
  Card,
  Field,
  Input,
  Modal,
  NumberInput,
  Select,
  Table,
  Toolbar,
  confirmDialog,
  useAsync,
} from '../ui.jsx';

const EMPTY_FORM = { name: '', parent_id: '', sort: '', remark: '' };

// 把树形分类拍平成带层级的行。
function flattenTree(list, depth = 0) {
  const out = [];
  (list || []).forEach((c) => {
    out.push({ ...c, _depth: depth, _hasChild: !!(c.children && c.children.length) });
    if (c.children && c.children.length) out.push(...flattenTree(c.children, depth + 1));
  });
  return out;
}

// 扁平分类 -> 下拉选项（带缩进）。
function flatOptions(list, excludeId) {
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
      if (c.id !== excludeId) out.push({ value: c.id, label: '　'.repeat(depth) + c.name });
      walk(c.id, depth + 1);
    });
  };
  walk(0, 0);
  arr.forEach((c) => {
    if (!used.has(c.id) && c.id !== excludeId) out.push({ value: c.id, label: c.name });
  });
  return out;
}

export default function CategoriesPage({ route }) {
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState(EMPTY_FORM);
  const [saving, setSaving] = useState(false);

  // 分类树。
  const { data, loading, reload } = useAsync(async () => {
    try {
      return await Api.categories();
    } catch (e) {
      notifyError(e);
      throw e;
    }
  }, []);

  // 扁平分类（用于上级下拉与名称回显）。
  const { data: flatData, reload: reloadFlat } = useAsync(async () => {
    try {
      return await Api.categoriesFlat();
    } catch (e) {
      notifyError(e);
      throw e;
    }
  }, []);

  const rows = flattenTree(data || []);
  const flat = flatData || [];
  const nameOf = (id) => {
    const hit = flat.find((c) => c.id === id);
    return hit ? hit.name : '';
  };

  const openForm = (row) => {
    setEditing(row || null);
    setForm(
      row
        ? {
            name: row.name || '',
            parent_id: row.parent_id ? String(row.parent_id) : '',
            sort: row.sort === undefined ? '' : String(row.sort),
            remark: row.remark || '',
          }
        : { ...EMPTY_FORM },
    );
    setFormOpen(true);
  };

  const setField = (key) => (v) => setForm((prev) => ({ ...prev, [key]: v }));

  const save = async () => {
    if (!String(form.name || '').trim()) {
      notifyError(new Error('请填写分类名称'));
      return;
    }
    setSaving(true);
    try {
      await Api.saveCategory(
        {
          name: String(form.name || '').trim(),
          parent_id: Number(form.parent_id || 0),
          sort: Number(form.sort || 0),
          remark: String(form.remark || '').trim(),
        },
        editing ? editing.id : undefined,
      );
      notifyOK(editing ? '修改成功' : '新增成功');
      setFormOpen(false);
      reload();
      reloadFlat();
    } catch (e) {
      notifyError(e);
    } finally {
      setSaving(false);
    }
  };

  const remove = async (row) => {
    const ok = await confirmDialog('确定删除分类 ' + row.name + ' 吗？');
    if (!ok) return;
    try {
      await Api.deleteCategory(row.id);
      notifyOK('删除成功');
      reload();
      reloadFlat();
    } catch (e) {
      notifyError(e);
    }
  };

  const columns = [
    {
      title: '分类名称',
      key: 'name',
      render: (r) => (
        <span
          style={{
            paddingLeft: (r._depth || 0) * 18 + 'px',
            fontWeight: r._hasChild ? 600 : 400,
          }}
        >
          {r._depth ? <span class="muted">└ </span> : null}
          {r.name}
        </span>
      ),
    },
    {
      title: '上级分类',
      key: 'parent_id',
      render: (r) => (r.parent_id ? nameOf(r.parent_id) || '-' : <span class="muted">顶级分类</span>),
    },
    { title: '排序', key: 'sort', width: 80, align: 'right', render: (r) => <span class="num">{r.sort || 0}</span> },
    { title: '备注', key: 'remark', render: (r) => r.remark || '-' },
    {
      title: '操作',
      width: 130,
      align: 'center',
      render: (r) =>
        can('category:manage') ? (
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
      <Card title="商品分类" icon="tag" bodyClass="no-pad">
        <Toolbar
          right={
            can('category:manage') ? (
              <Button variant="primary" icon="plus" onClick={() => openForm(null)}>
                新增分类
              </Button>
            ) : null
          }
        >
          <span class="muted small">共 {rows.length} 个分类，支持两级结构</span>
        </Toolbar>
        <Table columns={columns} data={rows} loading={loading} empty="暂无分类" rowKey="id" />
      </Card>

      <Modal
        open={formOpen}
        title={editing ? '编辑分类' : '新增分类'}
        onClose={() => setFormOpen(false)}
        width={520}
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
          <Field label="分类名称" required>
            <Input value={form.name} onInput={setField('name')} />
          </Field>
          <Field label="上级分类">
            <Select
              value={form.parent_id}
              onInput={setField('parent_id')}
              options={flatOptions(flat, editing ? editing.id : undefined)}
              placeholder="顶级分类"
            />
          </Field>
          <Field label="排序">
            <NumberInput value={form.sort} onInput={setField('sort')} placeholder="数字越小越靠前" />
          </Field>
          <Field label="备注">
            <Input value={form.remark} onInput={setField('remark')} />
          </Field>
        </div>
      </Modal>
    </div>
  );
}
