import { h } from 'preact';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/preact';
import { useAsync } from '../../src/ui.jsx';

function Probe({ loader, dep }) {
  const { data, loading } = useAsync(loader, [dep]);
  return (
    <div>
      <span data-testid="data">{data ? data.id : 'none'}</span>
      <span data-testid="loading">{String(loading)}</span>
    </div>
  );
}

describe('useAsync', () => {
  it('依赖变化时先清空旧数据，避免用新表头展示旧结果', async () => {
    const first = vi.fn(() => Promise.resolve({ id: 'old' }));
    const pending = new Promise(() => {});
    const second = vi.fn(() => pending);

    const { rerender } = render(<Probe loader={first} dep={1} />);
    await waitFor(() => expect(screen.getByTestId('data').textContent).toBe('old'));

    rerender(<Probe loader={second} dep={2} />);
    expect(screen.getByTestId('data').textContent).toBe('none');
    expect(screen.getByTestId('loading').textContent).toBe('true');
  });
});
