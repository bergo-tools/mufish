import { Api } from '../api.js';
import { navigate } from '../router.js';
import { fmtMoney, fmtQty, fmtDate, docStatus } from '../format.js';
import { Button, Card, Money, Spinner, StatCard, Table, Tag, useAsync } from '../ui.jsx';

export default function DashboardPage() {
  const { data, loading, error } = useAsync(() => Api.dashboard(), []);

  if (loading) return <Spinner text="正在加载看板数据…" />;
  if (error) {
    return (
      <div class="page">
        <Card title="加载失败">
          <p class="muted">{error.message}</p>
          <div class="mt-12">
            <Button onClick={() => location.reload()}>重新加载</Button>
          </div>
        </Card>
      </div>
    );
  }

  const s = data.summary || {};
  const trend = data.trend || [];
  const vis = data.visibility || {
    sales: true,
    purchase: true,
    stock: true,
    finance: true,
    account: true,
    product: true,
    partner: true,
  };
  const hasOps = vis.sales || vis.purchase || vis.stock;
  const hasFinance = vis.finance || vis.account || vis.stock;
  const hasTrend = vis.sales || vis.purchase;
  const hasRecent = vis.sales || vis.purchase;
  const maxVal = Math.max(1, ...trend.map((t) => Math.max(Number(t.sales), Number(t.purchase))));

  return (
    <div class="page">
      {hasOps ? (
        <div class="stats">
          {vis.sales ? (
            <StatCard
              label="本月销售额"
              value={fmtMoney(s.month_sales, true)}
              icon="sell"
              color="blue"
              sub={`今日 ${fmtMoney(s.today_sales, true)}`}
            />
          ) : null}
          {vis.purchase ? (
            <StatCard
              label="本月采购额"
              value={fmtMoney(s.month_purchase, true)}
              icon="cart"
              color="orange"
              sub={`今日 ${fmtMoney(s.today_purchase, true)}`}
            />
          ) : null}
          {vis.sales ? (
            <StatCard
              label="本月毛利"
              value={fmtMoney(s.month_profit, true)}
              icon="chart"
              color="green"
              sub={
                s.month_sales > 0
                  ? `毛利率 ${((s.month_profit / s.month_sales) * 100).toFixed(1)}%`
                  : '—'
              }
            />
          ) : null}
          {vis.stock ? (
            <StatCard
              label="库存总值"
              value={fmtMoney(s.stock_amount, true)}
              icon="database"
              color="purple"
              sub={`${s.stock_kinds || 0} 个在库商品`}
            />
          ) : null}
        </div>
      ) : null}

      {hasFinance ? (
        <div class="stats">
          {vis.finance ? (
            <StatCard
              label="应收账款"
              value={fmtMoney(s.receivable, true)}
              icon="money"
              color="red"
              sub="客户欠款合计"
            />
          ) : null}
          {vis.finance ? (
            <StatCard
              label="应付账款"
              value={fmtMoney(s.payable, true)}
              icon="wallet"
              color="orange"
              sub="供应商欠款合计"
            />
          ) : null}
          {vis.account ? (
            <StatCard
              label="资金余额"
              value={fmtMoney(s.account_total, true)}
              icon="wallet"
              color="green"
              sub="全部资金账户"
            />
          ) : null}
          {vis.stock ? (
            <StatCard
              label="库存预警"
              value={String(s.low_stock_count || 0)}
              icon="alert"
              color="red"
              sub={[
                vis.product ? `商品 ${s.product_count || 0}` : '',
                vis.partner ? `往来单位 ${s.partner_count || 0}` : '',
              ]
                .filter(Boolean)
                .join(' · ')}
            />
          ) : null}
        </div>
      ) : null}

      {hasTrend || vis.stock ? (
        <div class="row" style="gap:14px; align-items:stretch; flex-wrap:wrap">
          {hasTrend ? (
            <Card
              title="近 14 天走势"
              icon="chart"
              className="grow"
              extra={
                <div class="legend">
                  {vis.sales ? (
                    <span>
                      <i style="background:var(--primary)" />
                      销售
                    </span>
                  ) : null}
                  {vis.purchase ? (
                    <span>
                      <i style="background:#f0a13a" />
                      采购
                    </span>
                  ) : null}
                </div>
              }
            >
              <div class="bar-chart">
                {trend.map((t) => (
                  <div
                    class="bar-item"
                    key={t.date}
                    title={[
                      t.date,
                      vis.sales ? `销售 ${fmtMoney(t.sales)}` : '',
                      vis.purchase ? `采购 ${fmtMoney(t.purchase)}` : '',
                      vis.sales ? `毛利 ${fmtMoney(t.profit)}` : '',
                    ]
                      .filter(Boolean)
                      .join('\n')}
                  >
                    <div class="bar-stack">
                      {vis.sales ? (
                        <div
                          class="bar bar-sales"
                          style={{ height: Math.max(2, (Number(t.sales) / maxVal) * 100) + '%' }}
                        />
                      ) : null}
                      {vis.purchase ? (
                        <div
                          class="bar bar-purchase"
                          style={{ height: Math.max(2, (Number(t.purchase) / maxVal) * 100) + '%' }}
                        />
                      ) : null}
                    </div>
                    <span class="bar-label">{t.date.slice(5)}</span>
                  </div>
                ))}
              </div>
            </Card>
          ) : null}

          {vis.stock ? (
            <Card
              title="库存预警"
              icon="alert"
              className="grow"
              extra={
                <Button
                  size="sm"
                  variant="ghost"
                  icon="chevronRight"
                  onClick={() => navigate('/alerts')}
                >
                  查看全部
                </Button>
              }
            >
              <Table
                compact
                empty="暂无低库存商品"
                rowKey="product_id"
                columns={[
                  {
                    title: '商品',
                    key: 'product_name',
                    render: (r) => (
                      <span>
                        {r.product_name}
                        <span class="muted small"> {r.spec}</span>
                      </span>
                    ),
                  },
                  { title: '仓库', key: 'warehouse_name' },
                  {
                    title: '结存',
                    align: 'right',
                    render: (r) => <span class="num">{fmtQty(r.qty)}</span>,
                  },
                  {
                    title: '安全库存',
                    align: 'right',
                    render: (r) => <span class="num">{fmtQty(r.safety_stock)}</span>,
                  },
                  {
                    title: '缺口',
                    align: 'right',
                    render: (r) => (
                      <Tag color="red">{fmtQty(Number(r.safety_stock) - Number(r.qty))}</Tag>
                    ),
                  },
                ]}
                data={(data.alerts || []).slice(0, 6)}
              />
            </Card>
          ) : null}
        </div>
      ) : null}

      {hasRecent ? (
        <Card title="最近单据" icon="ledger" bodyClass="no-pad">
          <Table
            empty="暂无单据"
            rowKey={(r) => r.doc_type + '-' + r.id}
            columns={[
              {
                title: '类型',
                key: 'doc_type',
                width: 90,
                render: (r) =>
                  r.doc_type === 'sales' ? (
                    <Tag color="blue">销售出库</Tag>
                  ) : (
                    <Tag color="orange">采购入库</Tag>
                  ),
              },
              { title: '单据编号', key: 'doc_no', className: 'num' },
              { title: '往来单位', key: 'partner' },
              { title: '日期', key: 'date', render: (r) => fmtDate(r.date) },
              { title: '金额', align: 'right', render: (r) => <Money value={r.amount} bold /> },
              {
                title: '状态',
                align: 'center',
                width: 80,
                render: (r) => {
                  const st = docStatus(r.status);
                  return <Tag color={st.color}>{st.text}</Tag>;
                },
              },
              {
                title: '操作',
                width: 70,
                align: 'center',
                render: (r) => (
                  <Button
                    size="sm"
                    variant="link"
                    onClick={() =>
                      navigate((r.doc_type === 'sales' ? '/sales?id=' : '/purchases?id=') + r.id)
                    }
                  >
                    查看
                  </Button>
                ),
              },
            ]}
            data={data.recent || []}
          />
        </Card>
      ) : null}

      {!hasOps && !hasFinance ? (
        <Card title="暂无可展示的看板数据">
          <p class="muted">
            当前账号只有看板访问权限，没有销售、采购、库存、资金或报表模块的查看权限。
          </p>
        </Card>
      ) : null}
    </div>
  );
}
