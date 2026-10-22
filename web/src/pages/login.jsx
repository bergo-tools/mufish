import { useState } from 'preact/hooks';
import { Api } from '../api.js';
import { navigate } from '../router.js';
import { firstAllowed } from '../nav.js';
import { session, setSession, useStore } from '../store.js';

export default function LoginPage() {
  const { meta } = useStore(session);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const submit = async (e) => {
    e.preventDefault();
    if (!username.trim() || !password) {
      setError('请输入用户名与口令');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const res = await Api.login(username.trim(), password);
      setSession(res.user, res.permissions || []);
      navigate(firstAllowed(), true);
    } catch (err) {
      setError(err.message || '登录失败');
      setLoading(false);
    }
  };

  return (
    <div class="login-page">
      <form class="login-card" onSubmit={submit}>
        <div class="login-brand">
          <span class="login-logo">木</span>
          <div>
            <h1>{meta.company_name || '木鱼进销存'}</h1>
            <p>简单易用的进销存管理</p>
          </div>
        </div>

        {error ? <div class="login-error">{error}</div> : null}

        <div class="login-form">
          <input
            class="input"
            name="username"
            aria-label="用户名"
            placeholder="用户名"
            autoFocus
            autocomplete="username"
            value={username}
            onInput={(e) => setUsername(e.target.value)}
          />
          <input
            class="input"
            name="password"
            type="password"
            aria-label="口令"
            placeholder="口令"
            autocomplete="current-password"
            value={password}
            onInput={(e) => setPassword(e.target.value)}
          />
          <button class="btn btn-primary btn-lg" type="submit" disabled={loading}>
            {loading ? '正在登录…' : '登 录'}
          </button>
        </div>

        <div class="login-tip">
          {meta.default_login ? (
            <>
              初始账号：<b>admin</b> / <b>admin123</b>
              <br />
              首次登录后请到「个人中心」修改口令。
            </>
          ) : (
            <span>请使用管理员分配的账号登录。</span>
          )}
          {meta.version ? <span> · v{meta.version}</span> : null}
        </div>
      </form>
    </div>
  );
}
