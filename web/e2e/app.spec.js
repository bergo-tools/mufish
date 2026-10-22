import { test, expect } from '@playwright/test';

const today = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai' }).format(new Date());

async function login(page) {
  await page.goto('/');
  await page.getByPlaceholder('用户名').fill('admin');
  await page.getByPlaceholder('口令').fill('admin123');
  await page.getByRole('button', { name: /登\s*录/ }).click();
  await expect(page.locator('.header h2')).toHaveText('首页看板');
  await expect(page.getByText('本月销售额')).toBeVisible();
}

async function nav(page, navLabel, cardText) {
  await page.locator('.nav-item').filter({ hasText: navLabel }).click();
  await expect(page.locator('.header h2')).toHaveText(navLabel);
  if (cardText) {
    await expect(page.locator('.content')).toContainText(cardText);
  }
}

async function toast(page, text) {
  await expect(page.locator('.toast').filter({ hasText: text }).first()).toBeVisible({ timeout: 10000 });
}

test('登录后加载首页看板', async ({ page }) => {
  await login(page);
  await expect(page.locator('.stat-value').first()).toBeVisible();
  await expect(page.getByText('近 14 天走势')).toBeVisible();
  await expect(page.getByText('最近单据')).toBeVisible();
});

test('商品 → 采购入库 → 销售出库 → 库存流水', async ({ page }) => {
  const suffix = Date.now().toString().slice(-8);
  const code = 'E2E-' + suffix;
  const name = 'E2E测试商品' + suffix;

  await login(page);

  // 1. 新建商品。
  await nav(page, '商品管理', '商品列表');
  await page.getByRole('button', { name: '新增商品' }).click();
  const productModal = page.locator('.modal');
  await expect(productModal).toBeVisible();
  await productModal.getByLabel('商品编码').fill(code);
  await productModal.getByLabel('商品名称').fill(name);
  await productModal.getByLabel('参考进价').fill('10');
  await productModal.getByLabel('参考售价').fill('20');
  await productModal.getByLabel('安全库存').fill('10');
  await productModal.getByRole('button', { name: '保存' }).click();
  await expect(productModal).toHaveCount(0);
  await toast(page, '新增成功');

  const productSearch = page.getByPlaceholder('商品名称 / 编码 / 条码');
  await productSearch.fill(code);
  await productSearch.press('Enter');
  await expect(page.locator('.table tbody tr').filter({ hasText: code })).toHaveCount(1);

  // 2. 采购入库 10 个。
  await nav(page, '采购入库', '采购入库单');
  await page.getByRole('button', { name: '新建采购单' }).click();
  const purchaseModal = page.locator('.modal');
  await expect(purchaseModal).toBeVisible();
  await purchaseModal.locator('.form-grid select').nth(0).selectOption({ index: 1 });
  await purchaseModal.locator('.form-grid select').nth(1).selectOption({ label: '主仓库' });
  const purchaseProduct = purchaseModal.locator('.doc-items-table select').first();
  await expect(purchaseProduct.locator('option').filter({ hasText: code })).toHaveCount(1);
  await purchaseProduct.selectOption({ label: code + ' ' + name });
  await purchaseModal.locator('.doc-items-table input[type="number"]').nth(0).fill('10');
  await purchaseModal.locator('.doc-items-table input[type="number"]').nth(1).fill('10');
  await purchaseModal.getByRole('button', { name: '保存' }).click();
  await expect(purchaseModal).toHaveCount(0);
  await toast(page, '新增成功');

  const purchaseRow = page.locator('.table tbody tr').filter({ hasText: '主仓库' }).first();
  await expect(purchaseRow).toBeVisible();
  await purchaseRow.getByRole('button', { name: '确认入库' }).click();
  await page.getByRole('button', { name: '确定' }).click();
  await toast(page, '入库确认成功');

  // 3. 销售出库 4 个。
  await nav(page, '销售出库', '销售出库单');
  await page.getByRole('button', { name: '新建销售单' }).click();
  const saleModal = page.locator('.modal');
  await expect(saleModal).toBeVisible();
  await saleModal.locator('.form-grid select').nth(0).selectOption({ index: 1 });
  await saleModal.locator('.form-grid select').nth(1).selectOption({ label: '主仓库' });
  const saleProduct = saleModal.locator('.doc-items-table select').first();
  await expect(saleProduct.locator('option').filter({ hasText: code })).toHaveCount(1);
  await saleProduct.selectOption({ label: code + ' ' + name });
  await saleModal.locator('.doc-items-table input[type="number"]').nth(0).fill('4');
  await saleModal.locator('.doc-items-table input[type="number"]').nth(1).fill('20');
  await saleModal.getByRole('button', { name: '保存' }).click();
  await expect(saleModal).toHaveCount(0);
  await toast(page, '新增成功');

  const saleRow = page.locator('.table tbody tr').filter({ hasText: '主仓库' }).first();
  await expect(saleRow).toBeVisible();
  await saleRow.getByRole('button', { name: '确认出库' }).click();
  await page.getByRole('button', { name: '确定' }).click();
  await toast(page, '出库确认成功');

  // 4. 库存查询：结存应为 6。
  await nav(page, '库存查询', '库存明细');
  const stockSearch = page.getByPlaceholder('商品编码 / 名称 / 规格');
  await stockSearch.fill(code);
  await page.getByRole('button', { name: '查询' }).click();
  const stockRow = page.locator('.table tbody tr').filter({ hasText: code });
  await expect(stockRow).toHaveCount(1);
  await expect(stockRow.locator('td').nth(6)).toHaveText('6');

  // 5. 低库存口径：库存查询与库存预警都应包含该商品（安全库存 10 > 6）。
  await page.locator('.switch').filter({ hasText: '仅看低库存' }).click();
  await expect(page.locator('.table tbody tr').filter({ hasText: code })).toHaveCount(1);

  await nav(page, '库存预警', '库存预警');
  await expect(page.locator('.table tbody tr').filter({ hasText: code }).first()).toBeVisible();

  // 6. 库存流水：业务日期与记账时间都在，且采购/销售各一条。
  await nav(page, '库存流水', '库存流水');
  const ledgerSearch = page.getByPlaceholder('商品 / 单据编号 / 备注');
  await ledgerSearch.fill(code);
  await page.getByRole('button', { name: '查询' }).click();
  const ledgerRows = page.locator('.table tbody tr').filter({ hasText: code });
  await expect(ledgerRows).toHaveCount(2);
  await expect(page.locator('.table thead')).toContainText('业务日期');
  await expect(page.locator('.table thead')).toContainText('记账时间');
  await expect(ledgerRows.first().locator('td').nth(0)).toHaveText(today);
  await expect(page.locator('.table tbody')).toContainText('采购入库');
  await expect(page.locator('.table tbody')).toContainText('销售出库');
});

