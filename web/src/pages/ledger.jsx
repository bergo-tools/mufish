// 库存流水：查询所有出入库明细，支持筛选与导出。
import { useEffect, useState } from 'preact/hooks';
import { Api } from '../api.js';
import { notifyError, notifyOK } from '../store.js';
import { BIZ_TYPE_TEXT, fmtDateTime, fmtQty, daysAgo, today } from '../format.js';
import {
  Button,
  Card,
  DateRange,
  Money,
  Pagination,
  SearchBox,
  Select,
  Table,
  Tag,
  Toolbar,
  usePagedList,
  useAsync,
} from '../ui.jsx';

// 业务类型可选项（与后端保持一致）。
const BIZ_OPTIONS = [
  { value: 'purchase', label: '采购入库' },
  { value: 'sales', label: '销售出库' },
  { value: 'take', label: '库存盘点' },
  { value: 'transfer_in', label: '调拨入库' },
  { value: 'transfer_out', label: '调拨出库' },
  { value: 'init', label: '期初库存' },
];

// 业务类型对应的标签颜色。
const BIZ_COLOR = {
  purchase: 'orange',
  sales: 'blue',
  take: 'purple',
  transfer_in: 'green',
  transfer_out: 'gray',
  init: 'gray',
};

// 分页请求包装：失败时统一提示。
function wrapList(fn) {
  return async (params) => {
    try {
      return await fn(params);
    } catch (e) {
      notifyError(e);
      throw e;
    }
  };
}

export default function LedgerPage({ route }) {
  const queryProductId = (route && route.query && route.query.product_id) || '';
  const [keyword, setKeyword] = useState('');

  const { list, total, page, size, loading, filters, setFilters, setPage, setSize } = usePagedList(
    wrapList(Api.ledger),
    {
      keyword: '',
      warehouse_id: '',
      product_id: queryProductId,
      biz_type: '',
      from: daysAgo(30),
      to: today(),
    },
    20,
  );

  // 仓库下拉。
  const warehouses = useAsync(async () => {
    try {
      return (await Api.warehouseOptions()) || [];
    } catch (e) {
      notifyError(e);
      throw e;
    }
  }, []);

  // 支持从其它页面带 product_id 跳转过来查看单个商品流水。
  useEffect(() => {
    const pid = (route && route.query && route.query.product_id) || '';
    if (pid !== filters.product_id) setFilters({ product_id: pid });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [queryProductId]);

  const warehouseOptions = (warehouses.data || []).map((w) => ({ value: String(w.id), label: w.name }));
  const doSearch = () => setFilters({ keyword });

  // 导出当前筛选条件下的流水 CSV。
  const handleExport = () => {
    try {
      Api.exportLedger({ ...filters });
      notifyOK('导出成功');
    } catch (e) {
      notifyError(e);
    }
  };

  return (
    <div class="page">
      <Card
        title="库存流水"
        icon="ledger"
        bodyClass="no-pad"
        extra={<span class="muted small">共 {total} 条记录</span>}
      >
        <Toolbar
          right={
            <>
              <Button icon="search" variant="primary" onClick={doSearch}>
                查询
              </Button>
              <Button
                icon="refresh"
                onClick={() => {
                  setKeyword('');
                  setFilters({
                    keyword: '',
                    warehouse_id: '',
                    product_id: '',
                    biz_type: '',
                    from: daysAgo(30),
                    to: today(),
                  });
                }}
              >
                重置
              </Button>
              <Button icon="download" onClick={handleExport}>
                导出
              </Button>
            </>
          }
        >
          <SearchBox
            value={keyword}
            onInput={setKeyword}
            onSearch={doSearch}
            placeholder="商品 / 单据编号 / 备注"
            width={200}
          />
          <Select
            value={filters.warehouse_id}
            onInput={(v) => setFilters({ warehouse_id: v })}
            options={warehouseOptions}
            placeholder="全部仓库"
          />
          <Select
            value={filters.biz_type}
            onInput={(v) => setFilters({ biz_type: v })}
            options={BIZ_OPTIONS}
            placeholder="全部业务类型"
          />
          <DateRange
            from={filters.from}
            to={filters.to}
            onFrom={(v) => setFilters({ from: v })}
            onTo={(v) => setFilters({ to: v })}
          />
          {filters.product_id ? (
            <>
              <Tag color="blue">指定商品 #{filters.product_id}</Tag>
              <Button size="sm" variant="ghost" onClick={() => setFilters({ product_id: '' })}>
                清除
              </Button>
            </>
          ) : null}
        </Toolbar>

        <Table
          rowKey="id"
          loading={loading}
          empty="没有符合条件的流水记录"
          columns={[
            { title: '时间', width: 160, render: (r) => fmtDateTime(r.created_at) },
            { title: '仓库', key: 'warehouse_name', width: 110 },
            {
              title: '商品',
              render: (r) => (
                <div>
                  <div>{r.product_name}</div>
                  <div class="muted small">
                    {r.product_code}
                    {r.spec ? ' · ' + r.spec : ''}
                  </div>
                </div>
              ),
            },
            {
              title: '业务类型',
              width: 100,
              align: 'center',
              render: (r) => (
                <Tag color={BIZ_COLOR[r.biz_type] || 'gray'}>
                  {BIZ_TYPE_TEXT[r.biz_type] || r.biz_type || '-'}
                </Tag>
              ),
            },
            { title: '单据编号', className: 'num', width: 140, render: (r) => r.biz_no || '-' },
            {
              title: '方向',
              width: 70,
              align: 'center',
              render: (r) =>
                r.direction > 0 ? <Tag color="green">入库</Tag> : <Tag color="red">出库</Tag>,
            },
            {
              title: '数量',
              align: 'right',
              width: 100,
              render: (r) => {
                const n = Number(r.qty || 0);
                return (
                  <span class={n > 0 ? 'money-pos' : n < 0 ? 'money-neg' : ''}>
                    {n > 0 ? '+' : ''}
                    {fmtQty(n)}
                  </span>
                );
              },
            },
            { title: '单价', align: 'right', width: 100, render: (r) => <Money value={r.price} /> },
            { title: '金额', align: 'right', width: 120, render: (r) => <Money value={r.amount} bold /> },
            {
              title: '结存数量',
              align: 'right',
              width: 100,
              render: (r) => <span class="num">{fmtQty(r.balance_after)}</span>,
            },
            { title: '操作人', key: 'created_by_name', width: 90 },
            { title: '备注', key: 'remark', render: (r) => r.remark || '-' },
          ]}
          data={list}
        />

        <Pagination page={page} size={size} total={total} onPage={setPage} onSize={setSize} />
      </Card>
    </div>
  );
}
