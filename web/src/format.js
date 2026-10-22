// 格式化助手。

export function fmtMoney(v, withSymbol = false) {
  const n = Number(v || 0);
  const s = n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  return withSymbol ? '¥' + s : s;
}

// 金额（分）与“数量 × 单价”的精确计算。
//
// 后端 AmountOf 会先把数量换算成 4 位小数整数、单价换算成万分之一元
// 整数，再按分四舍五入；前端合计必须使用同一规则，否则多条半分边界
// 明细会出现“每行显示 0.04、合计却显示 0.07”的差异。
export function moneyToCents(v) {
  const n = Number(v);
  return Number.isFinite(n) ? Math.round(n * 100) : 0;
}

export function moneyFromCents(cents) {
  return Number(cents || 0) / 100;
}

export function amountCents(qty, price) {
  const q = Number(qty);
  const p = Number(price);
  if (!Number.isFinite(q) || !Number.isFinite(p)) return 0;
  const q4 = Math.round(q * 10000);
  const p4 = Math.round(p * 10000);
  if (q4 === 0 || p4 === 0) return 0;
  const product = BigInt(q4) * BigInt(p4);
  const denominator = 1000000n;
  const half = 500000n;
  const rounded = product >= 0n
    ? (product + half) / denominator
    : (product - half) / denominator;
  return Number(rounded);
}

export function sumAmountCents(items, qtyKey = 'qty', priceKey = 'price') {
  return (items || []).reduce(
    (sum, item) => sum + amountCents(item && item[qtyKey], item && item[priceKey]),
    0,
  );
}

export function fmtQty(v, digits = 4) {
  const n = Number(v || 0);
  if (Number.isInteger(n)) return String(n);
  const s = n.toFixed(digits).replace(/\.?0+$/, '');
  return s;
}

export function fmtNumber(v) {
  return Number(v || 0).toLocaleString('zh-CN');
}

export function fmtPercent(v, digits = 2) {
  return Number(v || 0).toFixed(digits) + '%';
}

export function fmtSize(bytes) {
  const n = Number(bytes || 0);
  if (n < 1024) return n + ' B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
  if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(2) + ' MB';
  return (n / 1024 / 1024 / 1024).toFixed(2) + ' GB';
}

export function fmtUptime(seconds) {
  const s = Number(seconds || 0);
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d} 天 ${h} 小时`;
  if (h > 0) return `${h} 小时 ${m} 分`;
  return `${m} 分 ${s % 60} 秒`;
}

function pad(n) {
  return n < 10 ? '0' + n : String(n);
}

export function today() {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export function monthStart() {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-01`;
}

export function daysAgo(n) {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export function fmtDateTime(v) {
  if (!v) return '-';
  return String(v).slice(0, 19);
}

export function fmtDate(v) {
  if (!v) return '-';
  return String(v).slice(0, 10);
}

// 单据状态
export const DOC_STATUS = {
  0: { text: '草稿', color: 'gray' },
  1: { text: '已确认', color: 'green' },
  2: { text: '已作废', color: 'red' },
};

export function docStatus(status) {
  return DOC_STATUS[status] || { text: '未知', color: 'gray' };
}

export const BIZ_TYPE_TEXT = {
  purchase: '采购入库',
  sales: '销售出库',
  take: '库存盘点',
  transfer_in: '调拨入库',
  transfer_out: '调拨出库',
  init: '期初库存',
  adjust: '手工调整',
};

export const PAY_TYPE_TEXT = { receipt: '收款', payment: '付款' };
export const METHOD_TEXT = {
  cash: '现金',
  bank: '银行转账',
  alipay: '支付宝',
  wechat: '微信',
  offset: '往来冲抵',
};
export const PARTNER_TYPE_TEXT = { customer: '客户', supplier: '供应商', both: '客户/供应商' };
export const ACCOUNT_TYPE_TEXT = { cash: '现金', bank: '银行', alipay: '支付宝', wechat: '微信', other: '其他' };
