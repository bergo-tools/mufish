// 用户管理页面。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import { fmtDateTime } from '../format.js';
import {
  Button,
  Card,
  Field,
  Input,
  Modal,
  Pagination,
  SearchBox,
  Select,
  Switch,
  Table,
  Tag,
  Toolbar,
  confirmDialog,
  usePagedList,
} from '../ui.jsx';

// 新建 / 编辑表单的初始值。
function emptyForm() {
  return {
    username: '',
    password: '',
    name: '',
    phone: '',
    email: '',
    role_id: '',
    status: true,
    remark: '',
  };
}

export default function UsersPage({ route }) {
  const canManage = can('user:manage');

  const [kw, setKw] = useState('');
  const [roles, setRoles] = useState([]);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState(emptyForm());
  const [saving, setSaving] = useState(false);
  const [resetUser, setResetUser] = useState(null);
  const [resetPwd, setResetPwd] = useState('');

  const list = usePagedList((params) => Api.users(params), { keyword: '' }, 20);

  const roleOptions = (roles || []).map((r) => ({ value: String(r.id), label: r.name }));

  // 打开表单前懒加载角色下拉，避免无 role:view 权限时页面初始化报错。
  async function loadRoles() {
    if (roles.length) return;
    try {
      setRoles((await Api.roles()) || []);
    } catch (e) {
      notifyError(e);
    }
  }

  function openCreate() {
    loadRoles();
    setEditing(null);
    setForm(emptyForm());
    setFormOpen(true);
  }

  function openEdit(row) {
    loadRoles();
    setEditing(row);
    setForm({
      username: row.username || '',
      password: '',
      name: row.name || '',
      phone: row.phone || '',
      email: row.email || '',
      role_id: row.role_id ? String(row.role_id) : '',
      status: row.status === 1,
      remark: row.remark || '',
    });
    setFormOpen(true);
  }

  const setField = (key, value) => setForm((prev) => ({ ...prev, [key]: value }));

  async function submit() {
    const username = (form.username || '').trim();
    if (!username) {
      notifyError(new Error('请输入登录名'));
      return;
    }
    if (!editing && (form.password || '').length < 6) {
      notifyError(new Error('新增用户时口令至少 6 位'));
      return;
    }
    if (editing && form.password && form.password.length < 6) {
      notifyError(new Error('口令至少 6 位'));
      return;
    }

    const payload = {
      username,
      name: (form.name || '').trim(),
      phone: (form.phone || '').trim(),
      email: (form.email || '').trim(),
      role_id: Number(form.role_id) || 0,
      status: form.status ? 1 : 0,
      remark: (form.remark || '').trim(),
    };
    // 编辑时留空表示不修改口令，此处不传该字段。
    if (form.password) payload.password = form.password;

    setSaving(true);
    try {
      if (editing) {
        await Api.updateUser(editing.id, payload);
        notifyOK('修改成功');
      } else {
        await Api.createUser(payload);
        notifyOK('新增成功');
      }
      setFormOpen(false);
      await list.reload();
    } catch (e) {
      notifyError(e);
    } finally {
      setSaving(false);
    }
  }

  async function remove(row) {
    const ok = await confirmDialog(`确定删除用户「${row.username}」吗？删除后其登录会话将一并失效。`);
    if (!ok) return;
    try {
      await Api.deleteUser(row.id);
      notifyOK('删除成功');
      await list.reload();
    } catch (e) {
      notifyError(e);
    }
  }

  async function submitReset() {
    if (!resetUser) return;
    if ((resetPwd || '').length < 6) {
      notifyError(new Error('口令至少 6 位'));
      return;
    }
    setSaving(true);
    try {
      await Api.resetPassword(resetUser.id, resetPwd);
      notifyOK('口令重置成功');
      setResetUser(null);
      setResetPwd('');
    } catch (e) {
      notifyError(e);
    } finally {
      setSaving(false);
    }
  }

  const columns = [
    { title: '登录名', key: 'username', render: (r) => <span class="mono">{r.username}</span> },
    { title: '姓名', key: 'name', render: (r) => r.name || '-' },
    { title: '电话', key: 'phone', render: (r) => r.phone || '-' },
    { title: '邮箱', key: 'email', render: (r) => r.email || '-' },
    {
      title: '角色',
      key: 'role_name',
      render: (r) =>
        r.is_super ? <Tag color="purple">超级管理员</Tag> : r.role_name ? <Tag color="blue">{r.role_name}</Tag> : <span class="muted">未分配</span>,
    },
    {
      title: '状态',
      key: 'status',
      align: 'center',
      width: 80,
      render: (r) => (r.status === 1 ? <Tag color="green">启用</Tag> : <Tag color="gray">停用</Tag>),
    },
    { title: '最后登录', key: 'last_login_at', width: 165, render: (r) => fmtDateTime(r.last_login_at) },
    {
      title: '操作',
      width: 190,
      align: 'center',
      render: (r) =>
        canManage ? (
          <div class="row" style="justify-content:center; gap:2px">
            <Button size="sm" variant="link" onClick={() => openEdit(r)}>
              编辑
            </Button>
            <Button size="sm" variant="link" onClick={() => { setResetPwd(''); setResetUser(r); }}>
              重置口令
            </Button>
            <Button size="sm" variant="link" onClick={() => remove(r)} disabled={r.is_super}>
              删除
            </Button>
          </div>
        ) : (
          <span class="muted small">只读</span>
        ),
    },
  ];

  if (!can('user:view')) {
    return (
      <div class="page">
        <Card title="用户管理">
          <p class="muted">当前账号没有查看用户的权限（user:view）。</p>
        </Card>
      </div>
    );
  }

  return (
    <div class="page">
      <Card
        title="用户管理"
        icon="users"
        bodyClass="no-pad"
        extra={
          canManage ? (
            <Button variant="primary" icon="plus" onClick={openCreate}>
              新增用户
            </Button>
          ) : null
        }
      >
        <Toolbar>
          <SearchBox
            value={kw}
            onInput={(v) => {
              setKw(v);
              if (!v) list.setFilters({ keyword: '' });
            }}
            onSearch={() => list.setFilters({ keyword: kw })}
            onClear={() => list.setFilters({ keyword: '' })}
            placeholder="登录名 / 姓名 / 电话"
            width={240}
          />
        </Toolbar>
        <Table
          columns={columns}
          data={list.list}
          loading={list.loading}
          empty="暂无用户"
        />
        <div style="padding:0 12px 12px">
          <Pagination
            page={list.page}
            size={list.size}
            total={list.total}
            onPage={list.setPage}
            onSize={list.setSize}
          />
        </div>
      </Card>

      <Modal
        open={formOpen}
        title={editing ? '编辑用户' : '新增用户'}
        onClose={() => setFormOpen(false)}
        closeOnMask={false}
        closeOnEsc={false}
        width={640}
        footer={
          <>
            <Button onClick={() => setFormOpen(false)}>取消</Button>
            <Button variant="primary" icon="save" loading={saving} onClick={submit}>
              保存
            </Button>
          </>
        }
      >
        <div class="form-grid form-grid-2">
          <Field label="登录名" required hint={editing ? '后端不支持修改登录名' : ''}>
            <Input
              value={form.username}
              disabled={!!editing}
              placeholder="请输入登录名"
              onInput={(v) => setField('username', v)}
            />
          </Field>
          <Field label="姓名">
            <Input value={form.name} placeholder="请输入姓名" onInput={(v) => setField('name', v)} />
          </Field>
          <Field label="电话">
            <Input value={form.phone} placeholder="请输入联系电话" onInput={(v) => setField('phone', v)} />
          </Field>
          <Field label="邮箱">
            <Input value={form.email} placeholder="请输入邮箱" onInput={(v) => setField('email', v)} />
          </Field>
          <Field label="角色">
            <Select
              value={form.role_id}
              options={roleOptions}
              placeholder="请选择角色"
              onInput={(v) => setField('role_id', v)}
            />
          </Field>
          <Field label="状态">
            <Switch checked={form.status} onInput={(v) => setField('status', v)} label={form.status ? '启用' : '停用'} />
          </Field>
          <Field
            label="口令"
            required={!editing}
            hint={editing ? '留空表示不修改口令' : '至少 6 位'}
            className="span-2"
          >
            <Input
              type="password"
              value={form.password}
              placeholder={editing ? '留空表示不修改口令' : '请输入初始口令'}
              onInput={(v) => setField('password', v)}
            />
          </Field>
          <Field label="备注" className="span-2">
            <Input value={form.remark} placeholder="备注信息（可选）" onInput={(v) => setField('remark', v)} />
          </Field>
        </div>
      </Modal>

      <Modal
        open={!!resetUser}
        title="重置口令"
        onClose={() => setResetUser(null)}
        closeOnMask={false}
        closeOnEsc={false}
        width={440}
        footer={
          <>
            <Button onClick={() => setResetUser(null)}>取消</Button>
            <Button variant="primary" loading={saving} onClick={submitReset}>
              确定重置
            </Button>
          </>
        }
      >
        <p class="muted">
          为「{resetUser ? resetUser.username : ''}」设置新口令，重置后该用户需使用新口令登录。
        </p>
        <div class="mt-12">
          <Field label="新口令" required hint="至少 6 位">
            <Input
              type="password"
              value={resetPwd}
              placeholder="请输入新口令"
              onInput={(v) => setResetPwd(v)}
            />
          </Field>
        </div>
      </Modal>
    </div>
  );
}
