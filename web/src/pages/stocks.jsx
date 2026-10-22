// 库存查询：按仓库 / 分类 / 关键字筛选库存结存。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { notifyError } from '../store.js';
import { fmtMoney, fmtQty, fmtDateTime } from '../format.js';
import {
  Button,
  Card,
  Money,
  Pagination,
  SearchBox,
  Select,
  StatCard,
  Switch,
  Table,
  Tag,
  Toolbar,
  useAsync,
  usePagedList,
} from '../ui.jsx';

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

// 判断是否低于安全库存。
function isLow(row) {
  const safety = Number(row.safety_stock || 0);
  return safety > 0 && Number(row.qty || 0) < safety;
}

export default function StocksPage({ route }) {
  const [keyword, setKeyword] = useState('');

  const { list, total, page, size, loading, filters, setFilters, setPage, setSize } = usePagedList(
    wrapList(Api.stocks),
    { keyword: '', warehouse_id: '', category_id: '', low: '', only_stock: '' },
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

  // 分类下拉（扁平结构）。
  const categories = useAsync(async () => {
    try {
      return (await Api.categoriesFlat()) || [];
    } catch (e) {
      notifyError(e);
      throw e;
    }
  }, []);

  // 顶部汇总随仓库 / 分类筛选变化。
  const summary = useAsync(async () => {
    try {
      return await Api.stockSummary({
        warehouse_id: filters.warehouse_id,
        category_id: filters.category_id,
      });
    } catch (e) {
      notifyError(e);
      throw e;
    }
  }, [filters.warehouse_id, filters.category_id]);

  const s = summary.data || {};
  const warehouseOptions = (warehouses.data || []).map((w) => ({ value: String(w.id), label: w.name }));
  const categoryOptions = (categories.data || []).map((c) => ({ value: String(c.id), label: c.name }));

  const doSearch = () => setFilters({ keyword });

  return (
    <div class="page">
      <div class="stats">
        <StatCard label="在库商品种类" value={String(s.kinds || 0)} icon="box" color="blue" sub="按商品去重统计" />
        <StatCard label="库存总数量" value={fmtQty(s.qty)} icon="database" color="green" sub="全部仓库合计" />
        <StatCard label="库存总金额" value={fmtMoney(s.amount, true)} icon="money" color="purple" sub="按库存成本计价" />
      </div>

      <Card
        title="库存明细"
        icon="database"
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
                  setFilters({ keyword: '', warehouse_id: '', category_id: '', low: '', only_stock: '' });
                }}
              >
                重置
              </Button>
            </>
          }
        >
          <SearchBox
            value={keyword}
            onInput={setKeyword}
            onSearch={doSearch}
            onClear={() => {
              setKeyword('');
              setFilters({ keyword: '' });
            }}
            placeholder="商品编码 / 名称 / 规格"
          />
          <Select
            value={filters.warehouse_id}
            onInput={(v) => setFilters({ warehouse_id: v })}
            options={warehouseOptions}
            placeholder="全部仓库"
          />
          <Select
            value={filters.category_id}
            onInput={(v) => setFilters({ category_id: v })}
            options={categoryOptions}
            placeholder="全部分类"
          />
          <Switch
            checked={!!filters.low}
            onInput={(v) => setFilters({ low: v ? '1' : '' })}
            label="仅看低库存"
          />
          <Switch
            checked={!!filters.only_stock}
            onInput={(v) => setFilters({ only_stock: v ? '1' : '' })}
            label="隐藏零库存"
          />
        </Toolbar>

        <Table
          rowKey="id"
          loading={loading}
          empty="没有符合条件的库存记录"
          columns={[
            { title: '仓库', key: 'warehouse_name', width: 120 },
            { title: '商品编码', key: 'product_code', className: 'num', width: 120 },
            { title: '商品名称', key: 'product_name' },
            { title: '规格', key: 'spec', width: 120 },
            { title: '单位', key: 'unit', width: 70, align: 'center' },
            { title: '分类', key: 'category_name', width: 110 },
            {
              title: '结存数量',
              align: 'right',
              width: 110,
              render: (r) =>
                isLow(r) ? (
                  <Tag color="red">{fmtQty(r.qty)}</Tag>
                ) : (
                  <span class="num">{fmtQty(r.qty)}</span>
                ),
            },
            {
              title: '成本单价',
              align: 'right',
              width: 100,
              render: (r) => <Money value={r.cost_price} />,
            },
            {
              title: '库存金额',
              align: 'right',
              width: 120,
              render: (r) => <Money value={r.amount} bold />,
            },
            {
              title: '安全库存',
              align: 'right',
              width: 100,
              render: (r) => <span class="num">{fmtQty(r.safety_stock)}</span>,
            },
            {
              title: '更新时间',
              width: 160,
              render: (r) => fmtDateTime(r.updated_at),
            },
          ]}
          data={list}
        />

        <Pagination page={page} size={size} total={total} onPage={setPage} onSize={setSize} />
      </Card>
    </div>
  );
}