test('盘点可添加零库存商品并完成盘盈调整', async ({ page }) => {
  const suffix = Date.now().toString().slice(-8);
  const code = 'E2E-ZERO-' + suffix;
  const name = '零库存盘点商品' + suffix;

  await login(page);

  // 新建一个从未入库的商品。
  await nav(page, '商品管理', '商品列表');
  await page.getByRole('button', { name: '新增商品' }).click();
  const productModal = page.locator('.modal');
  await expect(productModal).toBeVisible();
  await productModal.getByLabel('商品编码').fill(code);
  await productModal.getByLabel('商品名称').fill(name);
  await productModal.getByRole('button', { name: '保存' }).click();
  await expect(productModal).toHaveCount(0);
  await toast(page, '新增成功');

  // 新建盘点单，把零库存商品加进来并录入实盘数 5。
  await nav(page, '库存盘点', '盘点单列表');
  await page.getByRole('button', { name: '新建盘点' }).click();
  const takeModal = page.locator('.modal');
  await expect(takeModal).toBeVisible();
  await takeModal.locator('.form-grid select').first().selectOption({ label: '主仓库' });
  await expect(takeModal.locator('.doc-items-table tbody tr').first()).toBeVisible();

  const search = takeModal.getByPlaceholder('搜索商品后添加到盘点');
  await search.fill(code);
  await takeModal.getByRole('button', { name: '搜索商品' }).click();
  const productSelect = takeModal.locator('select').filter({
    has: page.locator('option', { hasText: code }),
  });
  await expect(productSelect).toHaveCount(1);
  await productSelect.selectOption({ label: code + ' ' + name });
  await takeModal.getByRole('button', { name: '添加商品' }).click();

  const takeRow = takeModal.locator('.doc-items-table tbody tr').filter({ hasText: code });
  await expect(takeRow).toHaveCount(1);
  await takeRow.locator('input[type="number"]').fill('5');
  await takeModal.getByRole('button', { name: '保存' }).click();
  await expect(takeModal).toHaveCount(0);
  await toast(page, '新建成功');

  const takeListRow = page.locator('.table tbody tr').filter({ hasText: '主仓库' }).first();
  await expect(takeListRow).toBeVisible();
  await takeListRow.getByRole('button', { name: '完成盘点' }).click();
  await page.getByRole('button', { name: '确定' }).click();
  await toast(page, '完成盘点成功');

  // 库存查询应出现该商品的 5 个结存。
  await nav(page, '库存查询', '库存明细');
  await page.getByPlaceholder('商品编码 / 名称 / 规格').fill(code);
  await page.getByRole('button', { name: '查询' }).click();
  const stockRow = page.locator('.table tbody tr').filter({ hasText: code });
  await expect(stockRow).toHaveCount(1);
  await expect(stockRow.locator('td').nth(6)).toHaveText('5');
});
