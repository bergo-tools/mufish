// 数据维护页面：运行信息 + 数据备份 / 恢复。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import { fmtDateTime, fmtSize, fmtUptime } from '../format.js';
import {
  Button,
  Card,
  Descriptions,
  Table,
  Tag,
  confirmDialog,
  useAsync,
} from '../ui.jsx';

export default function SystemPage({ route }) {
  const canRun = can('backup:run');
  // 正在执行的操作：'' | 'backup' | 'restore'
  const [busy, setBusy] = useState('');

  // 运行信息
  const info = useAsync(async () => {
    try {
      return await Api.systemInfo();
    } catch (e) {
      notifyError(e);
      return null;
    }
  }, []);

  // 备份文件列表
  const backups = useAsync(async () => {
    try {
      return (await Api.backups()) || [];
    } catch (e) {
      notifyError(e);
      return [];
    }
  }, []);

  const s = info.data || {};

  const infoItems = [
    { label: '版本', value: s.version || '-' },
    { label: 'Go 版本', value: s.go_version || '-' },
    { label: '操作系统', value: `${s.os || '-'} / ${s.arch || '-'}` },
    { label: '数据库路径', value: <span class="mono small">{s.db_path || '-'}</span> },
    { label: '数据库大小', value: fmtSize(s.db_size) },
    { label: 'WAL 大小', value: fmtSize(s.wal_size) },
    { label: '数据目录', value: <span class="mono small">{s.data_dir || '-'}</span> },
    { label: '运行时长', value: fmtUptime(s.uptime) },
    { label: '启动时间', value: fmtDateTime(s.started_at) },
    { label: '当前时间', value: fmtDateTime(s.now) },
    { label: 'Goroutine 数', value: String(s.goroutines == null ? '-' : s.goroutines) },
    { label: '内存占用', value: fmtSize(s.memory_alloc) },
  ];

  // 立即备份
  async function runBackup() {
    setBusy('backup');
    try {
      const res = await Api.backup();
      notifyOK(`备份成功：${res && res.name ? res.name : ''}`);
      await backups.reload();
      await info.reload();
    } catch (e) {
      notifyError(e);
    } finally {
      setBusy('');
    }
  }

  // 从备份恢复
  async function runRestore(row) {
    const ok = await confirmDialog('恢复后当前数据将被备份文件覆盖，确定继续吗？');
    if (!ok) return;
    setBusy('restore');
    try {
      await Api.restore(row.name);
      notifyOK('恢复完成');
      notifyOK('请刷新页面以加载恢复后的数据');
      // 稍作停留让提示可见，再刷新页面。
      setTimeout(() => location.reload(), 900);
    } catch (e) {
      notifyError(e);
    } finally {
      setBusy('');
    }
  }

  if (!canRun) {
    return (
      <div class="page">
        <Card title="数据维护">
          <p class="muted">当前账号没有数据备份 / 恢复的权限（backup:run）。</p>
        </Card>
      </div>
    );
  }

  return (
    <div class="page">
      <Card
        title="运行信息"
        icon="database"
        extra={
          <Button icon="refresh" onClick={() => { info.reload(); backups.reload(); }}>
            刷新
          </Button>
        }
      >
        <Descriptions items={infoItems} columns={3} />
      </Card>

      <Card
        title="数据备份"
        icon="save"
        bodyClass="no-pad"
        extra={
          <>
            <span class="muted small">共 {backups.data ? backups.data.length : 0} 个备份文件</span>
            <Button variant="primary" icon="download" loading={busy === 'backup'} disabled={!!busy} onClick={runBackup}>
              立即备份
            </Button>
          </>
        }
      >
        <Table
          rowKey="name"
          loading={backups.loading}
          empty="暂无备份文件"
          data={backups.data || []}
          columns={[
            { title: '文件名', key: 'name', render: (r) => <span class="mono small">{r.name}</span> },
            { title: '大小', key: 'size', width: 120, align: 'right', render: (r) => fmtSize(r.size) },
            { title: '备份时间', key: 'mod_time', width: 175, render: (r) => fmtDateTime(r.mod_time) },
            {
              title: '操作',
              width: 100,
              align: 'center',
              render: (r) => (
                <Button size="sm" variant="link" disabled={!!busy} onClick={() => runRestore(r)}>
                  恢复
                </Button>
              ),
            },
          ]}
        />
        <p class="muted small" style="padding:10px 14px 14px">
          <Tag color="orange">注意</Tag> 恢复操作会用备份文件覆盖当前数据库，执行前请先做一次备份。
        </p>
      </Card>
    </div>
  );
}
