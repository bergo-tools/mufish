import test from 'node:test';
import assert from 'node:assert/strict';

import { filenameFromDisposition } from '../src/api.js';

test('filenameFromDisposition 优先解析 filename*', () => {
  const header =
    "attachment; filename=products.csv; filename*=UTF-8''%E5%95%86%E5%93%81%E6%B8%85%E5%8D%95.csv";
  assert.equal(filenameFromDisposition(header), '商品清单.csv');
});

test('filenameFromDisposition 回退到 filename 或默认名', () => {
  assert.equal(filenameFromDisposition('attachment; filename="report.csv"'), 'report.csv');
  assert.equal(filenameFromDisposition('', '导出.csv'), '导出.csv');
  assert.equal(filenameFromDisposition('attachment'), '导出.csv');
});
