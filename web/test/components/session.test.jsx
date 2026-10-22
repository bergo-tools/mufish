import { h } from 'preact';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { waitFor } from '@testing-library/preact';

vi.mock('../../src/api.js', () => ({
  Api: { meta: vi.fn() },
}));

import { Api } from '../../src/api.js';
import { clearSession, session } from '../../src/store.js';

beforeEach(() => {
  Api.meta.mockReset();
});

function loggedIn(defaultLogin) {
  session.set({
    ready: true,
    user: { id: 1, username: 'admin' },
    permissions: ['*'],
    meta: { company_name: '测试公司', default_login: defaultLogin },
  });
}

describe('退出登录后的公开 meta', () => {
  it('立即隐藏默认口令提示，再按最新 meta 更新', async () => {
    loggedIn(true);
    Api.meta.mockResolvedValue({ company_name: '测试公司', logged_in: false, default_login: false });
    clearSession();

    expect(session.get().user).toBeNull();
    expect(session.get().meta.default_login).toBe(false);
    await waitFor(() => expect(Api.meta).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(session.get().meta.company_name).toBe('测试公司'));
    expect(session.get().meta.default_login).toBe(false);
  });

  it('普通退出后若仍使用默认口令，重新显示初始账号提示', async () => {
    loggedIn(false);
    Api.meta.mockResolvedValue({ company_name: '测试公司', logged_in: false, default_login: true });
    clearSession();
    expect(session.get().meta.default_login).toBe(false);

    await waitFor(() => expect(session.get().meta.default_login).toBe(true));
  });
});
