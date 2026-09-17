// 个人中心页面：账号信息、我的权限、修改口令。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { clearSession, notifyError, notifyOK, session, useStore } from '../store.js';
import { fmtDateTime } from '../format.js';
import { navigate } from '../router.js';
import {
  Button,
  Card,
  Descriptions,
  Field,
  Input,
  Tag,
  confirmDialog,
} from '../ui.jsx';

export default function ProfilePage({ route }) {
  const st = useStore(session);
  const user = st.user || {};
  const permissions = st.permissions || [];

  const [oldPwd, setOldPwd] = useState('');
  const [newPwd, setNewPwd] = useState('');
  const [confirmPwd, setConfirmPwd] = useState('');
  const [saving, setSaving] = useState(false);

  const roleText = user.role_name || (user.is_super ? '超级管理员' : '未分配');

  const infoItems = [
    { label: '用户名', value: <span class="mono">{user.username || '-'}</span> },
    { label: '姓名', value: user.name || '-' },
    { label: '角色', value: roleText },
    { label: '电话', value: user.phone || '-' },
    { label: '邮箱', value: user.email || '-' },
    {
      label: '状态',
      value: user.status === 1 ? <Tag color="green">启用</Tag> : <Tag color="gray">停用</Tag>,
    },
    { label: '最后登录', value: fmtDateTime(user.last_login_at) },
    { label: '创建时间', value: fmtDateTime(user.created_at) },
  ];

  // 本地清理会话并回到登录页。
  function toLogin() {
    clearSession();
    navigate('/');
  }

  async function changePassword() {
    if (!oldPwd) {
      notifyError(new Error('请输入原口令'));
      return;
    }
    if (!newPwd || newPwd.length < 6) {
      notifyError(new Error('新口令至少 6 位'));
      return;
    }
    if (newPwd !== confirmPwd) {
      notifyError(new Error('两次输入的新口令不一致'));
      return;
    }
    setSaving(true);
    try {
      await Api.changePassword(oldPwd, newPwd);
      setOldPwd('');
      setNewPwd('');
      setConfirmPwd('');
      notifyOK('口令修改成功，请重新登录');
      try {
        await Api.logout();
      } catch (e) {
        // 退出接口失败也继续清理本地会话。
      }
      toLogin();
    } catch (e) {
      notifyError(e);
    } finally {
      setSaving(false);
    }
  }

  async function logout() {
    const ok = await confirmDialog('确定退出当前账号吗？', { okText: '退出登录' });
    if (!ok) return;
    try {
      await Api.logout();
    } catch (e) {
      // 退出接口失败也继续清理本地会话。
    }
    toLogin();
  }

  return (
    <div class="page">
      <Card
        title="账号信息"
        icon="users"
        extra={
          <Button icon="logout" onClick={logout}>
            退出登录
          </Button>
        }
      >
        <Descriptions items={infoItems} columns={4} />
      </Card>

      <Card title="我的权限" icon="shield">
        {permissions.includes('*') ? (
          <div>
            <Tag color="purple">全部权限</Tag>
            <p class="muted small mt-8">当前账号为超级管理员，拥有系统全部功能权限。</p>
          </div>
        ) : permissions.length === 0 ? (
          <p class="muted">当前账号暂未分配任何权限，请联系管理员。</p>
        ) : (
          <div class="row" style="flex-wrap:wrap; gap:6px">
            {permissions.map((code) => (
              <Tag key={code} color="blue">
                {code}
              </Tag>
            ))}
          </div>
        )}
      </Card>

      <Card title="修改口令" icon="lock">
        <div class="form-grid form-grid-3">
          <Field label="原口令" required>
            <Input
              type="password"
              value={oldPwd}
              placeholder="请输入原口令"
              onInput={(v) => setOldPwd(v)}
            />
          </Field>
          <Field label="新口令" required hint="至少 6 位">
            <Input
              type="password"
              value={newPwd}
              placeholder="请输入新口令"
              onInput={(v) => setNewPwd(v)}
            />
          </Field>
          <Field label="确认新口令" required>
            <Input
              type="password"
              value={confirmPwd}
              placeholder="请再次输入新口令"
              onInput={(v) => setConfirmPwd(v)}
            />
          </Field>
        </div>
        <div class="mt-12">
          <Button variant="primary" icon="lock" loading={saving} onClick={changePassword}>
            修改口令
          </Button>
        </div>
        <p class="muted small mt-8">口令修改成功后需要重新登录。</p>
      </Card>
    </div>
  );
}
