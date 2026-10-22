// 统计报表：进销存汇总、销售 / 采购排行、客户 / 供应商排行、应收应付、购销明细、资金流水。
import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { can, notifyError, notifyOK } from '../store.js';
import {
  PAY_TYPE_TEXT,
  METHOD_TEXT,
  fmtDate,
  fmtMoney,
  fmtPercent,
  fmtQty,
  monthStart,
  today,
} from '../format.js';
import {
  Button,
  Card,
  DateRange,
  Money,
  SearchBox,
  Select,
  Table,
  Tabs,
  Tag,
  Toolbar,
  useAsync,
} from '../ui.jsx';

// 报表页签。
const TABS = [
  { key: 'inout', label: '进销存汇总' },
  { key: 'salesRank', label: '销售排行' },
  { key: 'purchaseRank', label: '采购排行' },
  { key: 'customerRank', label: '客户排行' },
  { key: 'supplierRank', label: '供应商排行' },
  { key: 'arap', label: '应收应付' },
  { key: 'salesDetail', label: '销售明细' },
  { key: 'purchaseDetail', label: '采购明细' },
  { key: 'cashFlow', label: '资金流水' },
];

// 汇总求和（数量按最多 4 位小数展示）。
function sumOf(rows, key) {
  return (rows || []).reduce((acc, r) => acc + Number(r[key] || 0), 0);
}

// 表格底部合计行。
function totalRow(cols, label, cells) {
  return (
    <tr>
      {cols.map((c, i) => (
        <td key={i} style={{ textAlign: c.align || 'left' }} class={c.className || ''}>
          {i === 0 ? label : cells[c.key] !== undefined ? cells[c.key] : null}
        </td>
      ))}
    </tr>
  );
}

// 商品单元格：名称 + 编码 / 规格。
function productCell(r) {
  return (
    <span>
      {r.name}
      <span class="muted small"> {r.code}</span>
      {r.spec ? <span class="muted small"> / {r.spec}</span> : null}
    </span>
  );
}

