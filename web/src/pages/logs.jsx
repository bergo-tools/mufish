// 操作日志页面。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError } from '../store.js';
import { fmtDateTime } from '../format.js';
import {
  Button,
  Card,
  DateRange,
  Pagination,
  SearchBox,
  Select,
  Table,
  Tag,
  Toolbar,
  useAsync,
  usePagedList,
} from '../ui.jsx';

// 详情过长时截断显示，完整内容放到 title 中。
function truncate(text, max = 40) {
  const s = text == null ? '' : String(text);
  if (s.length <= max) return s;
  return s.slice(0, max) + '…';
}

export default function LogsPage({ route }) {
  const [kw, setKw] = useState('');
  const [module, setModule] = useState('');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');

  const list = usePagedList(
    (params) => Api.logs(params),
    { keyword: '', module: '', from: '', to: '' },
    20
  );

  // 模块下拉数据
  const { data: modules } = useAsync(async () => {
    try {
      return (await Api.logModules()) || [];
    } catch (e) {
      notifyError(e);
      return [];
    }
  }, []);

  const moduleOptions = (modules || []).map((m) => ({ value: m, label: m }));

  function applyFilters(patch) {
    list.setFilters({ keyword: kw, module, from, to, ...patch });
  }

  const columns = [
    { title: '时间', key: 'created_at', width: 165, render: (r) => fmtDateTime(r.created_at) },
    { title: '操作人', key: 'username', width: 110, render: (r) => r.username || '-' },
    {
      title: '模块',
      key: 'module',
      width: 110,
      render: (r) => (r.module ? <Tag color="blue">{r.module}</Tag> : '-'),
    },
    { title: '动作', key: 'action', width: 100, render: (r) => r.action || '-' },
    {
      title: '详情',
      key: 'detail',
      render: (r) => (
        <span title={r.detail || ''} class="nowrap" style="display:inline-block; max-width:420px; overflow:hidden; text-overflow:ellipsis; vertical-align:bottom">
          {truncate(r.detail) || '-'}
        </span>
      ),
    },
    { title: 'IP', key: 'ip', width: 130, render: (r) => <span class="mono small">{r.ip || '-'}</span> },
  ];

  if (!can('log:view')) {
    return (
      <div class="page">
        <Card title="操作日志">
          <p class="muted">当前账号没有查看操作日志的权限（log:view）。</p>
        </Card>
      </div>
    );
  }

  return (
    <div class="page">
      <Card
        title="操作日志"
        icon="log"
        bodyClass="no-pad"
        extra={
          <Button icon="refresh" onClick={() => list.reload()}>
            刷新
          </Button>
        }
      >
        <Toolbar>
          <SearchBox
            value={kw}
            onInput={(v) => {
              setKw(v);
              if (!v) applyFilters({ keyword: '' });
            }}
            onSearch={() => applyFilters({ keyword: kw })}
            placeholder="操作人 / 动作 / 详情"
            width={230}
          />
          <Select
            value={module}
            options={moduleOptions}
            placeholder="全部模块"
            onInput={(v) => {
              setModule(v);
              applyFilters({ module: v });
            }}
          />
          <DateRange
            from={from}
            to={to}
            onFrom={(v) => {
              setFrom(v);
              applyFilters({ from: v });
            }}
            onTo={(v) => {
              setTo(v);
              applyFilters({ to: v });
            }}
          >
            {from || to ? (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => {
                  setFrom('');
                  setTo('');
                  applyFilters({ from: '', to: '' });
                }}
              >
                清空日期
              </Button>
            ) : null}
          </DateRange>
        </Toolbar>
        <Table columns={columns} data={list.list} loading={list.loading} empty="暂无操作日志" />
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
    </div>
  );
}
