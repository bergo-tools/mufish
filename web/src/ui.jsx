// 基础 UI 组件库（Preact）。
import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks';
import { fmtMoney, fmtQty } from './format.js';

// ---------------------------------------------------------------------------
// 图标
// ---------------------------------------------------------------------------

const ICON_PATHS = {
  dashboard: 'M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z',
  box: 'M12 2 3 7v10l9 5 9-5V7l-9-5zm0 2.3 6.5 3.6L12 11.5 5.5 7.9 12 4.3zM5 9.6l6 3.3v6.5l-6-3.3V9.6zm8 9.8v-6.5l6-3.3v6.5l-6 3.3z',
  tag: 'M21.4 11.6 12.4 2.6A2 2 0 0 0 11 2H4a2 2 0 0 0-2 2v7c0 .5.2 1 .6 1.4l9 9a2 2 0 0 0 2.8 0l7-7a2 2 0 0 0 0-2.8zM6.5 8A1.5 1.5 0 1 1 8 6.5 1.5 1.5 0 0 1 6.5 8z',
  users: 'M16 11a4 4 0 1 0-4-4 4 4 0 0 0 4 4zm-8 1a3 3 0 1 0-3-3 3 3 0 0 0 3 3zm0 2c-2.7 0-6 1.3-6 3.5V20h8v-2.5c0-.9.4-1.8 1-2.5A11 11 0 0 0 8 14zm8 0c-3 0-8 1.5-8 4.5V20h16v-1.5c0-3-5-4.5-8-4.5z',
  warehouse: 'M12 2 2 7v2h20V7L12 2zM4 11v9H2v2h20v-2h-2v-9h-2v9h-3v-9h-2v9H9v-9H7v9H4v-9z',
  cart: 'M7 18a2 2 0 1 0 2 2 2 2 0 0 0-2-2zm10 0a2 2 0 1 0 2 2 2 2 0 0 0-2-2zM7.2 14h9.5c.8 0 1.4-.4 1.7-1.1l3.5-8.4A1 1 0 0 0 21 3H6.2l-.3-1.4A1 1 0 0 0 5 1H2v2h2l2.6 13.6A2 2 0 0 0 8.5 18h12v-2H8.5l-.3-2z',
  sell: 'M3 3h18v4H3V3zm1 5h16v13H4V8zm3 3v2h10v-2H7zm0 4v2h7v-2H7z',
  ledger: 'M4 2h13l3 3v17H4V2zm2 2v16h12V6h-2V4H6zm2 5h8v2H8V9zm0 4h8v2H8v-2zm0 4h5v2H8v-2z',
  alert: 'M12 2 1 21h22L12 2zm0 5 7 12H5l7-12zm-1 4v4h2v-4h-2zm0 5v2h2v-2h-2z',
  clipboard: 'M9 2h6v2h3v18H6V4h3V2zm0 4H8v14h8V6h-1v2H9V6zm2 4h4v2h-4v-2zm0 4h4v2h-4v-2z',
  transfer: 'M7 3 2 8h3v9h2V8h3L7 3zm10 18 5-5h-3V7h-2v9h-3l3 5z',
  money: 'M12 2a10 10 0 1 0 10 10A10 10 0 0 0 12 2zm1 15.9V19h-2v-1.1a4 4 0 0 1-3.4-2.4l1.9-.9A2.2 2.2 0 0 0 12 16c.9 0 1.5-.4 1.5-1s-.5-.9-2-1.3-3.2-1-3.2-3c0-1.5 1.1-2.6 2.7-2.9V6.6h2v1.2a3.7 3.7 0 0 1 3 2.2l-1.8 1a1.9 1.9 0 0 0-1.7-1.1c-.9 0-1.4.4-1.4.9s.4.8 2 1.2 3.3 1.1 3.3 3.1c0 1.6-1.2 2.7-3.4 2.9z',
  wallet: 'M21 7V5a2 2 0 0 0-2-2H5a3 3 0 0 0-3 3v12a2 2 0 0 0 2 2h15a2 2 0 0 0 2-2v-2h-6a3 3 0 0 1 0-6h6zm-6 4h7v2h-7a1 1 0 0 1 0-2z',
  chart: 'M4 20h16v2H4v-2zm2-9h3v8H6v-8zm5-6h3v14h-3V5zm5 9h3v5h-3v-5z',
  gear: 'M12 8a4 4 0 1 0 4 4 4 4 0 0 0-4-4zm9.4 4a7.6 7.6 0 0 0-.1-1.2l2-1.6-2-3.4-2.4 1a7.5 7.5 0 0 0-2-1.2L16.5 3h-4l-.4 2.6a7.5 7.5 0 0 0-2 1.2l-2.4-1-2 3.4 2 1.6a7.6 7.6 0 0 0 0 2.4l-2 1.6 2 3.4 2.4-1a7.5 7.5 0 0 0 2 1.2l.4 2.6h4l.4-2.6a7.5 7.5 0 0 0 2-1.2l2.4 1 2-3.4-2-1.6c.1-.4.1-.8.1-1.2z',
  shield: 'M12 1 3 5v6c0 5.5 3.8 10.7 9 12 5.2-1.3 9-6.5 9-12V5l-9-4zm0 2.2 7 3.1V11c0 4.5-3 8.8-7 10-4-1.2-7-5.5-7-10V6.3l7-3.1z',
  log: 'M5 3h14v18H5V3zm2 3v2h10V6H7zm0 4v2h10v-2H7zm0 4v2h7v-2H7z',
  plus: 'M11 5h2v6h6v2h-6v6h-2v-6H5v-2h6V5z',
  search: 'M15.5 14h-.8l-.3-.3a6.5 6.5 0 1 0-.7.7l.3.3v.8l5 5 1.5-1.5-5-5zm-6 0A4.5 4.5 0 1 1 14 9.5 4.5 4.5 0 0 1 9.5 14z',
  edit: 'M3 17.2V21h3.8L18 9.8 14.2 6 3 17.2zM20.7 7.1a1 1 0 0 0 0-1.4l-2.4-2.4a1 1 0 0 0-1.4 0l-1.9 1.9L18.8 9l1.9-1.9z',
  trash: 'M6 19a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z',
  check: 'M9 16.2 4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z',
  close: 'M19 6.4 17.6 5 12 10.6 6.4 5 5 6.4 10.6 12 5 17.6 6.4 19 12 13.4 17.6 19 19 17.6 13.4 12 19 6.4z',
  ban: 'M12 2a10 10 0 1 0 10 10A10 10 0 0 0 12 2zM4 12a8 8 0 0 1 12.9-6.3L5.7 16.9A8 8 0 0 1 4 12zm8 8a8 8 0 0 1-4.9-1.7L18.3 7.1A8 8 0 0 1 12 20z',
  download: 'M12 16 6 10h4V3h4v7h4l-6 6zm-7 3h14v2H5v-2z',
  upload: 'M12 3l6 6h-4v7h-4V9H6l6-6zM5 19h14v2H5v-2z',
  refresh: 'M12 5V2L7 6l5 4V7a5 5 0 1 1-5 5H5a7 7 0 1 0 7-7z',
  menu: 'M3 6h18v2H3V6zm0 5h18v2H3v-2zm0 5h18v2H3v-2z',
  logout: 'M10 17l1.4-1.4-2.6-2.6H17v-2H8.8l2.6-2.6L10 7l-5 5 5 5zM19 3H5a2 2 0 0 0-2 2v4h2V5h14v14H5v-4H3v4a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V5a2 2 0 0 0-2-2z',
  print: 'M19 8h-1V3H6v5H5a3 3 0 0 0-3 3v6h4v4h12v-4h4v-6a3 3 0 0 0-3-3zM8 5h8v3H8V5zm8 14H8v-5h8v5zm3-6a1 1 0 1 1 1-1 1 1 0 0 1-1 1z',
  eye: 'M12 5C6.5 5 2 12 2 12s4.5 7 10 7 10-7 10-7-4.5-7-10-7zm0 11a4 4 0 1 1 4-4 4 4 0 0 1-4 4zm0-6a2 2 0 1 0 2 2 2 2 0 0 0-2-2z',
  chevronLeft: 'M15.4 7.4 14 6l-6 6 6 6 1.4-1.4L10.8 12l4.6-4.6z',
  chevronRight: 'M8.6 16.6 10 18l6-6-6-6-1.4 1.4L13.2 12l-4.6 4.6z',
  save: 'M17 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V7l-4-4zm-5 16a3 3 0 1 1 3-3 3 3 0 0 1-3 3zm3-11H6V5h9v3z',
  lock: 'M17 9V7a5 5 0 0 0-10 0v2a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8a2 2 0 0 0-2-2zm-8-2a3 3 0 0 1 6 0v2H9V7zm4 9.7V18h-2v-1.3a1.7 1.7 0 1 1 2 0z',
  database: 'M12 2C7 2 3 3.8 3 6v12c0 2.2 4 4 9 4s9-1.8 9-4V6c0-2.2-4-4-9-4zm0 2c4.4 0 7 1.4 7 2s-2.6 2-7 2-7-1.4-7-2 2.6-2 7-2zM5 9.6C6.6 10.5 9.1 11 12 11s5.4-.5 7-1.4V12c0 .6-2.6 2-7 2s-7-1.4-7-2V9.6zm0 5C6.6 15.5 9.1 16 12 16s5.4-.5 7-1.4V18c0 .6-2.6 2-7 2s-7-1.4-7-2v-3.4z',
  filter: 'M3 5h18v2l-7 7v6l-4 2v-8L3 7V5z',
};

