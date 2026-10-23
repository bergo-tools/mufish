import { h } from 'preact';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/preact';

vi.mock('../../src/api.js', () => ({
  Api: {
    stocks: vi.fn(),
    stockSummary: vi.fn(),
    warehouseOptions: vi.fn(),
    categoriesFlat: vi.fn(),
  },
}));

import { Api } from '../../src/api.js';
import StocksPage from '../../src/pages/stocks.jsx';
import { clearSession, setSession } from '../../src/store.js';

beforeEach(() => {
  setSession({ id: 1, username: 'stock', status: 1 }, ['stock:view']);
  Api.stocks.mockReset().mockResolvedValue({ list: [], total: 0, page: 1, size: 20 });
  Api.stockSummary.mockReset().mockResolvedValue({ kinds: 0, qty: 0, amount: 0 });
  Api.warehouseOptions.mockReset().mockResolvedValue([{ id: 1, name: '甲仓库' }]);
  Api.categoriesFlat.mockReset().mockResolvedValue([]);
});

afterEach(() => {
  clearSession();
});

describe('库存查询统计口径', () => {
  it('筛选仓库后统计文案不再显示“全部仓库合计”', async () => {
    render(<StocksPage route={{}} />);
    expect(await screen.findByText('全部仓库合计')).toBeInTheDocument();

    const option = await screen.findByRole('option', { name: '甲仓库' });
    fireEvent.change(option.closest('select'), { target: { value: '1' } });

    await waitFor(() => expect(screen.getByText('当前筛选仓库')).toBeInTheDocument());
    expect(screen.queryByText('全部仓库合计')).not.toBeInTheDocument();
  });
});
