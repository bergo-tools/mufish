import { h } from 'preact';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/preact';

vi.mock('../../src/api.js', () => {
  const empty = vi.fn().mockResolvedValue([]);
  return {
    Api: {
      warehouseOptions: empty,
      categoriesFlat: empty,
      partnerOptions: empty,
      accountOptions: empty,
      reportInout: empty,
      reportSalesRank: empty,
      reportPurchaseRank: empty,
      reportCustomerRank: empty,
      reportSupplierRank: empty,
      reportArAp: empty,
      reportSalesDetail: empty,
      reportPurchaseDetail: empty,
      reportCashFlow: empty,
      exportInout: vi.fn(),
      exportArAp: vi.fn(),
    },
  };
});

import ReportsPage from '../../src/pages/reports.jsx';
import { clearSession, setSession } from '../../src/store.js';

beforeEach(() => {
  setSession({ id: 1, username: 'admin', status: 1 }, ['report:view']);
});

afterEach(() => {
  clearSession();
});

function dateInputCount() {
  return document.querySelectorAll('input[type="date"]').length;
}

describe('报表筛选口径', () => {
  it('客户/供应商排行显示日期范围，应收应付不显示', async () => {
    render(<ReportsPage />);
    await waitFor(() => expect(screen.getByRole('button', { name: '客户排行' })).toBeInTheDocument());
    expect(dateInputCount()).toBe(2);

    fireEvent.click(screen.getByRole('button', { name: '客户排行' }));
    await waitFor(() => expect(screen.getByRole('button', { name: '客户排行' })).toHaveClass('active'));
    expect(dateInputCount()).toBe(2);

    fireEvent.click(screen.getByRole('button', { name: '供应商排行' }));
    await waitFor(() => expect(screen.getByRole('button', { name: '供应商排行' })).toHaveClass('active'));
    expect(dateInputCount()).toBe(2);

    fireEvent.click(screen.getByRole('button', { name: '应收应付' }));
    await waitFor(() => expect(screen.getByRole('button', { name: '应收应付' })).toHaveClass('active'));
    expect(dateInputCount()).toBe(0);
  });
});
