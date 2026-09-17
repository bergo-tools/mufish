// 基于 hash 的极简路由。
import { useEffect, useState } from 'preact/hooks';

export function currentPath() {
  const h = window.location.hash.replace(/^#/, '');
  return h || '/';
}

export function navigate(to, replace = false) {
  const target = '#' + (to.startsWith('/') ? to : '/' + to);
  if (replace) {
    window.history.replaceState(null, '', target);
    window.dispatchEvent(new HashChangeEvent('hashchange'));
  } else {
    window.location.hash = target;
  }
}

export function useRoute() {
  const [path, setPath] = useState(currentPath);
  useEffect(() => {
    const onChange = () => setPath(currentPath());
    window.addEventListener('hashchange', onChange);
    return () => window.removeEventListener('hashchange', onChange);
  }, []);

  const [pathname, queryString] = path.split('?');
  const segments = pathname.split('/').filter(Boolean);
  const query = {};
  if (queryString) {
    new URLSearchParams(queryString).forEach((v, k) => {
      query[k] = v;
    });
  }
  return { path: pathname, segments, query };
}

// 拼接查询字符串。
export function buildQuery(params) {
  const search = new URLSearchParams();
  Object.entries(params || {}).forEach(([k, v]) => {
    if (v === undefined || v === null || v === '') return;
    search.append(k, String(v));
  });
  const s = search.toString();
  return s ? '?' + s : '';
}