export function Icon({ name, size = 16, className = '' }) {
  const d = ICON_PATHS[name] || ICON_PATHS.box;
  return (
    <svg
      class={'icon ' + className}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden="true"
    >
      <path d={d} />
    </svg>
  );
}

// ---------------------------------------------------------------------------
// 按钮
// ---------------------------------------------------------------------------

export function Button({
  children,
  variant = 'default',
  size = 'md',
  icon,
  onClick,
  disabled,
  type = 'button',
  className = '',
  title,
  loading,
}) {
  return (
    <button
      type={type}
      title={title}
      class={`btn btn-${variant} btn-${size} ${className}`}
      disabled={disabled || loading}
      onClick={onClick}
    >
      {loading ? <span class="spinner spinner-sm" /> : icon ? <Icon name={icon} size={size === 'sm' ? 14 : 16} /> : null}
      {children != null && children !== '' ? <span>{children}</span> : null}
    </button>
  );
}

// ---------------------------------------------------------------------------
// 表单控件
// ---------------------------------------------------------------------------

export function Field({ label, required, children, hint, className = '', inline }) {
  return (
    <label class={`field ${inline ? 'field-inline' : ''} ${className}`}>
      {label ? (
        <span class="field-label">
          {label}
          {required ? <em>*</em> : null}
        </span>
      ) : null}
      <span class="field-control">{children}</span>
      {hint ? <span class="field-hint">{hint}</span> : null}
    </label>
  );
}

