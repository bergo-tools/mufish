// 导航配置与路由权限：独立成模块，供 App 与登录页共用。
import { can } from './store.js';

export const NAV = [
  { group: '概览', items: [{ key: '/', label: '首页看板', icon: 'dashboard', perm: 'dashboard:view' }] },
  {
    group: '基础资料',
    items: [
      { key: '/products', label: '商品管理', icon: 'box', perm: 'product:view' },
      { key: '/categories', label: '商品分类', icon: 'tag', perm: 'category:view' },
      { key: '/partners', label: '往来单位', icon: 'users', perm: 'partner:view' },
      { key: '/warehouses', label: '仓库管理', icon: 'warehouse', perm: 'warehouse:view' },
      { key: '/accounts', label: '资金账户', icon: 'wallet', perm: 'account:view' },
    ],
  },
  {
    group: '采购销售',
    items: [
      { key: '/purchases', label: '采购入库', icon: 'cart', perm: 'purchase:view' },
      { key: '/sales', label: '销售出库', icon: 'sell', perm: 'sales:view' },
    ],
  },
  {
    group: '库存管理',
    items: [
      { key: '/stocks', label: '库存查询', icon: 'database', perm: 'stock:view' },
      { key: '/ledger', label: '库存流水', icon: 'ledger', perm: 'stock:ledger' },
      { key: '/alerts', label: '库存预警', icon: 'alert', perm: 'stock:view' },
      { key: '/stock-takes', label: '库存盘点', icon: 'clipboard', perm: 'stock:take' },
      { key: '/transfers', label: '库存调拨', icon: 'transfer', perm: 'stock:transfer' },
    ],
  },
  {
    group: '资金往来',
    items: [{ key: '/payments', label: '收付款单', icon: 'money', perm: 'payment:view' }],
  },
  {
    group: '统计分析',
    items: [{ key: '/reports', label: '统计报表', icon: 'chart', perm: 'report:view' }],
  },
  {
    group: '系统管理',
    items: [
      { key: '/users', label: '用户管理', icon: 'users', perm: 'user:view' },
      { key: '/roles', label: '角色权限', icon: 'shield', perm: 'role:view' },
      { key: '/logs', label: '操作日志', icon: 'log', perm: 'log:view' },
      { key: '/settings', label: '系统参数', icon: 'gear', perm: 'setting:view' },
      { key: '/system', label: '数据维护', icon: 'database', perm: 'backup:run' },
    ],
  },
];

// 找出第一个有权限的页面，作为登录后的落地页。
export function firstAllowed() {
  for (const g of NAV) {
    for (const it of g.items) {
      if (!it.perm || can(it.perm)) return it.key;
    }
  }
  return '/profile';
}

// 路由需要的权限；个人中心和未知路径返回空。
export function routePerm(path) {
  for (const g of NAV) {
    for (const it of g.items) {
      if (it.key === path) return it.perm || '';
    }
  }
  return '';
}

export function findTitle(path) {
  for (const g of NAV) {
    for (const it of g.items) {
      if (it.key === path) return it.label;
    }
  }
  if (path === '/profile') return '个人中心';
  return '木鱼进销存';
}