export default function ReportsPage() {
  const [tab, setTab] = useState('inout');
  const [from, setFrom] = useState(monthStart());
  const [to, setTo] = useState(today());
  const [warehouseId, setWarehouseId] = useState('');
  const [categoryId, setCategoryId] = useState('');
  const [partnerId, setPartnerId] = useState('');
  const [accountId, setAccountId] = useState('');
  const [arapType, setArapType] = useState('receivable');
  // keyword 是已提交的查询条件；keywordInput 只保存输入框内容。
  // 两者分离可避免每输入一个字符就刷新一次报表。
  const [keyword, setKeyword] = useState('');
  const [keywordInput, setKeywordInput] = useState('');
  const [partnerKw, setPartnerKw] = useState('');

  // 下拉数据：仓库 / 分类 / 客户 / 供应商 / 资金账户。
  const {
    data: optionsData,
    error: optionsError,
    reload: reloadOptions,
    setData: setOptionsData,
  } = useAsync(async () => {
    const [warehouses, categories, customers, suppliers, accounts] = await Promise.all([
      Api.warehouseOptions(),
      Api.categoriesFlat(),
      Api.partnerOptions('customer'),
      Api.partnerOptions('supplier'),
      Api.accountOptions(),
    ]);
    return {
      warehouses: warehouses || [],
      categories: categories || [],
      customers: customers || [],
      suppliers: suppliers || [],
      accounts: accounts || [],
    };
  }, []);

  const opt = optionsData || {};

  // 往来单位下拉默认最多 300 条；报表筛选需要能搜索到更后面的单位。
  const searchReportPartners = async () => {
    const kind = tab === 'salesDetail' ? 'customer' : 'supplier';
    try {
      const partners = await Api.partnerOptions(kind, partnerKw);
      setOptionsData((prev) => {
        const key = kind === 'customer' ? 'customers' : 'suppliers';
        const merged = new Map();
        (((prev || {})[key]) || []).forEach((p) => merged.set(p.id, p));
        (partners || []).forEach((p) => merged.set(p.id, p));
        return { ...(prev || {}), [key]: Array.from(merged.values()) };
      });
    } catch (e) {
      notifyError(e);
    }
  };

  // 按当前页签加载数据（数组型报表）。
  const report = useAsync(async () => {
    if (tab === 'inout') {
      return Api.reportInout({ from, to, warehouse_id: warehouseId, category_id: categoryId, keyword });
    }
    if (tab === 'salesRank') {
      return Api.reportSalesRank({ from, to, limit: 100 });
    }
    if (tab === 'purchaseRank') {
      return Api.reportPurchaseRank({ from, to, limit: 100 });
    }
    if (tab === 'customerRank') {
      return Api.reportCustomerRank({ from, to, limit: 100 });
    }
    if (tab === 'supplierRank') {
      return Api.reportSupplierRank({ from, to, limit: 100 });
    }
    if (tab === 'arap') {
      return Api.reportArAp({ type: arapType, keyword });
    }
    if (tab === 'salesDetail') {
      return Api.reportSalesDetail({
        from,
        to,
        warehouse_id: warehouseId,
        partner_id: partnerId,
        keyword,
        limit: 1000,
      });
    }
    if (tab === 'purchaseDetail') {
      return Api.reportPurchaseDetail({
        from,
        to,
        warehouse_id: warehouseId,
        partner_id: partnerId,
        keyword,
        limit: 1000,
      });
    }
    if (tab === 'cashFlow') {
      return Api.reportCashFlow({ from, to, keyword, account_id: accountId, limit: 500 });
    }
    return [];
  }, [tab, from, to, warehouseId, categoryId, partnerId, accountId, arapType, keyword]);

  const rows = report.data || [];
  const reportLimit =
    tab === 'salesDetail' || tab === 'purchaseDetail' ? 1000 : tab === 'cashFlow' ? 500 : 0;
  const truncated = reportLimit > 0 && rows.length >= reportLimit;

  // 导出处理：统一捕获异常并提示。
  const doExport = async (fn, params) => {
    try {
      await fn(params);
      notifyOK('导出成功');
    } catch (e) {
      notifyError(e);
    }
  };

  const exportBtn = (fn, params) =>
    can('report:export') ? (
      <Button size="sm" variant="ghost" icon="download" onClick={() => doExport(fn, params)}>
        导出
      </Button>
    ) : null;

  // 当前报表的列定义。
  let columns = [];
  let footer = null;
  let extra = null;

  if (tab === 'inout') {
    columns = [
      { title: '商品编码', key: 'code', width: 120 },
      { title: '名称', key: 'name' },
      { title: '规格', key: 'spec', width: 110 },
      { title: '单位', key: 'unit', width: 60 },
      { title: '期初数量', key: 'init_qty', align: 'right', render: (r) => fmtQty(r.init_qty) },
      { title: '期初金额', key: 'init_amount', align: 'right', render: (r) => <Money value={r.init_amount} /> },
      { title: '入库数量', key: 'in_qty', align: 'right', render: (r) => fmtQty(r.in_qty) },
      { title: '入库金额', key: 'in_amount', align: 'right', render: (r) => <Money value={r.in_amount} /> },
      { title: '出库数量', key: 'out_qty', align: 'right', render: (r) => fmtQty(r.out_qty) },
      { title: '出库金额', key: 'out_amount', align: 'right', render: (r) => <Money value={r.out_amount} /> },
      { title: '期末数量', key: 'end_qty', align: 'right', render: (r) => fmtQty(r.end_qty) },
      {
        title: '期末金额',
        key: 'end_amount',
        align: 'right',
        render: (r) => <Money value={r.end_amount} bold />,
      },
    ];
    footer = totalRow(columns, '合计', {
      init_qty: fmtQty(sumOf(rows, 'init_qty')),
      init_amount: <Money value={sumOf(rows, 'init_amount')} />,
      in_qty: fmtQty(sumOf(rows, 'in_qty')),
      in_amount: <Money value={sumOf(rows, 'in_amount')} />,
      out_qty: fmtQty(sumOf(rows, 'out_qty')),
      out_amount: <Money value={sumOf(rows, 'out_amount')} />,
      end_qty: fmtQty(sumOf(rows, 'end_qty')),
      end_amount: <Money value={sumOf(rows, 'end_amount')} bold />,
    });
    extra = exportBtn(Api.exportInout, {
      from,
      to,
      warehouse_id: warehouseId,
      category_id: categoryId,
      keyword,
    });
  } else if (tab === 'salesRank') {
    columns = [
      { title: '商品', key: 'name', render: productCell },
      { title: '数量', key: 'qty', align: 'right', render: (r) => fmtQty(r.qty) },
      { title: '销售金额', key: 'amount', align: 'right', render: (r) => <Money value={r.amount} bold /> },
      { title: '成本', key: 'cost', align: 'right', render: (r) => <Money value={r.cost} /> },
      { title: '毛利', key: 'profit', align: 'right', render: (r) => <Money value={r.profit} colored bold /> },
      { title: '毛利率', key: 'rate', align: 'right', render: (r) => fmtPercent(r.rate) },
      { title: '单据数', key: 'doc_count', align: 'right' },
    ];
  } else if (tab === 'purchaseRank') {
    columns = [
      { title: '商品', key: 'name', render: productCell },
      { title: '数量', key: 'qty', align: 'right', render: (r) => fmtQty(r.qty) },
      { title: '采购金额', key: 'amount', align: 'right', render: (r) => <Money value={r.amount} bold /> },
      { title: '单据数', key: 'doc_count', align: 'right' },
    ];
  } else if (tab === 'customerRank') {
    columns = [
      { title: '客户', key: 'name' },
      { title: '数量', key: 'qty', align: 'right', render: (r) => fmtQty(r.qty) },
      { title: '销售金额', key: 'amount', align: 'right', render: (r) => <Money value={r.amount} bold /> },
      { title: '成本', key: 'cost', align: 'right', render: (r) => <Money value={r.cost} /> },
      { title: '毛利', key: 'profit', align: 'right', render: (r) => <Money value={r.profit} colored bold /> },
      { title: '毛利率', key: 'rate', align: 'right', render: (r) => fmtPercent(r.rate) },
      { title: '单据数', key: 'doc_count', align: 'right' },
    ];
  } else if (tab === 'supplierRank') {
    columns = [
      { title: '供应商', key: 'name' },
      { title: '数量', key: 'qty', align: 'right', render: (r) => fmtQty(r.qty) },
      { title: '采购金额', key: 'amount', align: 'right', render: (r) => <Money value={r.amount} bold /> },
      { title: '单据数', key: 'doc_count', align: 'right' },
    ];
  } else if (tab === 'arap') {
    columns = [
      { title: '单位名称', key: 'partner_name' },
      { title: '联系人', key: 'contact', width: 100 },
      { title: '电话', key: 'phone', width: 130 },
      { title: '期初金额', key: 'init_amount', align: 'right', render: (r) => <Money value={r.init_amount} /> },
      { title: '累计发生', key: 'doc_amount', align: 'right', render: (r) => <Money value={r.doc_amount} /> },
      { title: '已结算', key: 'paid_amount', align: 'right', render: (r) => <Money value={r.paid_amount} /> },
      {
        title: '期末余额',
        key: 'balance',
        align: 'right',
        render: (r) => <Money value={r.balance} colored bold />,
      },
    ];
    footer = totalRow(columns, '合计', {
      init_amount: <Money value={sumOf(rows, 'init_amount')} />,
      doc_amount: <Money value={sumOf(rows, 'doc_amount')} />,
      paid_amount: <Money value={sumOf(rows, 'paid_amount')} />,
      balance: <Money value={sumOf(rows, 'balance')} colored bold />,
    });
    extra = exportBtn(Api.exportArAp, { type: arapType, keyword });
  } else if (tab === 'salesDetail' || tab === 'purchaseDetail') {
    columns = [
      { title: '单据编号', key: 'doc_no', width: 150, render: (r) => r.doc_no || '-' },
      { title: '单据日期', key: 'doc_date', width: 104, render: (r) => fmtDate(r.doc_date) },
      { title: '商品编码', key: 'product_code', width: 120 },
      { title: '名称', key: 'product_name' },
      { title: '规格', key: 'spec', width: 110 },
      { title: '单位', key: 'unit', width: 60 },
      { title: '数量', key: 'qty', align: 'right', render: (r) => fmtQty(r.qty) },
      { title: '单价', key: 'price', align: 'right', render: (r) => <Money value={r.price} /> },
      { title: '金额', key: 'amount', align: 'right', render: (r) => <Money value={r.amount} bold /> },
      { title: '备注', key: 'remark' },
    ];
    footer = totalRow(columns, '合计', {
      qty: fmtQty(sumOf(rows, 'qty')),
      amount: <Money value={sumOf(rows, 'amount')} bold />,
    });
  } else if (tab === 'cashFlow') {
    columns = [
      { title: '日期', key: 'pay_date', width: 110, render: (r) => fmtDate(r.pay_date) },
      { title: '单据编号', key: 'payment_no', width: 160 },
      {
        title: '类型',
        key: 'type',
        width: 80,
        align: 'center',
        render: (r) => (
          <Tag color={r.type === 'receipt' ? 'green' : 'orange'}>{PAY_TYPE_TEXT[r.type] || r.type}</Tag>
        ),
      },
      { title: '往来单位', key: 'partner_name' },
      { title: '账户', key: 'account_name', width: 120 },
      { title: '方式', key: 'method', width: 100, render: (r) => METHOD_TEXT[r.method] || r.method || '-' },
      {
        title: '金额',
        key: 'amount',
        align: 'right',
        render: (r) => (
          <span class={r.type === 'receipt' ? 'money money-pos' : 'money money-neg'}>
            {fmtMoney(r.type === 'receipt' ? r.amount : -Number(r.amount || 0))}
          </span>
        ),
      },
      { title: '关联单据', key: 'order_no', render: (r) => r.order_no || <span class="muted">—</span> },
      { title: '备注', key: 'remark' },
    ];
    footer = totalRow(columns, '合计', {
      amount: (
        <span class="money money-bold">
          {fmtMoney(sumOf(rows.filter((r) => r.type === 'receipt'), 'amount') -
            sumOf(rows.filter((r) => r.type === 'payment'), 'amount'))}
        </span>
      ),
    });
  }

  // 顶部筛选区：仓库 / 分类仅进销存汇总与购销明细使用；往来单位用于明细。
  const showWarehouse = tab === 'inout' || tab === 'salesDetail' || tab === 'purchaseDetail';
  const showCategory = tab === 'inout';
  const showPartner = tab === 'salesDetail' || tab === 'purchaseDetail';
  const showAccount = tab === 'cashFlow';
  const showKeyword =
    tab !== 'salesRank' && tab !== 'purchaseRank' && tab !== 'customerRank' && tab !== 'supplierRank';
  // 应收应付是累计余额口径，不使用日期；其余报表（含客户/供应商排行）都按日期范围统计。
  const showDate = tab !== 'arap';

  const partnerOptions = (tab === 'salesDetail' ? opt.customers : opt.suppliers) || [];

  return (
    <div class="page">
      <Card bodyClass="no-pad">
        <Tabs
          tabs={TABS}
          active={tab}
          onChange={(k) => {
            setTab(k);
            // 客户 ID 与供应商 ID 不能跨页签复用，切换时统一清空。
            setPartnerId('');
            setPartnerKw('');
          }}
        />

        {optionsError ? (
          <div class="row-between" style="padding:10px 14px; gap:10px; flex-wrap:wrap">
            <span class="muted small">筛选下拉加载失败：{optionsError.message}</span>
            <Button size="sm" icon="refresh" onClick={() => reloadOptions()}>
              重试
            </Button>
          </div>
        ) : null}

        <Toolbar>
          {showDate ? (
            <DateRange from={from} to={to} onFrom={setFrom} onTo={setTo} />
          ) : null}
          {showWarehouse ? (
            <Select
              value={warehouseId}
              onInput={setWarehouseId}
              options={(opt.warehouses || []).map((w) => ({ value: String(w.id), label: w.name }))}
              placeholder="全部仓库"
            />
          ) : null}
          {showCategory ? (
            <Select
              value={categoryId}
              onInput={setCategoryId}
              options={(opt.categories || []).map((c) => ({ value: String(c.id), label: c.name }))}
              placeholder="全部分类"
            />
          ) : null}
          {showPartner ? (
            <div class="row" style="gap:6px">
              <Select
                value={partnerId}
                onInput={setPartnerId}
                options={partnerOptions.map((p) => ({ value: String(p.id), label: p.name }))}
                placeholder={tab === 'salesDetail' ? '全部客户' : '全部供应商'}
              />
              <SearchBox
                value={partnerKw}
                onInput={setPartnerKw}
                onSearch={searchReportPartners}
                placeholder={tab === 'salesDetail' ? '搜索客户' : '搜索供应商'}
                width={160}
              />
              <Button size="sm" icon="search" onClick={searchReportPartners}>
                搜索
              </Button>
            </div>
          ) : null}
          {showAccount ? (
            <Select
              value={accountId}
              onInput={setAccountId}
              options={(opt.accounts || []).map((a) => ({ value: String(a.id), label: a.name }))}
              placeholder="全部账户"
            />
          ) : null}
          {tab === 'arap' ? (
            <Select
              value={arapType}
              onInput={setArapType}
              options={[
                { value: 'receivable', label: '应收账款' },
                { value: 'payable', label: '应付账款' },
              ]}
              placeholder={null}
            />
          ) : null}
          {showKeyword ? (
            <SearchBox
              value={keywordInput}
              onInput={setKeywordInput}
              onSearch={() => setKeyword(keywordInput.trim())}
              onClear={() => {
                setKeywordInput('');
                setKeyword('');
              }}
              placeholder="商品 / 单位 / 单据号"
              width={220}
            />
          ) : null}
          <Button variant="ghost" icon="refresh" onClick={() => report.reload()}>
            刷新
          </Button>
        </Toolbar>

      </Card>

      <Card
        title={TABS.find((t) => t.key === tab).label}
        icon="chart"
        extra={extra}
        bodyClass="no-pad"
      >
        {truncated ? (
          <div class="muted small" style="padding:8px 14px">
            <Tag color="orange">已截断</Tag> 当前报表最多显示 {reportLimit} 条，请缩小日期范围。
          </div>
        ) : null}
        {report.error ? (
          <div class="row-between" style="padding:12px 14px; gap:10px; flex-wrap:wrap">
            <span class="muted small">报表加载失败：{report.error.message}</span>
            <Button size="sm" icon="refresh" onClick={() => report.reload()}>
              重试
            </Button>
          </div>
        ) : (
          <Table
            columns={columns}
            data={rows}
            loading={report.loading}
            empty="暂无数据"
            footer={footer}
          />
        )}
      </Card>
    </div>
  );
}
