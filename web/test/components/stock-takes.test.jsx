import { h } from 'preact';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/preact';

vi.mock('../../src/api.js', () => ({
  Api: {
    stockTakes: vi.fn(),
    warehouseOptions: vi.fn(),
    stocks: vi.fn(),
    productOptions: vi.fn(),
    stockTake: vi.fn(),
    saveStockTake: vi.fn(),
    finishStockTake: vi.fn(),
    voidStockTake: vi.fn(),
    deleteStockTake: vi.fn(),
  },
}));

import { Api } from '../../src/api.js';
import StockTakesPage from '../../src/pages/stock-takes.jsx';
import { clearSession, setSession } from '../../src/store.js';

const stockA = {
  product_id: 1,
  product_code: 'P001',
  product_name: '仓库一商品',
  spec: 'A',
  unit: '个',
  qty: 3,
  cost_price: 10,
};
const stockB = {
  product_id: 2,
  product_code: 'P002',
  product_name: '仓库二商品',
  spec: 'B',
  unit: '个',
  qty: 8,
  cost_price: 20,
};

function pageOf(list) {
  return { list, total: list.length, page: 1, size: 500 };
}

function deferredByWarehouse() {
  const pending = new Map();
  Api.stocks.mockImplementation(
    (params) =>
      new Promise((resolve) => {
        pending.set(String(params.warehouse_id), resolve);
      }),
  );
  return pending;
}

beforeEach(() => {
  setSession({ id: 1, username: 'stock', status: 1 }, ['stock:take']);
  Api.stockTakes.mockReset().mockResolvedValue({ list: [], total: 0, page: 1, size: 20 });
  Api.warehouseOptions.mockReset().mockResolvedValue([
    { id: 1, name: '甲仓库' },
    { id: 2, name: '乙仓库' },
  ]);
  Api.stocks.mockReset();
  Api.productOptions.mockReset().mockResolvedValue([]);
});

afterEach(() => {
  clearSession();
});

async function openCreateModal() {
  render(<StockTakesPage route={{}} />);
  fireEvent.click(await screen.findByRole('button', { name: '新建盘点' }));
  return screen.findByLabelText(/盘点仓库/);
}

describe('盘点明细加载', () => {
  it('切换仓库后忽略旧仓库的迟到响应，并在加载中禁用保存', async () => {
    const pending = deferredByWarehouse();
    const select = await openCreateModal();

    fireEvent.change(select, { target: { value: '1' } });
    await waitFor(() => expect(pending.has('1')).toBe(true));

    fireEvent.change(select, { target: { value: '2' } });
    await waitFor(() => expect(pending.has('2')).toBe(true));
    expect(screen.getByRole('button', { name: '保存' })).toBeDisabled();

    pending.get('2')(pageOf([stockB]));
    expect(await screen.findByText('仓库二商品')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', { name: '保存' })).not.toBeDisabled());

    pending.get('1')(pageOf([stockA]));
    await new Promise((resolve) => setTimeout(resolve, 30));
    expect(screen.queryByText('仓库一商品')).not.toBeInTheDocument();
    expect(screen.getByText('仓库二商品')).toBeInTheDocument();
  });

  it('切换仓库加载失败时清空旧仓库明细', async () => {
    Api.stocks.mockImplementation((params) => {
      if (String(params.warehouse_id) === '1') return Promise.resolve(pageOf([stockA]));
      return Promise.reject(new Error('库存加载失败'));
    });
    const select = await openCreateModal();

    fireEvent.change(select, { target: { value: '1' } });
    expect(await screen.findByText('仓库一商品')).toBeInTheDocument();

    fireEvent.change(select, { target: { value: '2' } });
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(screen.queryByText('仓库一商品')).not.toBeInTheDocument();
    expect(screen.getByText('该仓库暂无可盘点的库存商品')).toBeInTheDocument();
  });

  it('刷新同一仓库时保留手工添加的零库存商品和实盘数量', async () => {
    Api.stocks.mockResolvedValue(pageOf([stockA]));
    Api.productOptions.mockResolvedValue([
      {
        id: 2,
        code: 'P002',
        name: '盘盈商品',
        spec: '',
        unit: '个',
        purchase_price: 12.5,
      },
    ]);
    const select = await openCreateModal();

    fireEvent.change(select, { target: { value: '1' } });
    expect(await screen.findByText('仓库一商品')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '搜索商品' }));
    const option = await screen.findByRole('option', { name: /P002 盘盈商品/ });
    const productSelect = option.closest('select');
    fireEvent.change(productSelect, { target: { value: '2' } });
    fireEvent.click(screen.getByRole('button', { name: '添加商品' }));
    expect(await screen.findByText('盘盈商品')).toBeInTheDocument();

    const row = screen.getByText('盘盈商品').closest('tr');
    const realInput = row.querySelector('input');
    fireEvent.input(realInput, { target: { value: '7' } });
    expect(realInput).toHaveValue(7);

    fireEvent.click(screen.getByRole('button', { name: '刷新账面库存' }));
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(screen.getByText('盘盈商品')).toBeInTheDocument();
    expect(screen.getByText('盘盈商品').closest('tr').querySelector('input')).toHaveValue(7);
  });
});
