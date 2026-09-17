// 应用外壳：侧边栏导航 + 顶栏 + 路由分发。
import { useEffect, useMemo, useState } from 'preact/hooks';
import { navigate, useRoute } from './router.js';
import { can, clearSession, notifyError, session, toasts, useStore } from './store.js';
import { Api } from './api.js';
import { Button, ConfirmHost, Icon, ToastHost } from './ui.jsx';

import LoginPage from './pages/login.jsx';
import DashboardPage from './pages/dashboard.jsx';
import ProductsPage from './pages/products.jsx';
import CategoriesPage from './pages/categories.jsx';
import PartnersPage from './pages/partners.jsx';
import WarehousesPage from './pages/warehouses.jsx';
import AccountsPage from './pages/accounts.jsx';
import PurchasesPage from './pages/purchases.jsx';
import SalesPage from './pages/sales.jsx';
import StocksPage from './pages/stocks.jsx';
import LedgerPage from './pages/ledger.jsx';
import AlertsPage from './pages/alerts.jsx';
import StockTakesPage from './pages/stock-takes.jsx';
import TransfersPage from './pages/transfers.jsx';
import PaymentsPage from './pages/payments.jsx';
import ReportsPage from './pages/reports.jsx';
import UsersPage from './pages/users.jsx';
import RolesPage from './pages/roles.jsx';
import LogsPage from './pages/logs.jsx';
import SettingsPage from './pages/settings.jsx';
import SystemPage from './pages/system.jsx';
import ProfilePage from './pages/profile.jsx';

// 导航配置：perm 为空表示登录后即可访问。
const NAV = [
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

const ROUTES = {
  '/': DashboardPage,
  '/products': ProductsPage,
  '/categories': CategoriesPage,
  '/partners': PartnersPage,
  '/warehouses': WarehousesPage,
  '/accounts': AccountsPage,
  '/purchases': PurchasesPage,
  '/sales': SalesPage,
  '/stocks': StocksPage,
  '/ledger': LedgerPage,
  '/alerts': AlertsPage,
  '/stock-takes': StockTakesPage,
  '/transfers': TransfersPage,
  '/payments': PaymentsPage,
  '/reports': ReportsPage,
  '/users': UsersPage,
  '/roles': RolesPage,
  '/logs': LogsPage,
  '/settings': SettingsPage,
  '/system': SystemPage,
  '/profile': ProfilePage,
};

// 找出第一个有权限的页面，作为登录后的落地页。
function firstAllowed() {
  for (const g of NAV) {
    for (const it of g.items) {
      if (!it.perm || can(it.perm)) return it.key;
    }
  }
  return '/profile';
}

export default function App() {
  const state = useStore(session);
  const route = useRoute();

  if (!state.ready) {
    return (
      <div class="loading-screen">
        <span class="spinner" />
      </div>
    );
  }

  if (!state.user) {
    return (
      <>
        <LoginPage />
        <ToastHost store={toasts} />
      </>
    );
  }

  const path = route.path;
  const Page = ROUTES[path];

  if (!Page) {
    return (
      <Shell route={route} title="页面不存在">
        <div class="page">
          <div class="card">
            <div class="card-body">
              <p>未找到路径 {path}，请从左侧菜单重新选择。</p>
              <div class="mt-12">
                <Button variant="primary" onClick={() => navigate(firstAllowed())}>
                  返回首页
                </Button>
              </div>
            </div>
          </div>
        </div>
        <ToastHost store={toasts} />
        <ConfirmHost />
      </Shell>
    );
  }

  const title = findTitle(path);
  return (
    <>
      <Shell route={route} title={title}>
        <Page route={route} />
      </Shell>
      <ToastHost store={toasts} />
      <ConfirmHost />
    </>
  );
}

function findTitle(path) {
  for (const g of NAV) {
    for (const it of g.items) {
      if (it.key === path) return it.label;
    }
  }
  if (path === '/profile') return '个人中心';
  return '木鱼进销存';
}

function Shell({ route, title, children }) {
  const state = useStore(session);
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    setMenuOpen(false);
    const el = document.querySelector('.content');
    if (el) el.scrollTop = 0;
  }, [route.path]);

  const groups = useMemo(
    () =>
      NAV.map((g) => ({ ...g, items: g.items.filter((it) => !it.perm || can(it.perm)) })).filter(
        (g) => g.items.length > 0,
      ),
    [state.permissions],
  );

  const logout = async () => {
    try {
      await Api.logout();
    } catch (e) {
      /* 忽略 */
    }
    clearSession();
    navigate('/');
  };

  const user = state.user || {};
  const initial = (user.name || user.username || '?').slice(0, 1);

  return (
    <div class="layout">
      <aside class={'sidebar ' + (menuOpen ? 'open' : '')}>
        <div class="brand">
          <span class="brand-logo">木</span>
          <span>木鱼进销存</span>
        </div>
        <nav class="nav">
          {groups.map((g) => (
            <div key={g.group}>
              <div class="nav-group">{g.group}</div>
              {g.items.map((it) => (
                <div
                  key={it.key}
                  class={'nav-item ' + (isActive(route.path, it.key) ? 'active' : '')}
                  onClick={() => navigate(it.key)}
                >
                  <Icon name={it.icon} size={15} />
                  <span>{it.label}</span>
                </div>
              ))}
            </div>
          ))}
        </nav>
      </aside>

      <div class="main">
        <header class="header">
          <button class="icon-btn no-print" onClick={() => setMenuOpen((v) => !v)} title="菜单">
            <Icon name="menu" size={18} />
          </button>
          <h2>{title}</h2>
          <div class="header-right">
            <div class="user-chip" onClick={() => navigate('/profile')} title="个人中心">
              <span class="avatar">{initial}</span>
              <span class="user-meta">
                <span class="user-name">{user.name || user.username}</span>
                <span class="user-role">{user.role_name || (user.is_super ? '超级管理员' : '')}</span>
              </span>
            </div>
            <button class="icon-btn no-print" onClick={logout} title="退出登录">
              <Icon name="logout" size={17} />
            </button>
          </div>
        </header>
        <main class="content">{children}</main>
      </div>
    </div>
  );
}

function isActive(current, key) {
  if (key === '/') return current === '/';
  return current === key || current.startsWith(key + '/');
}
