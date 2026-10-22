import { h } from 'preact';
import { describe, it, expect, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/preact';
import { usePagedList } from '../../src/ui.jsx';

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function ListProbe({ fetcher }) {
  const { list, total, page, loading, setPage, setSize } = usePagedList(fetcher, {}, 2);
  return (
    <div>
      <span data-testid="page">{page}</span>
      <span data-testid="total">{total}</span>
      <span data-testid="loading">{String(loading)}</span>
      <span data-testid="list">{list.map((x) => x.id).join(',')}</span>
      <button onClick={() => setPage(3)}>go3</button>
      <button onClick={() => setSize(10)}>size10</button>
    </div>
  );
}

describe('usePagedList', () => {
  it('较早的请求后返回时不会覆盖最新结果', async () => {
    const first = deferred();
    const second = deferred();
    const fetcher = vi.fn()
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);

    render(<ListProbe fetcher={fetcher} />);
    expect(fetcher).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByText('go3'));
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));

    await act(async () => {
      second.resolve({ list: [{ id: 'new' }], total: 5 });
      await second.promise;
    });
    expect(screen.getByTestId('list').textContent).toBe('new');
    expect(screen.getByTestId('loading').textContent).toBe('false');

    // 先发出的请求此时才返回，必须被忽略。
    await act(async () => {
      first.resolve({ list: [{ id: 'old' }], total: 5 });
      await first.promise;
    });
    expect(screen.getByTestId('list').textContent).toBe('new');
  });

  it('页码超过最大页时自动回到最后一页', async () => {
    const fetcher = vi.fn(async ({ page }) => ({
      list: page === 1 ? [{ id: 'one' }] : [],
      total: 1,
    }));
    render(<ListProbe fetcher={fetcher} />);
    await waitFor(() => expect(screen.getByTestId('list').textContent).toBe('one'));

    fireEvent.click(screen.getByText('go3'));
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(3));
    await waitFor(() => expect(screen.getByTestId('page').textContent).toBe('1'));
    expect(screen.getByTestId('list').textContent).toBe('one');
  });
});
