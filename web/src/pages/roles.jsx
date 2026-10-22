// 角色权限页面：左侧角色列表，右侧权限勾选编辑区。
import { useEffect, useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import {
  Button,
  Card,
  Checkbox,
  Field,
  Input,
  Modal,
  Switch,
  Tag,
  confirmDialog,
  useAsync,
} from '../ui.jsx';

export default function RolesPage({ route }) {
  const canManage = can('role:manage');

  const [selId, setSelId] = useState(null);
  const [form, setForm] = useState(null);
  const [saving, setSaving] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm, setCreateForm] = useState({ code: '', name: '', remark: '' });

  // 角色列表
  const {
    data: roleData,
    loading: roleLoading,
    setData: setRoleData,
  } = useAsync(async () => {
    try {
      return (await Api.roles()) || [];
    } catch (e) {
      notifyError(e);
      return [];
    }
  }, []);

  // 权限目录
  const { data: permMeta } = useAsync(async () => {
    try {
      return await Api.permissions();
    } catch (e) {
      notifyError(e);
      return null;
    }
  }, []);

  const roleList = roleData || [];
  const current = roleList.find((r) => r.id === selId) || null;
  const catalog = (permMeta && permMeta.catalog) || [];
  const superCode = (permMeta && permMeta.super) || '*';
  // 无管理权限或内置管理员角色时，权限区只读。
  const readOnly = !canManage || (current && current.code === 'admin');

  // 默认选中第一个角色。
  useEffect(() => {
    if (!roleList.length) return;
    if (!selId || !roleList.some((r) => r.id === selId)) setSelId(roleList[0].id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [roleData]);

  // 切换角色时同步编辑表单。
  useEffect(() => {
    if (!current) {
      setForm(null);
      return;
    }
    const perms = current.perms || [];
    setForm({
      name: current.name || '',
      code: current.code || '',
      remark: current.remark || '',
      perms: perms.filter((p) => p !== superCode),
      super: perms.includes(superCode),
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selId, roleData]);

  // 重新拉取角色列表，可同时指定选中项。
  async function refreshRoles(selectId) {
    try {
      const fresh = (await Api.roles()) || [];
      setRoleData(fresh);
      if (selectId) setSelId(selectId);
    } catch (e) {
      notifyError(e);
    }
  }

  const setField = (key, value) => setForm((prev) => ({ ...prev, [key]: value }));

  function togglePerm(code, checked) {
    if (!form) return;
    setForm((prev) => {
      const perms = checked ? [...prev.perms, code] : prev.perms.filter((p) => p !== code);
      return { ...prev, perms };
    });
  }

  function groupState(group) {
    const codes = group.perms.map((p) => p.code);
    const hit = codes.filter((c) => form.perms.includes(c)).length;
    return { all: hit === codes.length, none: hit === 0 };
  }

  function toggleGroup(group) {
    if (!form) return;
    const codes = group.perms.map((p) => p.code);
    const { all } = groupState(group);
    setForm((prev) => ({
      ...prev,
      perms: all
        ? prev.perms.filter((c) => !codes.includes(c))
        : Array.from(new Set([...prev.perms, ...codes])),
    }));
  }

  async function save() {
    if (!current || !form) return;
    if (!form.name.trim()) {
      notifyError(new Error('角色名称不能为空'));
      return;
    }
    // 超级权限只保存通配符，普通情况只保存具体权限码。
    const perms = form.super ? [superCode] : form.perms;
    setSaving(true);
    try {
      await Api.updateRole(current.id, {
        code: current.code,
        name: form.name.trim(),
        remark: (form.remark || '').trim(),
        perms,
      });
      notifyOK('保存成功');
      await refreshRoles(current.id);
    } catch (e) {
      notifyError(e);
    } finally {
      setSaving(false);
    }
  }

  async function remove() {
    if (!current) return;
    const ok = await confirmDialog(`确定删除角色「${current.name}」吗？`);
    if (!ok) return;
    try {
      await Api.deleteRole(current.id);
      notifyOK('删除成功');
      setSelId(null);
      await refreshRoles();
    } catch (e) {
      notifyError(e);
    }
  }

  async function submitCreate() {
    if (!createForm.code.trim() || !createForm.name.trim()) {
      notifyError(new Error('角色编码与名称不能为空'));
      return;
    }
    setSaving(true);
    try {
      const res = await Api.createRole({
        code: createForm.code.trim(),
        name: createForm.name.trim(),
        remark: (createForm.remark || '').trim(),
        perms: [],
      });
      notifyOK('新增成功');
      setCreateOpen(false);
      setCreateForm({ code: '', name: '', remark: '' });
      await refreshRoles(res && res.id ? res.id : null);
    } catch (e) {
      notifyError(e);
    } finally {
      setSaving(false);
    }
  }

  if (!can('role:view')) {
    return (
      <div class="page">
        <Card title="角色权限">
          <p class="muted">当前账号没有查看角色的权限（role:view）。</p>
        </Card>
      </div>
    );
  }

  return (
    <div class="page">
      <Card
        title="角色权限"
        icon="shield"
        extra={
          canManage ? (
            <>
              <Button icon="plus" onClick={() => setCreateOpen(true)}>
                新增角色
              </Button>
              <Button
                variant="danger"
                icon="trash"
                disabled={!current || current.is_system}
                onClick={remove}
              >
                删除角色
              </Button>
              <Button
                variant="primary"
                icon="save"
                loading={saving}
                disabled={!current || current.code === 'admin'}
                onClick={save}
              >
                保存权限
              </Button>
            </>
          ) : (
            <span class="muted small">只读模式（缺少 role:manage）</span>
          )
        }
      >
        <p class="muted small" style="margin-bottom:10px">
          选中左侧角色后，在右侧按模块勾选权限点，「保存权限」后立即生效。
        </p>
        <div class="row" style="align-items:flex-start; gap:14px; flex-wrap:wrap">
          {/* 左侧：角色列表 */}
          <div class="col" style="flex:0 0 250px; min-width:210px; max-width:100%">
            {roleLoading ? (
              <p class="muted small">加载中…</p>
            ) : roleList.length === 0 ? (
              <p class="muted small">暂无角色</p>
            ) : (
              roleList.map((r) => {
                const active = r.id === selId;
                return (
                  <div
                    key={r.id}
                    class="perm-group"
                    style={{
                      cursor: 'pointer',
                      marginBottom: 0,
                      borderColor: active ? 'var(--primary)' : undefined,
                      background: active ? 'var(--bg-hover)' : undefined,
                    }}
                    onClick={() => setSelId(r.id)}
                  >
                    <div class="row-between">
                      <strong>{r.name}</strong>
                      {r.is_system ? <Tag color="purple">内置</Tag> : null}
                    </div>
                    <div class="row-between mt-8">
                      <span class="muted small mono">{r.code}</span>
                      <span class="muted small">{r.user_count || 0} 个用户</span>
                    </div>
                  </div>
                );
              })
            )}
          </div>

          {/* 右侧：权限编辑区 */}
          <div class="grow" style="min-width:300px">
            {!current || !form ? (
              <p class="muted">请选择左侧角色以编辑权限。</p>
            ) : (
              <div class="col">
                <div class="form-grid form-grid-3">
                  <Field label="角色名称" required>
                    <Input
                      value={form.name}
                      disabled={readOnly}
                      onInput={(v) => setField('name', v)}
                    />
                  </Field>
                  <Field label="角色编码" hint="编码创建后不可修改">
                    <Input value={form.code} disabled />
                  </Field>
                  <Field label="备注">
                    <Input
                      value={form.remark}
                      disabled={readOnly}
                      placeholder="角色说明（可选）"
                      onInput={(v) => setField('remark', v)}
                    />
                  </Field>
                </div>

                <div class="row-between mt-12" style="flex-wrap:wrap">
                  <Switch
                    checked={!!form.super}
                    onInput={readOnly ? undefined : (v) => setField('super', v)}
                    label="超级管理员（拥有全部权限）"
                  />
                  {current.is_system ? <Tag color="purple">内置角色</Tag> : null}
                </div>

                {current.code === 'admin' ? (
                  <p class="muted small">内置管理员角色不可修改。</p>
                ) : null}

                {form.super ? (
                  <div class="perm-group" style="border-color:var(--primary)">
                    <div class="perm-group-head">
                      <Tag color="purple">全部权限</Tag>
                      <strong>该角色拥有全部权限</strong>
                    </div>
                    <p class="muted small">
                      已开启「超级管理员」开关，保存后将以通配符 {superCode} 授权，无法再单独勾选权限点。
                    </p>
                  </div>
                ) : catalog.length === 0 ? (
                  <p class="muted">权限目录加载失败或为空。</p>
                ) : (
                  catalog.map((g) => {
                    const st = groupState(g);
                    return (
                      <div class="perm-group" key={g.module}>
                        <div class="perm-group-head">
                          <strong>{g.name}</strong>
                          <span class="muted small mono">{g.module}</span>
                          <span class="grow" />
                          <Button
                            size="sm"
                            variant="link"
                            disabled={readOnly}
                            onClick={() => toggleGroup(g)}
                          >
                            {st.all ? '取消全选' : '全选'}
                          </Button>
                        </div>
                        <div class="perm-list">
                          {g.perms.map((p) => (
                            <Checkbox
                              key={p.code}
                              label={p.name}
                              checked={form.perms.includes(p.code)}
                              disabled={readOnly}
                              onInput={(v) => togglePerm(p.code, v)}
                            />
                          ))}
                        </div>
                      </div>
                    );
                  })
                )}
              </div>
            )}
          </div>
        </div>
      </Card>

      <Modal
        open={createOpen}
        title="新增角色"
        onClose={() => setCreateOpen(false)}
        closeOnMask={false}
        closeOnEsc={false}
        width={520}
        footer={
          <>
            <Button onClick={() => setCreateOpen(false)}>取消</Button>
            <Button variant="primary" loading={saving} onClick={submitCreate}>
              创建
            </Button>
          </>
        }
      >
        <div class="form-grid form-grid-2">
          <Field label="角色编码" required hint="例如 warehouse_keeper">
            <Input
              value={createForm.code}
              placeholder="英文编码"
              onInput={(v) => setCreateForm((p) => ({ ...p, code: v }))}
            />
          </Field>
          <Field label="角色名称" required>
            <Input
              value={createForm.name}
              placeholder="例如 仓管员"
              onInput={(v) => setCreateForm((p) => ({ ...p, name: v }))}
            />
          </Field>
          <Field label="备注" className="span-2">
            <Input
              value={createForm.remark}
              placeholder="角色说明（可选）"
              onInput={(v) => setCreateForm((p) => ({ ...p, remark: v }))}
            />
          </Field>
        </div>
        <p class="muted small mt-12">创建后请在右侧权限区勾选权限并保存。</p>
      </Modal>
    </div>
  );
}
