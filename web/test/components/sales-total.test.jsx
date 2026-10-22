import { h } from 'preact';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/preact';

vi.mock('../../src/api.js', () => ({
  Api: {
    sales: vi.fn(),
    sale: vi.fn(),
    saveSales: vi.fn(),
    confirmSales: vi.fn(),
    voidSales: vi.fn(),
    deleteSales: vi.fn(),
    partnerOptions: vi.fn(),
    warehouseOptions: vi.fn(),
    productOptions: vi.fn(),
  },
}));

import { Api } from '../../src/api.js';
import SalesPage from '../../src/pages/sales.jsx';
import { clearSession, setSession } from '../../src/store.js';

beforeEach(() => {
  setSession(
    { id: 1, username: 'admin', status: 1 },
    ['sales:view', 'sales:create'],
  );
  Api.sales.mockReset().mockResolvedValue({ list: [], total: 0, page: 1, size: 20 });
  Api.sale.mockReset();
  Api.saveSales.mockReset();
  Api.confirmSales.mockReset();
  Api.voidSales.mockReset();
  Api.deleteSales.mockReset();
  Api.partnerOptions.mockReset().mockResolvedValue([]);
  Api.warehouseOptions.mockReset().mockResolvedValue([]);
  Api.productOptions.mockReset().mockResolvedValue([]);
});

afterEach(() => {
  clearSession();
});

describe('销售单金额合计', () => {
  it('按每行分金额求和，不因浮点误差少计 1 分', async () => {
    render(<SalesPage route={{}} />);

    fireEvent.click(await screen.findByRole('button', { name: '新建销售单' }));
    fireEvent.click(screen.getByRole('button', { name: '添加一行' }));

    const inputs = screen.getAllByRole('spinbutton');
    fireEvent.input(inputs[0], { target: { value: '0.7' } });
    fireEvent.input(inputs[1], { target: { value: '0.05' } });
    fireEvent.input(inputs[2], { target: { value: '0.7' } });
    fireEvent.input(inputs[3], { target: { value: '0.05' } });

    await waitFor(() => {
      expect(screen.getByText('合计金额').closest('.doc-total-item')).toHaveTextContent('0.08');
    });
    expect(screen.getByText('结算金额').closest('.doc-total-item')).toHaveTextContent('0.08');
  });
});
