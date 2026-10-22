// 库存预警：列出结存低于安全库存的商品。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { notifyError } from '../store.js';
import { fmtQty } from '../format.js';
import { Button, Card, Money, Select, Table, Tag, Toolbar, useAsync } from '../ui.jsx';

export default function AlertsPage({ route }) {
  const [warehouseId, setWarehouseId] = useState('');

  // 预警列表（最多 200 条）。
  const { data, loading, reload } = useAsync(async () => {
    try {
      return (await Api.stockAlerts({ warehouse_id: warehouseId, limit: 200 })) || [];
    } catch (e) {
      notifyError(e);
      throw e;
    }
  }, [warehouseId]);

  // 仓库下拉。
  const warehouses = useAsync(async () => {
    try {
      return (await Api.warehouseOptions()) || [];
    } catch (e) {
      notifyError(e);
      throw e;
    }
  }, []);

  const rows = data || [];
  const warehouseOptions = (warehouses.data || []).map((w) => ({ value: String(w.id), label: w.name }));

  return (
    <div class="page">
      <Card
        title="库存预警"
        icon="alert"
        bodyClass="no-pad"
        extra={
          <span class="muted small">
            共 {rows.length} 个预警商品{rows.length >= 200 ? '（最多显示 200 条）' : ''}
          </span>
        }
      >
        <Toolbar
          right={
            <Button icon="refresh" onClick={() => reload()}>
              刷新
            </Button>
          }
        >
          <Select
            value={warehouseId}
            onInput={setWarehouseId}
            options={warehouseOptions}
            placeholder="全部仓库"
          />
          <span class="muted small">
            仅显示「已启用且设置了安全库存」的商品，结存数量低于安全库存时进入预警；建议采购金额 = 缺口数量 × 库存成本单价。
          </span>
        </Toolbar>

        <Table
          rowKey="id"
          loading={loading}
          empty="没有低库存商品，库存状态良好"
          columns={[
            { title: '商品编码', key: 'product_code', className: 'num', width: 120 },
            { title: '商品名称', key: 'product_name' },
            { title: '规格', key: 'spec', width: 130 },
            { title: '单位', key: 'unit', width: 70, align: 'center' },
            { title: '仓库', key: 'warehouse_name', width: 120 },
            {
              title: '当前结存',
              align: 'right',
              width: 110,
              render: (r) => <span class="num">{fmtQty(r.qty)}</span>,
            },
            {
              title: '安全库存',
              align: 'right',
              width: 110,
              render: (r) => <span class="num">{fmtQty(r.safety_stock)}</span>,
            },
            {
              title: '缺口',
              align: 'right',
              width: 110,
              render: (r) => {
                const gap = Number(r.safety_stock || 0) - Number(r.qty || 0);
                return <Tag color="red">{fmtQty(gap)}</Tag>;
              },
            },
            {
              title: '建议采购金额',
              align: 'right',
              width: 140,
              render: (r) => {
                const gap = Number(r.safety_stock || 0) - Number(r.qty || 0);
                return <Money value={gap * Number(r.cost_price || 0)} bold />;
              },
            },
          ]}
          data={rows}
        />
      </Card>
    </div>
  );
}
