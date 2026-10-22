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
import { ConfirmHost } from '../../src/ui.jsx';
import { clearSession, setSession } from '../../src/store.js';

function detail(status) {
  return {
    id: 1,
    order_no: 'CG20261022-0001',
    supplier_id: 1,
    supplier_name: '测试供应商',
    warehouse_id: 1,
    warehouse_name: '主仓库',
    order_date: '2026-10-22',
    status,
    total_qty: 1,
    total_amount: 10,
    discount_amount: 0,
    other_fee: 0,
    settle_amount: 10,
    paid_amount: 0,
    created_by_name: '管理员',
    created_at: '2026-10-22 10:00:00',
    confirmed_at: '',
    items: [],
  };
}

beforeEach(() => {
  setSession(
    { id: 1, username: 'admin', status: 1 },
    ['purchase:view', 'purchase:confirm', 'purchase:void'],
  );
  Api.purchases.mockReset().mockResolvedValue({ list: [], total: 0, page: 1, size: 20 });
  Api.purchase.mockReset();
  Api.savePurchase.mockReset();
  Api.confirmPurchase.mockReset().mockResolvedValue(undefined);
  Api.voidPurchase.mockReset().mockResolvedValue(undefined);
  Api.deletePurchase.mockReset();
  Api.partnerOptions.mockReset().mockResolvedValue([]);
  Api.warehouseOptions.mockReset().mockResolvedValue([]);
  Api.productOptions.mockReset().mockResolvedValue([]);
});

afterEach(() => {
  clearSession();
});

describe('采购单详情操作', () => {
  it('草稿详情可直接确认入库', async () => {
    Api.purchase.mockResolvedValue(detail(0));
    render(
      <>
        <PurchasesPage route={{ query: { id: '1' } }} />
        <ConfirmHost />
      </>,
    );

    await screen.findByText(/采购单详情/);
    fireEvent.click(screen.getByRole('button', { name: '确认入库' }));
    await screen.findByText(/确定要确认入库单据/);
    fireEvent.click(screen.getByRole('button', { name: '确定' }));
    await waitFor(() => expect(Api.confirmPurchase).toHaveBeenCalledWith(1));
  });

  it('已确认详情可直接作废', async () => {
    Api.purchase.mockResolvedValue(detail(1));
    render(
      <>
        <PurchasesPage route={{ query: { id: '1' } }} />
        <ConfirmHost />
      </>,
    );

    await screen.findByText(/采购单详情/);
    fireEvent.click(screen.getByRole('button', { name: '作废' }));
    await screen.findByText(/确定要作废单据/);
    fireEvent.click(screen.getByRole('button', { name: '确定' }));
    await waitFor(() => expect(Api.voidPurchase).toHaveBeenCalledWith(1));
  });
});
