# REQ-004 组件测试与 E2E 测试

## 背景

前端目前只有 `format.js` 的纯函数测试，UI 组件（按钮、表单、表格、弹窗、消息）和真实业务流程都没有自动化验证。每次改动需要手工点击，容易遗漏交互回归。

## 用户故事

作为项目维护者，我希望有组件级测试覆盖常用 UI 组件，并用真实浏览器跑通登录、建商品、采购入库、销售出库等核心流程，以便在前端改动后快速发现回归。

## 验收标准

- [x] 引入组件测试运行环境（jsdom），覆盖至少 Button、Input、Select、Tabs、Table、Money/Qty、Modal 等组件。
- [x] 组件测试验证渲染、交互回调、禁用状态、空数据等关键行为。
- [x] 引入 Playwright E2E 测试，能自动启动测试服务器并访问真实页面。
- [x] E2E 覆盖：登录、首页看板、新建商品、采购入库并确认、库存查询可见、销售出库并确认、库存流水可查。
- [x] E2E 使用独立的临时数据目录，不污染 `data/mufish.db`。
- [x] `npm run test`、`npm run test:components`、`npm run test:e2e` 均可独立执行。
- [x] 使用浏览器工具实际跑通一遍 E2E 流程，确认页面行为与测试一致。

## 实现方案

- 组件测试：Vitest + jsdom + @testing-library/preact + @testing-library/jest-dom。
- E2E：Playwright，使用系统 Chrome（`channel: 'chrome'`），`webServer` 启动 Go 测试服务；测试数据放 `mktemp -d` 临时目录。
- 选择器优先使用可访问角色/名称或稳定的文本、组件类名，避免依赖脆弱的 DOM 层级。
- 测试账号使用内置 `admin / admin123`，测试结束清理临时数据。

## 实现情况

- 状态：已完成

- 变更文件：
  - `web/vitest.config.js`：jsdom + Oxc JSX（`importSource: preact`）配置。
  - `web/test/components/setup.js`：jest-dom 与 cleanup。
  - `web/test/components/ui.test.jsx`：13 个组件测试。
  - `web/playwright.config.js`：Playwright 配置、系统 Chrome、webServer。
  - `web/e2e/start-server.sh`：构建前端、编译 Go 服务、使用临时数据目录启动。
  - `web/e2e/app.spec.js`：2 个 E2E 流程。
  - `web/package.json`、`Makefile`、`AGENTS.md`、`README.md`、`.gitignore`：测试命令与产物忽略。
  - `web/package-lock.json`：测试依赖锁文件。

- 测试与验证结果：
  - `npm run test`：7 个纯函数测试通过。
  - `npm run test:components`：13 个组件测试通过。
  - `npm run test:e2e`：2 个 E2E 流程通过（登录看板、商品→采购→销售→库存→预告警→流水）。
  - `make test-all`、`make test-e2e`、`make vet`、`make web`：全部通过。
  - 浏览器手动验证：内置浏览器实际打开测试服务，登录后查询 2001 年流水，确认业务日期/记账时间展示与筛选正确。

- 遗留问题 / 后续事项：
  1. E2E 使用系统 Chrome，CI 环境需要安装 Chrome 或执行 `npx playwright install chromium`。
  2. 目前只有 2 条 E2E 主流程，后续可扩展权限、盘点、调拨、报表导出等场景。
  3. 组件测试覆盖 `ui.jsx` 的常用组件，页面级组件仍以 E2E 覆盖为主。

## 变更记录

- 2026-10-22：创建需求
- 2026-10-22：完成 Vitest 组件测试与 Playwright E2E，全部通过
