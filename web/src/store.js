// 轻量全局状态管理（无需额外依赖）。
import { useEffect, useState } from 'preact/hooks';

export function createStore(initialState) {
  let state = initialState;
  const listeners = new Set();
  return {
    get: () => state,
    set(patch) {
      const next = typeof patch === 'function' ? patch(state) : { ...state, ...patch };
      if (next === state) return;
      state = next;
      listeners.forEach((fn) => fn(state));
    },
    subscribe(fn) {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
  };
}

// 在组件中订阅 store。
export function useStore(store) {
  const [, force] = useState(0);
  useEffect(() => store.subscribe(() => force((n) => n + 1)), [store]);
  return store.get();
}

// ---------------------------------------------------------------------------
// 会话
// ---------------------------------------------------------------------------

export const session = createStore({
  ready: false,
  user: null,
  permissions: [],
  meta: { company_name: '木鱼进销存', company_short: '木鱼', version: '' },
});

// 判断当前用户是否具备某权限（支持 "*"）。
export function can(code) {
  const perms = session.get().permissions || [];
  if (perms.includes('*')) return true;
  return perms.includes(code);
}

// 判断是否具备任意一个权限。
export function canAny(...codes) {
  return codes.some((c) => can(c));
}

export async function loadSession() {
  const { Api } = await import('./api.js');
  try {
    const meta = await Api.meta();
    if (meta.logged_in) {
      session.set({
        ready: true,
        user: meta.user,
        permissions: meta.permissions || [],
        meta,
      });
    } else {
      session.set({ ready: true, user: null, permissions: [], meta });
    }
  } catch (e) {
    session.set({ ready: true, user: null, permissions: [] });
  }
}

export function setSession(user, permissions) {
  session.set({ user, permissions, ready: true });
}

// 重新获取公开 meta（企业名、default_login 等）。退出/会话失效后调用，
// 避免登录页继续使用登录前的旧提示。
async function refreshPublicMeta() {
  try {
    const { Api } = await import('./api.js');
    const meta = await Api.meta();
    if (session.get().user) return; // 已经重新登录，忽略旧请求结果
    session.set({ ...session.get(), meta, ready: true });
  } catch (e) {
    // 获取失败时保留现有 meta；登录页至少不会误显示默认口令提示。
  }
}

export function clearSession() {
  const current = session.get();
  // 立即隐藏默认口令提示，不等异步刷新完成。正常退出后如果管理员
  // 仍在使用默认口令，refreshPublicMeta 会重新把提示显示出来。
  session.set({
    user: null,
    permissions: [],
    meta: { ...(current.meta || {}), default_login: false },
  });
  refreshPublicMeta();
}

// ---------------------------------------------------------------------------
// 消息提示
// ---------------------------------------------------------------------------

export const toasts = createStore({ items: [] });

let toastSeq = 0;

export function toast(message, type = 'info', duration = 2600) {
  const now = Date.now();
  const current = toasts.get().items;
  // 同一个错误可能同时被页面和 usePagedList 上报，短时间内去重。
  const last = current[current.length - 1];
  if (last && last.message === message && last.type === type && now - (last.at || 0) < 800) {
    return;
  }
  const id = ++toastSeq;
  const items = [...current, { id, message, type, at: now }];
  toasts.set({ items });
  setTimeout(() => {
    toasts.set({ items: toasts.get().items.filter((t) => t.id !== id) });
  }, duration);
}

export const notifyError = (err) => toast(err && err.message ? err.message : String(err), 'error', 4200);
export const notifyOK = (msg) => toast(msg, 'success');
