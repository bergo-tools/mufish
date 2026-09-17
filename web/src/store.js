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

export function clearSession() {
  session.set({ user: null, permissions: [] });
}

// ---------------------------------------------------------------------------
// 消息提示
// ---------------------------------------------------------------------------

export const toasts = createStore({ items: [] });

let toastSeq = 0;

export function toast(message, type = 'info', duration = 2600) {
  const id = ++toastSeq;
  const items = [...toasts.get().items, { id, message, type }];
  toasts.set({ items });
  setTimeout(() => {
    toasts.set({ items: toasts.get().items.filter((t) => t.id !== id) });
  }, duration);
}

export const notifyError = (err) => toast(err && err.message ? err.message : String(err), 'error', 4200);
export const notifyOK = (msg) => toast(msg, 'success');
