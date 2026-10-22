// 前端纯函数单元测试：使用 Node 内置 test runner，无需额外依赖。
import test from 'node:test';
import assert from 'node:assert/strict';

import {
  fmtMoney,
  fmtQty,
  fmtNumber,
  fmtPercent,
  fmtSize,
  fmtUptime,
  fmtDateTime,
  fmtDate,
  docStatus,
  DOC_STATUS,
  BIZ_TYPE_TEXT,
  PAY_TYPE_TEXT,
  METHOD_TEXT,
  PARTNER_TYPE_TEXT,
  ACCOUNT_TYPE_TEXT,
} from '../src/format.js';

test('fmtMoney 格式化金额', () => {
  assert.equal(fmtMoney(1234.5), '1,234.50');
  assert.equal(fmtMoney('12.3'), '12.30');
  assert.equal(fmtMoney(0), '0.00');
  assert.equal(fmtMoney(null), '0.00');
  assert.equal(fmtMoney(12.3, true), '¥12.30');
});

test('fmtQty 格式化数量', () => {
  assert.equal(fmtQty(3), '3');
  assert.equal(fmtQty(0), '0');
  assert.equal(fmtQty(1.2), '1.2');
  assert.equal(fmtQty(1.23456), '1.2346');
  assert.equal(fmtQty(-1.5), '-1.5');
  assert.equal(fmtQty(1.23456, 2), '1.23');
});

test('fmtNumber / fmtPercent', () => {
  assert.equal(fmtNumber(1234), '1,234');
  assert.equal(fmtNumber(null), '0');
  assert.equal(fmtPercent(12.345), '12.35%');
  assert.equal(fmtPercent(0), '0.00%');
});

test('fmtSize 格式化字节数', () => {
  assert.equal(fmtSize(0), '0 B');
  assert.equal(fmtSize(512), '512 B');
  assert.equal(fmtSize(1024), '1.0 KB');
  assert.equal(fmtSize(1024 * 1024), '1.00 MB');
  assert.equal(fmtSize(1024 * 1024 * 1024), '1.00 GB');
});

test('fmtUptime 格式化运行时长', () => {
  assert.equal(fmtUptime(0), '0 分 0 秒');
  assert.equal(fmtUptime(59), '0 分 59 秒');
  assert.equal(fmtUptime(60), '1 分 0 秒');
  assert.equal(fmtUptime(3600), '1 小时 0 分');
  assert.equal(fmtUptime(90000), '1 天 1 小时');
});

test('日期时间格式化', () => {
  assert.equal(fmtDateTime('2026-01-02 03:04:05'), '2026-01-02 03:04:05');
  assert.equal(fmtDateTime('2026-01-02 03:04:05.123'), '2026-01-02 03:04:05');
  assert.equal(fmtDateTime(''), '-');
  assert.equal(fmtDate('2026-01-02 03:04:05'), '2026-01-02');
  assert.equal(fmtDate(null), '-');
});

test('单据状态与文案映射', () => {
  assert.deepEqual(docStatus(0), DOC_STATUS[0]);
  assert.equal(docStatus(0).text, '草稿');
  assert.equal(docStatus(1).text, '已确认');
  assert.equal(docStatus(2).text, '已作废');
  assert.equal(docStatus(99).text, '未知');

  assert.equal(BIZ_TYPE_TEXT.purchase, '采购入库');
  assert.equal(BIZ_TYPE_TEXT.sales, '销售出库');
  assert.equal(BIZ_TYPE_TEXT.transfer_in, '调拨入库');
  assert.equal(PAY_TYPE_TEXT.receipt, '收款');
  assert.equal(PAY_TYPE_TEXT.payment, '付款');
  assert.equal(METHOD_TEXT.bank, '银行转账');
  assert.equal(PARTNER_TYPE_TEXT.both, '客户/供应商');
  assert.equal(ACCOUNT_TYPE_TEXT.wechat, '微信');
});