export function Input({ value, onInput, type = 'text', placeholder, disabled, className = '', ...rest }) {
  return (
    <input
      class={'input ' + className}
      type={type}
      value={value === undefined || value === null ? '' : value}
      placeholder={placeholder}
      disabled={disabled}
      onInput={(e) => onInput && onInput(e.target.value, e)}
      {...rest}
    />
  );
}

export function NumberInput({ value, onInput, step = 'any', min, placeholder, disabled, className = '' }) {
  return (
    <input
      class={'input ' + className}
      type="number"
      step={step}
      min={min}
      value={value === undefined || value === null ? '' : value}
      placeholder={placeholder}
      disabled={disabled}
      onInput={(e) => onInput && onInput(e.target.value)}
    />
  );
}

export function Textarea({ value, onInput, rows = 3, placeholder, className = '' }) {
  return (
    <textarea
      class={'input textarea ' + className}
      rows={rows}
      placeholder={placeholder}
      value={value === undefined || value === null ? '' : value}
      onInput={(e) => onInput && onInput(e.target.value)}
    />
  );
}

export function Select({ value, onInput, options, placeholder, disabled, className = '', allowEmpty = true }) {
  return (
    <select
      class={'input select ' + className}
      value={value === undefined || value === null ? '' : value}
      disabled={disabled}
      onChange={(e) => onInput && onInput(e.target.value, e)}
    >
      {placeholder !== null ? <option value="">{placeholder || '请选择'}</option> : null}
      {(options || []).map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  );
}

export function Checkbox({ checked, onInput, label, disabled }) {
  return (
    <label class="checkbox">
      <input
        type="checkbox"
        checked={!!checked}
        disabled={disabled}
        onChange={(e) => onInput && onInput(e.target.checked)}
      />
      {label ? <span>{label}</span> : null}
    </label>
  );
}

export function Switch({ checked, onInput, label }) {
  return (
    <label class="switch">
      <input type="checkbox" checked={!!checked} onChange={(e) => onInput && onInput(e.target.checked)} />
      <span class="switch-track">
        <span class="switch-thumb" />
      </span>
      {label ? <span class="switch-label">{label}</span> : null}
    </label>
  );
}

// ---------------------------------------------------------------------------
// 标签 / 卡片
// ---------------------------------------------------------------------------

export function Tag({ children, color = 'gray', className = '' }) {
  return <span class={`tag tag-${color} ${className}`}>{children}</span>;
}

export function Card({ title, extra, children, className = '', bodyClass = '', icon }) {
  return (
    <section class={'card ' + className}>
      {title || extra ? (
        <header class="card-head">
          <h3 class="card-title">
            {icon ? <Icon name={icon} size={16} /> : null}
            {title}
          </h3>
          <div class="card-extra">{extra}</div>
        </header>
      ) : null}
      <div class={'card-body ' + bodyClass}>{children}</div>
    </section>
  );
}

export function StatCard({ label, value, sub, icon, color = 'blue' }) {
  return (
    <div class={'stat stat-' + color}>
      <div class="stat-icon">{icon ? <Icon name={icon} size={20} /> : null}</div>
      <div class="stat-main">
        <div class="stat-label">{label}</div>
        <div class="stat-value">{value}</div>
        {sub ? <div class="stat-sub">{sub}</div> : null}
      </div>
    </div>
  );
}

export function Empty({ text = '暂无数据', icon = 'box' }) {
  return (
    <div class="empty">
      <Icon name={icon} size={34} />
      <p>{text}</p>
    </div>
  );
}

export function Spinner({ text }) {
  return (
    <div class="loading-box">
      <span class="spinner" />
      {text ? <span>{text}</span> : null}
    </div>
  );
}

// ---------------------------------------------------------------------------
// 表格
// ---------------------------------------------------------------------------

export function Table({
  columns,
  data,
  loading,
  rowKey = 'id',
  onRowClick,
  empty = '暂无数据',
  footer,
  compact,
  maxHeight,
}) {
  const cols = columns || [];
  return (
    <div class="table-wrap" style={maxHeight ? { maxHeight, overflow: 'auto' } : undefined}>
      <table class={'table ' + (compact ? 'table-compact' : '')}>
        <thead>
          <tr>
            {cols.map((c, i) => (
              <th
                key={c.key || i}
                style={{ width: c.width, textAlign: c.align || 'left' }}
                class={c.className || ''}
              >
                {c.title}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {loading ? (
            <tr>
              <td colSpan={cols.length}>
                <Spinner text="加载中…" />
              </td>
            </tr>
          ) : !data || data.length === 0 ? (
            <tr>
              <td colSpan={cols.length}>
                <Empty text={empty} />
              </td>
            </tr>
          ) : (
            data.map((row, ri) => (
              <tr
                key={row[rowKey] !== undefined ? row[rowKey] : ri}
                class={onRowClick ? 'row-clickable' : ''}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
              >
                {cols.map((c, ci) => (
                  <td key={c.key || ci} style={{ textAlign: c.align || 'left' }} class={c.className || ''}>
                    {c.render ? c.render(row, ri) : row[c.key]}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
        {footer ? <tfoot>{footer}</tfoot> : null}
      </table>
    </div>
  );
}

export function Pagination({ page, size, total, onPage, onSize }) {
  const pages = Math.max(1, Math.ceil((total || 0) / (size || 20)));
  const nums = useMemo(() => {
    const out = [];
    const start = Math.max(1, Math.min(page - 2, pages - 4));
    const end = Math.min(pages, start + 4);
    for (let i = start; i <= end; i++) out.push(i);
    return out;
  }, [page, pages]);

  return (
    <div class="pagination">
      <span class="page-total">共 {total || 0} 条</span>
      <div class="page-btns">
        <button class="page-btn" disabled={page <= 1} onClick={() => onPage(page - 1)}>
          <Icon name="chevronLeft" size={14} />
        </button>
        {nums.map((n) => (
          <button key={n} class={'page-btn ' + (n === page ? 'active' : '')} onClick={() => onPage(n)}>
            {n}
          </button>
        ))}
        <button class="page-btn" disabled={page >= pages} onClick={() => onPage(page + 1)}>
          <Icon name="chevronRight" size={14} />
        </button>
      </div>
      {onSize ? (
        <select class="input select page-size" value={size} onChange={(e) => onSize(Number(e.target.value))}>
          {[10, 20, 50, 100].map((n) => (
            <option key={n} value={n}>
              {n} 条/页
            </option>
          ))}
        </select>
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------------------
// 工具栏 / 搜索
// ---------------------------------------------------------------------------

export function Toolbar({ children, right }) {
  return (
    <div class="toolbar">
      <div class="toolbar-left">{children}</div>
      <div class="toolbar-right">{right}</div>
    </div>
  );
}

export function SearchBox({ value, onInput, onSearch, placeholder = '输入关键字搜索', width = 220 }) {
  return (
    <div class="search-box" style={{ width }}>
      <Icon name="search" size={14} />
      <input
        class="search-input"
        value={value || ''}
        placeholder={placeholder}
        onInput={(e) => onInput && onInput(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && onSearch) onSearch();
        }}
      />
      {value ? (
        <button class="search-clear" onClick={() => onInput && onInput('')} title="清空">
          <Icon name="close" size={12} />
        </button>
      ) : null}
    </div>
  );
}

export function Tabs({ tabs, active, onChange }) {
  return (
    <div class="tabs">
      {tabs.map((t) => (
        <button
          key={t.key}
          class={'tab ' + (t.key === active ? 'active' : '')}
          onClick={() => onChange(t.key)}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

export function DateRange({ from, to, onFrom, onTo, children }) {
  return (
    <div class="date-range">
      <input class="input date-input" type="date" value={from || ''} onChange={(e) => onFrom(e.target.value)} />
      <span class="date-sep">至</span>
      <input class="input date-input" type="date" value={to || ''} onChange={(e) => onTo(e.target.value)} />
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// 弹窗
// ---------------------------------------------------------------------------

export function Modal({ open, title, onClose, children, footer, width = 640, bodyClass = '' }) {
  useEffect(() => {
    if (!open) return undefined;
    const onKey = (e) => {
      if (e.key === 'Escape' && onClose) onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div class="modal-mask" onClick={(e) => e.target === e.currentTarget && onClose && onClose()}>
      <div class="modal" style={{ width }}>
        <header class="modal-head">
          <h3>{title}</h3>
          <button class="icon-btn" onClick={onClose} title="关闭">
            <Icon name="close" size={16} />
          </button>
        </header>
        <div class={'modal-body ' + bodyClass}>{children}</div>
        {footer ? <footer class="modal-foot">{footer}</footer> : null}
      </div>
    </div>
  );
}

// 全局确认框
const confirmState = { current: null, listeners: new Set() };

function emitConfirm() {
  confirmState.listeners.forEach((fn) => fn(confirmState.current));
}

export function confirmDialog(message, { title = '操作确认', okText = '确定', danger = true } = {}) {
  return new Promise((resolve) => {
    confirmState.current = { message, title, okText, danger, resolve };
    emitConfirm();
  });
}

export function ConfirmHost() {
  const [state, setState] = useState(confirmState.current);
  useEffect(() => {
    confirmState.listeners.add(setState);
    return () => confirmState.listeners.delete(setState);
  }, []);

  const close = (v) => {
    const cur = confirmState.current;
    confirmState.current = null;
    emitConfirm();
    if (cur && cur.resolve) cur.resolve(v);
  };

  return (
    <Modal
      open={!!state}
      title={state ? state.title : ''}
      onClose={() => close(false)}
      width={420}
      footer={
        <>
          <Button onClick={() => close(false)}>取消</Button>
          <Button variant={state && state.danger ? 'danger' : 'primary'} onClick={() => close(true)}>
            {state ? state.okText : '确定'}
          </Button>
        </>
      }
    >
      <p class="confirm-text">{state ? state.message : ''}</p>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// 消息条
// ---------------------------------------------------------------------------

export function ToastHost({ store }) {
  const state = useStoreSafe(store);
  return (
    <div class="toast-host">
      {state.items.map((t) => (
        <div key={t.id} class={'toast toast-' + t.type}>
          <Icon name={t.type === 'error' ? 'alert' : t.type === 'success' ? 'check' : 'eye'} size={14} />
          <span>{t.message}</span>
        </div>
      ))}
    </div>
  );
}

function useStoreSafe(store) {
  const [state, setState] = useState(store.get());
  useEffect(() => store.subscribe(setState), [store]);
  return state;
}

// ---------------------------------------------------------------------------
// 数据钩子
// ---------------------------------------------------------------------------

// 分页列表：fetcher 接收 { page, size, ...filters } 并返回 { list, total }。
export function usePagedList(fetcher, initialFilters = {}, initialSize = 20) {
  const [list, setList] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(initialSize);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [filters, setFiltersState] = useState(initialFilters);
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;
  const filterKey = JSON.stringify(filters);

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetcherRef.current({ page, size, ...JSON.parse(filterKey) });
      setList((res && res.list) || []);
      setTotal((res && res.total) || 0);
    } catch (e) {
      setError(e);
      setList([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [page, size, filterKey]);

  useEffect(() => {
    reload();
  }, [reload]);

  const setFilters = useCallback((patch) => {
    setPage(1);
    setFiltersState((prev) => ({ ...prev, ...(typeof patch === 'function' ? patch(prev) : patch) }));
  }, []);

  return { list, total, page, size, loading, error, filters, setFilters, setPage, setSize, reload };
}

// 简单异步数据加载。
export function useAsync(loader, deps = []) {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [tick, setTick] = useState(0);
  const loaderRef = useRef(loader);
  loaderRef.current = loader;

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError(null);
    loaderRef
      .current()
      .then((d) => alive && setData(d))
      .catch((e) => alive && setError(e))
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick]);

  return { data, loading, error, reload: () => setTick((t) => t + 1), setData };
}

// ---------------------------------------------------------------------------
// 其它小组件
// ---------------------------------------------------------------------------

export function Money({ value, colored, bold, symbol = false }) {
  const n = Number(value || 0);
  const cls = 'money' + (colored ? (n > 0 ? ' money-pos' : n < 0 ? ' money-neg' : '') : '') + (bold ? ' money-bold' : '');
  return <span class={cls}>{fmtMoney(n, symbol)}</span>;
}

export function Qty({ value, unit }) {
  return (
    <span class="qty">
      {fmtQty(value)}
      {unit ? <em>{unit}</em> : null}
    </span>
  );
}

export function Divider() {
  return <div class="divider" />;
}

export function Descriptions({ items, columns = 3 }) {
  return (
    <div class="descriptions" style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}>
      {items.map((it, i) => (
        <div class="desc-item" key={i}>
          <span class="desc-label">{it.label}</span>
          <span class="desc-value">{it.value}</span>
        </div>
      ))}
    </div>
  );
}
