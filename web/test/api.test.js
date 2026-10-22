import test from 'node:test';
import assert from 'node:assert/strict';

import { Api, filenameFromDisposition } from '../src/api.js';

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

test('登录失败时保留服务端错误提示，不触发全局会话失效事件', async () => {
  const oldFetch = globalThis.fetch;
  const oldWindow = globalThis.window;
  const dispatched = [];
  globalThis.window = { dispatchEvent: (event) => dispatched.push(event.type) };
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ code: 401, msg: '用户名或口令错误' }), {
      status: 401,
      headers: { 'Content-Type': 'application/json' },
    });
  try {
    await assert.rejects(Api.login('admin', 'wrong'), (err) => {
      assert.equal(err.message, '用户名或口令错误');
      return true;
    });
    assert.deepEqual(dispatched, []);
  } finally {
    globalThis.fetch = oldFetch;
    globalThis.window = oldWindow;
  }
});
