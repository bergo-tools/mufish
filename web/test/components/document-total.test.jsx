import { h } from 'preact';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/preact';

vi.mock('../../src/api.js', () => ({
  Api: {
    purchases: vi.fn(),
    purchase: vi.fn(),
    savePurchase: vi.fn(),
    confirmPurchase: vi.fn(),
    voidPurchase: vi.fn(),
    deletePurchase: vi.fn(),
    partnerOptions: vi.fn(),
    warehouseOptions: vi.fn(),
    productOptions: vi.fn(),
  },
}));

import { Api } from '../../src/api.js';
import PurchasesPage from '../../src/pages/purchases.jsx';
import { clearSession, setSession } from '../../src/store.js';

beforeEach(() => {
  setSession(
    { id: 1, username: 'admin', status: 1 },
    ['purchase:view', 'purchase:create'],
  );
  Api.purchases.mockReset().mockResolvedValue({ list: [], total: 0, page: 1, size: 20 });
  Api.purchase.mockReset();
  Api.savePurchase.mockReset();
  Api.confirmPurchase.mockReset();
  Api.voidPurchase.mockReset();
  Api.deletePurchase.mockReset();
  Api.partnerOptions.mockReset().mockResolvedValue([]);
  Api.warehouseOptions.mockReset().mockResolvedValue([]);
  Api.productOptions.mockReset().mockResolvedValue([]);
});

afterEach(() => {
  clearSession();
});

describe('采购单金额合计', () => {
  it('按每行分金额求和，不因浮点误差少计 1 分', async () => {
    render(<PurchasesPage route={{}} />);

    fireEvent.click(await screen.findByRole('button', { name: '新建采购单' }));
    fireEvent.click(screen.getByRole('button', { name: '添加一行' }));

    const inputs = screen.getAllByRole('spinbutton');
    // 两行数量/单价，后面依次是优惠金额、其他费用。
    fireEvent.input(inputs[0], { target: { value: '0.7' } });
    fireEvent.input(inputs[1], { target: { value: '0.05' } });
    fireEvent.input(inputs[2], { target: { value: '0.7' } });
    fireEvent.input(inputs[3], { target: { value: '0.05' } });

    await waitFor(() => {
      const total = screen.getByText('合计金额').closest('.doc-total-item');
      expect(total).toHaveTextContent('0.08');
    });
    expect(screen.getByText('结算金额').closest('.doc-total-item')).toHaveTextContent('0.08');
  });
});
