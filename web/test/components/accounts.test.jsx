import { h } from 'preact';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/preact';

vi.mock('../../src/api.js', () => ({
  Api: {
    accounts: vi.fn(),
    saveAccount: vi.fn(),
    adjustAccount: vi.fn(),
    accountAdjustments: vi.fn(),
    deleteAccount: vi.fn(),
  },
}));

import { Api } from '../../src/api.js';
import AccountsPage from '../../src/pages/accounts.jsx';
import { clearSession, setSession } from '../../src/store.js';

const account = {
  id: 1,
  code: 'ACC001',
  name: '现金账户',
  type: 'cash',
  balance: 100,
  is_default: true,
  sort: 1,
  status: 1,
  remark: '',
};

beforeEach(() => {
  setSession({ id: 1, username: 'admin', status: 1 }, ['account:view', 'account:manage']);
  Api.accounts.mockReset().mockResolvedValue([account]);
  Api.saveAccount.mockReset().mockResolvedValue({ id: 1 });
  Api.adjustAccount.mockReset().mockResolvedValue({ id: 1 });
  Api.accountAdjustments.mockReset().mockResolvedValue([
    {
      id: 1,
      account_id: 1,
      account_name: '现金账户',
      before_balance: 100,
      after_balance: 250,
      amount: 150,
      reason: '期初修正',
      created_by_name: '财务',
      created_at: '2026-10-22 10:00:00',
    },
  ]);
  Api.deleteAccount.mockReset().mockResolvedValue(undefined);
});

afterEach(() => {
  clearSession();
});

describe('资金账户调账', () => {
  it('调账填写余额和原因后调用独立调账接口', async () => {
    render(<AccountsPage route={{}} />);
    await screen.findByText('现金账户');

    fireEvent.click(screen.getByRole('button', { name: '调账' }));
    expect(screen.getByText('账户余额调账')).toBeInTheDocument();

    fireEvent.input(screen.getByPlaceholderText('请输入调整后余额'), {
      target: { value: '250' },
    });
    fireEvent.input(screen.getByPlaceholderText(/例如：银行手续费/), {
      target: { value: '期初修正' },
    });
    fireEvent.click(screen.getByRole('button', { name: '确认调账' }));

    await waitFor(() => expect(Api.adjustAccount).toHaveBeenCalledWith(1, 250, '期初修正'));
  });

  it('调账记录可查看调整前后金额、差额、原因和操作人', async () => {
    render(<AccountsPage route={{}} />);
    await screen.findByText('现金账户');

    fireEvent.click(screen.getByRole('button', { name: '记录' }));
    await waitFor(() => expect(Api.accountAdjustments).toHaveBeenCalledWith(1));
    expect(await screen.findByText('期初修正')).toBeInTheDocument();
    expect(screen.getByText('250.00')).toBeInTheDocument();
    expect(screen.getByText('150.00')).toBeInTheDocument();
    expect(screen.getByText('财务')).toBeInTheDocument();
  });

  it('编辑账户时不提交余额，余额提示通过调账修改', async () => {
    render(<AccountsPage route={{}} />);
    await screen.findByText('现金账户');

    fireEvent.click(screen.getByRole('button', { name: '编辑' }));
    expect(screen.getByText('余额请通过列表中的「调账」修改')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(Api.saveAccount).toHaveBeenCalled());
    const [payload, id] = Api.saveAccount.mock.calls[0];
    expect(id).toBe(1);
    expect(payload).not.toHaveProperty('balance');
  });
});
