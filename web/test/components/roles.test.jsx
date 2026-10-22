import { h } from 'preact';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/preact';

vi.mock('../../src/api.js', () => ({
  Api: {
    roles: vi.fn(),
    permissions: vi.fn(),
  },
}));

import { Api } from '../../src/api.js';
import RolesPage from '../../src/pages/roles.jsx';
import { clearSession, setSession } from '../../src/store.js';

beforeEach(() => {
  setSession({ id: 1, username: 'viewer', status: 1 }, ['role:view']);
  Api.roles.mockResolvedValue([
    { id: 1, code: 'viewer', name: '只读角色', perms: ['*'], is_system: false, user_count: 1 },
  ]);
  Api.permissions.mockResolvedValue({ catalog: [], all: [], super: '*' });
});

afterEach(() => {
  clearSession();
});

describe('角色权限只读模式', () => {
  it('缺少 role:manage 时超级管理员开关不可操作', async () => {
    render(<RolesPage />);
    const box = await screen.findByRole('checkbox', { name: /超级管理员/ });
    expect(box).toBeChecked();
    expect(box).toBeDisabled();
  });
});
