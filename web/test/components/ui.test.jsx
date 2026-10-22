import { h } from 'preact';
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, screen } from '@testing-library/preact';

import {
  Button,
  Input,
  NumberInput,
  Select,
  Checkbox,
  Switch,
  Tabs,
  Table,
  Pagination,
  Modal,
  Money,
  Qty,
  Empty,
  NavItem,
} from '../../src/ui.jsx';

describe('Button', () => {
  it('渲染文本、样式并触发点击', () => {
    const onClick = vi.fn();
    render(<Button variant="primary" icon="plus" onClick={onClick}>保存</Button>);
    const btn = screen.getByRole('button', { name: '保存' });
    expect(btn).toHaveClass('btn-primary');
    expect(btn.querySelector('svg')).not.toBeNull();
    fireEvent.click(btn);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('disabled/loading 时渲染为禁用状态', () => {
    const onClick = vi.fn();
    const { rerender } = render(<Button disabled onClick={onClick}>保存</Button>);
    const btn = screen.getByRole('button', { name: '保存' });
    expect(btn).toBeDisabled();

    rerender(<Button loading onClick={onClick}>保存</Button>);
    expect(screen.getByRole('button')).toBeDisabled();
    expect(document.querySelector('.spinner-sm')).not.toBeNull();
  });
});

describe('表单控件', () => {
  it('Input 输入触发 onInput', () => {
    const onInput = vi.fn();
    render(<Input value="" onInput={onInput} placeholder="商品名称" />);
    const input = screen.getByPlaceholderText('商品名称');
    fireEvent.input(input, { target: { value: '测试商品' } });
    expect(onInput).toHaveBeenCalledWith('测试商品', expect.anything());
  });

  it('NumberInput 支持 min/step 并回传字符串', () => {
    const onInput = vi.fn();
    render(<NumberInput value="" min="0" step="0.01" onInput={onInput} placeholder="数量" />);
    const input = screen.getByPlaceholderText('数量');
    expect(input).toHaveAttribute('min', '0');
    expect(input).toHaveAttribute('step', '0.01');
    fireEvent.input(input, { target: { value: '2.5' } });
    expect(onInput).toHaveBeenCalledWith('2.5');
  });

  it('Select 选择选项触发 onInput', () => {
    const onInput = vi.fn();
    render(
      <Select
        value=""
        onInput={onInput}
        placeholder="请选择"
        options={[
          { value: 'a', label: '选项 A' },
          { value: 'b', label: '选项 B' },
        ]}
      />,
    );
    const select = screen.getByRole('combobox');
    fireEvent.change(select, { target: { value: 'b' } });
    expect(onInput).toHaveBeenCalledWith('b', expect.anything());
  });

  it('Checkbox 与 Switch 回传布尔值', () => {
    const onCheck = vi.fn();
    const { rerender } = render(<Checkbox checked={false} onInput={onCheck} label="启用" />);
    fireEvent.click(screen.getByRole('checkbox'));
    expect(onCheck).toHaveBeenCalledWith(true);

    const onSwitch = vi.fn();
    rerender(<Switch checked={false} onInput={onSwitch} label="开关" />);
    fireEvent.click(screen.getByRole('checkbox'));
    expect(onSwitch).toHaveBeenCalledWith(true);
    expect(screen.getByText('开关')).toBeInTheDocument();
  });
});

describe('Tabs 与分页', () => {
  it('Tabs 高亮当前项并切换', () => {
    const onChange = vi.fn();
    render(
      <Tabs
        active="inout"
        onChange={onChange}
        tabs={[
          { key: 'inout', label: '进销存汇总' },
          { key: 'profit', label: '商品毛利' },
        ]}
      />,
    );
    expect(screen.getByRole('button', { name: '进销存汇总' })).toHaveClass('active');
    fireEvent.click(screen.getByRole('button', { name: '商品毛利' }));
    expect(onChange).toHaveBeenCalledWith('profit');
  });

  it('Pagination 触发翻页并禁用边界按钮', () => {
    const onPage = vi.fn();
    render(<Pagination page={1} size={10} total={25} onPage={onPage} />);
    expect(screen.getByText('共 25 条')).toBeInTheDocument();
    expect(screen.getAllByRole('button')[0]).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: '2' }));
    expect(onPage).toHaveBeenCalledWith(2);
  });
});

describe('Table', () => {
  it('渲染表头、数据和自定义列', () => {
    render(
      <Table
        columns={[
          { title: '编码', key: 'code' },
          { title: '金额', render: (r) => <Money value={r.amount} /> },
        ]}
        data={[{ id: 1, code: 'P-1', amount: 100.5 }]}
      />,
    );
    expect(screen.getByText('编码')).toBeInTheDocument();
    expect(screen.getByText('P-1')).toBeInTheDocument();
    expect(screen.getByText('100.50')).toBeInTheDocument();
  });

  it('空数据与加载状态', () => {
    const { rerender } = render(<Table columns={[{ title: '名称', key: 'name' }]} data={[]} empty="暂无商品" />);
    expect(screen.getByText('暂无商品')).toBeInTheDocument();
    rerender(<Table columns={[{ title: '名称', key: 'name' }]} data={[]} loading />);
    expect(screen.getByText('加载中…')).toBeInTheDocument();
  });
});

describe('Modal 与展示组件', () => {
  it('Modal 打开时渲染内容并支持关闭', () => {
    const onClose = vi.fn();
    render(
      <Modal open title="新增商品" onClose={onClose} footer={<button>底部</button>}>
        <p>表单内容</p>
      </Modal>,
    );
    expect(screen.getByText('新增商品')).toBeInTheDocument();
    expect(screen.getByText('表单内容')).toBeInTheDocument();
    expect(screen.getByText('底部')).toBeInTheDocument();
    fireEvent.click(screen.getByTitle('关闭'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('Modal 关闭时不渲染', () => {
    const { container } = render(<Modal open={false} title="隐藏" onClose={() => {}}>内容</Modal>);
    expect(container).toBeEmptyDOMElement();
  });

  it('NavItem 是可用键盘聚焦的链接并标记当前页', () => {
    const { rerender } = render(
      <NavItem href="#/products" active icon="box" label="商品管理" />,
    );
    const link = screen.getByRole('link', { name: '商品管理' });
    expect(link).toHaveAttribute('href', '#/products');
    expect(link).toHaveAttribute('aria-current', 'page');
    expect(link).toHaveClass('nav-item', 'active');

    rerender(<NavItem href="#/sales" icon="sell" label="销售出库" />);
    expect(screen.getByRole('link', { name: '销售出库' })).not.toHaveAttribute('aria-current');
  });

  it('Money/Qty/Empty 格式化', () => {
    const { rerender } = render(<Money value={-12.5} colored bold symbol />);
    expect(screen.getByText('¥-12.50')).toHaveClass('money-neg');
    expect(screen.getByText('¥-12.50')).toHaveClass('money-bold');

    rerender(<Qty value={1.5} unit="个" />);
    expect(screen.getByText('1.5')).toBeInTheDocument();
    expect(screen.getByText('个')).toBeInTheDocument();

    rerender(<Empty text="暂无数据" />);
    expect(screen.getByText('暂无数据')).toBeInTheDocument();
  });
});
