import { h } from 'preact';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/preact';

vi.mock('../../src/api.js', () => {
  const empty = () => Promise.resolve([]);
  return {
    Api: {
      warehouseOptions: vi.fn(empty),
      categoriesFlat: vi.fn(empty),
      partnerOptions: vi.fn(empty),
      accountOptions: vi.fn(empty),
      reportInout: vi.fn(empty),
      reportSalesRank: vi.fn(empty),
      reportPurchaseRank: vi.fn(empty),
      reportCustomerRank: vi.fn(empty),
      reportSupplierRank: vi.fn(empty),
      reportArAp: vi.fn(empty),
      reportSalesDetail: vi.fn(empty),
      reportPurchaseDetail: vi.fn(empty),
      reportCashFlow: vi.fn(empty),
      exportInout: vi.fn(),
      exportArAp: vi.fn(),
    },
  };
});

import { Api } from '../../src/api.js';
import ReportsPage from '../../src/pages/reports.jsx';
import { clearSession, setSession } from '../../src/store.js';

const empty = () => Promise.resolve([]);
const reportFns = [
  'warehouseOptions',
  'categoriesFlat',
  'partnerOptions',
  'accountOptions',
  'reportInout',
  'reportSalesRank',
  'reportPurchaseRank',
  'reportCustomerRank',
  'reportSupplierRank',
  'reportArAp',
  'reportSalesDetail',
  'reportPurchaseDetail',
  'reportCashFlow',
];

beforeEach(() => {
  setSession({ id: 1, username: 'admin', status: 1 }, ['report:view']);
  reportFns.forEach((key) => Api[key].mockReset().mockImplementation(empty));
  Api.exportInout.mockReset();
  Api.exportArAp.mockReset();
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

  it('关键字只在回车/查询时提交，输入过程不请求', async () => {
    render(<ReportsPage />);
    await waitFor(() => expect(Api.reportInout).toHaveBeenCalledTimes(1));

    const box = screen.getByPlaceholderText('商品 / 单位 / 单据号');
    fireEvent.input(box, { target: { value: '测试关键字' } });
    expect(Api.reportInout).toHaveBeenCalledTimes(1);

    fireEvent.keyDown(box, { key: 'Enter' });
    await waitFor(() => expect(Api.reportInout).toHaveBeenCalledTimes(2));
    const last = Api.reportInout.mock.calls[Api.reportInout.mock.calls.length - 1][0];
    expect(last.keyword).toBe('测试关键字');
  });

  it('报表加载失败时显示错误与重试，而不是“暂无数据”', async () => {
    Api.reportInout.mockRejectedValue(new Error('网络连接失败'));
    render(<ReportsPage />);
    await waitFor(() => expect(screen.getByText(/报表加载失败：网络连接失败/)).toBeInTheDocument());
    expect(screen.getByRole('button', { name: '重试' })).toBeInTheDocument();
    expect(screen.queryByText('暂无数据')).not.toBeInTheDocument();
  });
});
