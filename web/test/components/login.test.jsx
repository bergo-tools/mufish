import { h } from 'preact';
import { act, render, screen } from '@testing-library/preact';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import LoginPage from '../../src/pages/login.jsx';
import { session } from '../../src/store.js';

function setMeta(meta) {
  session.set({
    ready: true,
    user: null,
    permissions: [],
    meta: { company_name: '测试公司', company_short: '测试', version: '0.7.0', ...meta },
  });
}

beforeEach(() => {
  setMeta({ default_login: false });
});

afterEach(() => {
  session.set({
    ready: false,
    user: null,
    permissions: [],
    meta: { company_name: '木鱼进销存', company_short: '木鱼', version: '' },
  });
});

describe('登录页', () => {
  it('管理员已修改默认口令时不展示 admin/admin123', () => {
    render(<LoginPage />);
    expect(screen.queryByText('admin123')).not.toBeInTheDocument();
    expect(screen.queryByText(/初始账号/)).not.toBeInTheDocument();
  });

  it('仍使用默认口令时展示初始账号提示', () => {
    act(() => setMeta({ default_login: true }));
    render(<LoginPage />);
    expect(screen.getByText('admin123')).toBeInTheDocument();
  });

  it('用户名和口令输入框有可访问名称', () => {
    render(<LoginPage />);
    expect(screen.getByLabelText('用户名')).toBeInTheDocument();
    expect(screen.getByLabelText('口令')).toBeInTheDocument();
  });
});
