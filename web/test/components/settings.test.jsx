import { h } from 'preact';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/preact';

vi.mock('../../src/api.js', () => ({
  Api: {
    settings: vi.fn(),
    saveSettings: vi.fn(),
  },
}));

import { Api } from '../../src/api.js';
import SettingsPage from '../../src/pages/settings.jsx';
import { clearSession, setSession } from '../../src/store.js';

beforeEach(() => {
  setSession({ id: 1, username: 'admin', status: 1 }, ['setting:view', 'setting:manage']);
});

afterEach(() => {
  clearSession();
  vi.clearAllMocks();
});

describe('系统参数加载失败', () => {
  it('显示错误并提供重试，而不是用默认值允许保存', async () => {
    Api.settings.mockRejectedValueOnce(new Error('网络连接失败'));
    render(<SettingsPage route={{}} />);

    await waitFor(() => expect(screen.getByText('系统参数加载失败')).toBeInTheDocument());
    expect(screen.getByText('网络连接失败')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '重新加载' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '保存' })).not.toBeInTheDocument();
    expect(Api.saveSettings).not.toHaveBeenCalled();
  });
});
