import test from 'node:test';
import assert from 'node:assert/strict';

import { session } from '../src/store.js';
import { findTitle, firstAllowed, routePerm } from '../src/nav.js';

test('firstAllowed 按权限选择登录落地页', () => {
  const old = session.get();
  try {
    session.set({ permissions: ['product:view'] });
    assert.equal(firstAllowed(), '/products');
    session.set({ permissions: [] });
    assert.equal(firstAllowed(), '/profile');
    session.set({ permissions: ['*'] });
    assert.equal(firstAllowed(), '/');
  } finally {
    session.set(old);
  }
});

test('routePerm / findTitle 与导航配置一致', () => {
  assert.equal(routePerm('/users'), 'user:view');
  assert.equal(routePerm('/profile'), '');
  assert.equal(routePerm('/unknown'), '');
  assert.equal(findTitle('/sales'), '销售出库');
  assert.equal(findTitle('/profile'), '个人中心');
  assert.equal(findTitle('/unknown'), '木鱼进销存');
});
