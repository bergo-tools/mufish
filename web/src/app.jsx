// 应用外壳：侧边栏导航 + 顶栏 + 路由分发。
import { useEffect, useMemo, useState } from 'preact/hooks';
import { navigate, useRoute } from './router.js';
import { can, clearSession, session, toast, toasts, useStore } from './store.js';
import { firstAllowed, findTitle, NAV, routePerm } from './nav.js';
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

export default function App() {
  const state = useStore(session);
  const route = useRoute();

  // 任意接口返回 401 时清空本地会话并回到登录页，避免用户停留在
  // 一个所有操作都会报错的旧页面。
  useEffect(() => {
    const onUnauthorized = () => {
      if (!session.get().user) return;
      clearSession();
      navigate('/', true);
      toast('登录状态已失效，请重新登录', 'error', 4200);
    };
    window.addEventListener('mufish:unauthorized', onUnauthorized);
    return () => window.removeEventListener('mufish:unauthorized', onUnauthorized);
  }, []);

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
  const title = findTitle(path);
  const requiredPerm = routePerm(path);

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

  if (requiredPerm && !can(requiredPerm)) {
    return <NoAccess route={route} title={title} />;
  }

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

// 无权限直达时自动回到第一个有权限的页面，避免渲染业务数据。
function NoAccess({ route, title }) {
  useEffect(() => {
    toast('没有访问该页面的权限', 'error', 3600);
    navigate(firstAllowed(), true);
  }, []);
  return (
    <Shell route={route} title={title}>
      <div class="page">
        <div class="card">
          <div class="card-body">
            <p class="muted">没有访问该页面的权限，正在返回可用页面…</p>
          </div>
        </div>
      </div>
      <ToastHost store={toasts} />
      <ConfirmHost />
    </Shell>
  );
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
