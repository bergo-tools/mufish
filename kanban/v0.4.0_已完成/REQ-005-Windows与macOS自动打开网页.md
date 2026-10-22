# REQ-005 Windows 与 macOS 启动时自动打开网页

## 背景

当前启动参数 `-open` 默认是 `false`，用户双击或命令行启动后，需要自己复制终端里的地址到浏览器。对 Windows / macOS 桌面用户来说，这不是“双击即用”的体验。

`main.go` 已有 `openURL`，在 macOS 使用 `open`，Windows 使用 `rundll32 url.dll,FileProtocolHandler`，只是默认没有触发。

## 用户故事

作为 Windows / macOS 桌面用户，我希望启动木鱼进销存后自动打开浏览器并进入登录页，以便双击程序就能直接使用，不需要手动复制地址。

## 验收标准

- [x] 在 Windows 上直接运行 `mufish`（不带参数）时，自动用默认浏览器打开 `http://127.0.0.1:8899`。
- [x] 在 macOS 上直接运行 `mufish`（不带参数）时，自动用默认浏览器打开 `http://127.0.0.1:8899`。
- [x] 使用 `-addr` 指定其他地址/端口时，打开的是实际监听地址对应的 URL。
- [x] 提供明确的关闭方式（例如 `-open=false`），关闭后不自动打开浏览器。
- [x] `-version` 只打印版本号，不启动服务、不打开浏览器。
- [x] 自动打开浏览器失败时，只记录警告日志，HTTP 服务继续正常运行。
- [x] Linux 行为保持现状：默认不自动打开浏览器，仍可通过 `-open` 显式开启（待确认）。
- [x] 不会重复打开多个浏览器窗口。
- [x] 增加可测试的 URL/平台命令选择逻辑，并在至少一个平台上完成手工验证。

## 实现方案

- 启动参数：
  - 将 `-open` 的默认值改为“Windows / macOS 为 true，其他平台为 false”，或新增 `-no-open` 并在 Windows/macOS 默认开启。
  - 最终采用哪种方式需在开始实现前确认。
- 平台命令：
  - macOS：`open <url>`
  - Windows：`rundll32 url.dll,FileProtocolHandler <url>`
  - Linux：保持 `xdg-open <url>`，是否默认开启待确认。
- 启动时序：服务开始监听后再打开浏览器；保留短暂延迟，避免浏览器先于服务就绪。
- 错误处理：打开失败只 `log.Printf` 警告，不退出进程。
- 测试计划：
  - 抽出 `browserCommand(goos, url)` 或等价函数，用单元测试覆盖 Windows / macOS / Linux 的命令选择。
  - 覆盖 `displayURL` 对 `127.0.0.1`、`0.0.0.0`、自定义端口的处理。
  - 在 Windows 和 macOS 上手工验证：启动、自动打开、`-open=false`、端口占用/浏览器打开失败场景。

## 已确认的口径

1. Windows / macOS 默认自动打开浏览器；Linux 默认不打开，可用 `-open` 显式开启。
2. 保留 `-open` 参数，用 `-open=false` 关闭；不新增 `-no-open`。
3. 启动时先完成端口监听，再打开浏览器；端口占用不会先弹出打不开的页面。

## 实现情况

- 状态：已完成
- 变更文件：
  - `main.go`：`defaultOpenBrowser`、`-open` 平台默认值、监听成功后再打开、失败仅警告。
  - `main_test.go`：平台默认值、`displayURL`、`browserCommand` 测试。
  - `README.md`：启动参数和示例更新。
- 实现要点：
  - `-open` 默认值：Windows/macOS 为 true，其他平台为 false。
  - 先 `net.Listen`，用实际监听地址生成 URL，再启动 `srv.Serve`；300ms 后调用平台命令打开。
  - macOS 用 `open`，Windows 用 `rundll32 url.dll,FileProtocolHandler`，Linux 用 `xdg-open`。
  - 打开失败只 `log.Printf` 警告，HTTP 服务继续运行。
  - `-version` 在启动服务前返回，不会打开浏览器。
- 测试与验证结果：
  - `go test ./...`、`go vet`、`go build ./...`：通过。
  - macOS 手工验证：用放在 PATH 前面的替代 `open` 捕获调用，默认启动打开 `http://127.0.0.1:18903`；`-addr 0.0.0.0:18905` 打开 `http://127.0.0.1:18905`；`-open=false` 不调用打开命令；移除 `open` 后只记录警告且服务正常响应；`-version` 只打印版本号。
  - `make cross` 交叉编译 Windows/macOS/Linux 通过（Windows 真机双击未验证）。
- 遗留问题 / 后续事项：
  - Windows 真机默认可视化验证、Linux 桌面环境验证需要在对应平台补充；命令选择已由单元测试覆盖。
  - 如果用户不希望自动弹浏览器，可传 `-open=false`。

## 变更记录

- 2026-10-22：创建需求，等待确认后再开始实现
- 2026-10-22：确认默认口径并完成实现；新增平台默认值测试，完成 macOS 手工验证和交叉编译。
