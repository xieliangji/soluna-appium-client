# soluna-appium-client 开发计划

> 文档状态：Active  
> 当前计划项：`DP-080`（已完成；下一项需显式选择）
> 最后更新：2026-08-31

## Agent 执行约束

- 只执行当前 Prompt 指定的一个 `DP-*` 项；Prompt 只说“下一项”时执行最靠前的 `Ready` 项。
- 队列顺序表示建议优先级；只有“硬前置”不可绕过。
- 开始前读取根 `AGENTS.md`、对应能力矩阵项和本任务涉及的权威文档。
- 不顺带实现相邻能力、后续计划项或无关重构。
- 设计缺失、硬前置未完成或权威文档冲突时停止并报告。
- 完成后同步必要测试、领域文档、能力矩阵和本项状态；不自动启动下一项。
- `Blocked` 项只有在阻塞条件满足且 Prompt 明确选择时执行。

状态：`Ready` 当前建议执行；`Queued` 等待选择；`Blocked` 等待条件；`Done` 已完成；`Superseded` 已替代。

## 队列

| 顺序 | 计划项 | 能力 ID | 状态 | 硬前置 |
|---:|---|---|---|---|
| 1 | `DP-010` 标准 Alert | `ALERT-001..003` | Done | — |
| 2 | `DP-020` 读取 Timeouts | `CFG-002` | Done | — |
| 3 | `DP-030` Session Settings | `CFG-003..004` | Done | — |
| 4 | `DP-040` Runtime Discovery 设计 | `DISC-001..003` | Done | — |
| 5 | `DP-041` Runtime Discovery 实现 | `DISC-001..003` | Done | DP-040 |
| 6 | `DP-050` Screenshot 资源模型 | `VIS-003..004` | Done | — |
| 7 | `DP-051` Element Screenshot | `ELM-006..007` | Done | DP-050 |
| 8 | `DP-060` Viewport 坐标设计 | `VIS-006` | Done | — |
| 9 | `DP-061` ViewportRect 实现 | `VIS-006` | Done | DP-060 |
| 10 | `DP-070` 通用显式等待 | `WAIT-001` | Done | — |
| 11 | `DP-071` Element 显式等待 | `WAIT-002` | Done | DP-070 |
| 12 | `DP-080` Pull Logs 设计 | `LOG-001..002` | Done | — |
| 13 | `DP-081` Pull Logs 实现 | `LOG-001..002` | Queued | DP-080 |
| 14 | `DP-090` Web Context 几何设计 | `CTX-001` | Queued | — |
| 15 | `DP-091` Context API 实现 | `CTX-001` | Queued | DP-090 |
| 16 | `DP-100` Keyboard 语义设计 | `KBD-001..002` | Queued | — |
| 17 | `DP-101` Keyboard 实现 | `KBD-001..002` | Queued | DP-100 |
| 18 | `DP-110` 应用放入后台 | `NAV-001` | Queued | — |
| 19 | `DP-111` 屏幕方向 | `NAV-002` | Queued | — |
| 20 | `DP-120` 活动 App ID | `DEV-001` | Queued | — |
| 21 | `DP-121` 设备时间 | `DEV-002` | Queued | — |
| 22 | `DP-130` Deep Link | `NAV-003` | Queued | — |
| 23 | `DP-140` 兼容性矩阵结构 | `INF-008` | Queued | — |
| 24 | `DP-141` 跨 Host Smoke | `INF-008` | Blocked | DP-140 + 实际环境 |
| 25 | `DP-150` XCUITest Picker Wheel | `XCUI-003` | Queued | DP-140 |
| 26 | `DP-151` XCUITest Alert label | `XCUI-004` | Queued | DP-010, DP-140 |
| 27 | `DP-152` XCUITest Simulated Location | `XCUI-005` | Queued | DP-140 |
| 28 | `DP-160` UiAutomator2 Driver 门禁 | `UIA-001` | Queued | — |
| 29 | `DP-161` UiAutomator2 能力复审 | `UIA-002..004` | Queued | DP-160 |
| 30 | `DP-170` BiDi 模型设计 | `BIDI-001..002`, `INF-006` | Queued | DP-140 |
| 31 | `DP-171` BiDi 核心实现 | `BIDI-001..002`, `INF-006` | Queued | DP-170 |
| 32 | `DP-172` Streaming Logs | `LOG-003` | Queued | DP-171 |
| 33 | `DP-173` XCUITest System Monitor | `XCUI-006` | Queued | DP-140, DP-171 |
| 34 | `DP-174` XCUITest Network Monitor | `XCUI-007` | Queued | DP-140, DP-171 |
| 35 | `DP-175` XCUITest Syslog / Crashlog | `XCUI-008` | Queued | DP-140, DP-171 |
| 36 | `DP-190` 稳定版本收敛 | 选定范围 | Queued | 选定范围完成 |

## 第一阶段：通用 HTTP 能力

### DP-010 标准 Alert

- 实现 `Session.AlertText`、`AcceptAlert`、`DismissAlert`、`SetAlertText`。
- 使用标准 W3C Alert 命令并严格解码文本与 null；`AlertText` 以 `hasText` 区分空字符串和缺失文本。
- `no such alert` 映射为独立的 `CodeAlertNotFound`，因为该 W3C 错误明确表示 Alert 资源不存在，调用方可据此区分通用命令失败。
- 覆盖协议、错误、Delivery 和本地失败零远端请求。
- 排除 `HasAlert`、按 label 处理、自动等待和自动重试。

### DP-020 读取 Timeouts

- 实现 `Session.Timeouts`，按 Appium 3 Get Timeouts 读取 Command、Implicit。
- 返回独立的 `CurrentTimeouts` 结果类型，保留既有 `Timeouts` 设置类型及其
  `Script`、`PageLoad`、`Implicit` 字段，避免无关的公共 API 破坏。
- 区分缺失字段、显式 null 与零值；校验整数毫秒、负数和 `time.Duration` 溢出。
- 不缓存远端结果，不修改现有 setter。

### DP-030 Session Settings

- 实现 `Session.Settings`、`Session.UpdateSettings` 和开放 `Settings` 类型。
- 返回独立的深拷贝，更新只发送调用方明确提供的增量字段。
- 不缓存、不 normalize、不维护 Driver setting 白名单。
- 排除平台强类型 setting helper。

### DP-040 Runtime Discovery 设计

确定并写入 `docs/design.md`：

- Appium、Driver、Plugin Source provenance；
- HTTP/BiDi command 与 Execute Method 类型；
- `params[].required` 参数元数据、section 内必需 child；
- 未知字段、深拷贝和按协议类型分开的 `Supports` 精确匹配规则。

已废弃原先的扁平 `CatalogEntry{Name, Origin, Kind, Extra}` 模型，改为按
Appium 3 `rest`/`bidi`/`base`/`driver`/`plugins[name]` 层级表达 HTTP、BiDi 和
Execute Method 三类 execution identity，并固定 Source provenance、`params[].required`
参数元数据、section 内必需 child 以及可选 section 的缺失与空 object 语义；
具体 HTTP 命令、解码和本地 helper 在 DP-041 实现。

排除远端请求、缓存、自动门禁和 fallback。

### DP-041 Runtime Discovery 实现

- 实现 Commands、Extensions 快照读取和本地 `Supports`。
- 使用 DP-040 类型；每次返回独立快照。
- 普通 SDK 命令不得隐式调用 Discovery。
- 排除缓存、自动门禁、fallback 和成功保证。

## 第二阶段：视觉、等待与 Pull Logs

### DP-050 Screenshot 资源模型

- 增加 `MaxScreenshotResponseBytes` 和 `Session.ScreenshotTo(io.Writer)`。
- `Screenshot` 与 `ScreenshotTo` 复用同一 Base64 解码路径。
- 覆盖部分写入、Writer 失败和 context 结束。
- 将 Screenshot 移入明确行为文件。
- 排除 Element Screenshot、Viewport Screenshot 和裁剪。

### DP-051 Element Screenshot

- 实现 `Element.Screenshot`、`Element.ScreenshotTo`。
- 复用 DP-050 的上限和解码路径。
- 覆盖 stale、远端错误、解码错误和部分写入。
- 排除自动滚动、可见性恢复、本地裁剪和坐标等价承诺。

### DP-060 Viewport 坐标设计

确定并写入 `docs/coordinate-system.md`：

- `Rect`/`Point` 与 `PixelRect` 的边界；
- 两 Driver 的 Viewport 语义；
- orientation、status bar、scale 的事实边界；
- 数值校验；
- `ViewportRect` 不参与现有 Find/Tap。

已固定 WebDriver 几何、Driver 像素几何和具体 Screenshot 像素平面的边界：
`Rect`/`Point` 继续表示 WebDriver 几何，`PixelRect` 只表示 Driver 报告的整数
像素几何，不自动绑定某一次 Screenshot；XCUITest 与 UiAutomator2 的
`mobile: viewportRect` 结果由 Driver 负责选择单位，客户端只做严格承载和
数值校验。已明确 SDK 每次发起读取且不缓存返回值、通过根包统一 Execute Script
链执行，以及 orientation、status bar、scale/density 不触发隐式转换；Driver
内部基础事实的缓存和刷新时机仍由远端实现决定。能否将 ViewportRect 用作某张
Screenshot 的 crop rectangle 属于带环境、Context 和采集路径条件的兼容性事实。
已记录验证流程、错误/Delivery 边界和未来 DP-061 的实现输入。

排除运行时代码和自动坐标转换。

### DP-061 ViewportRect 实现

- 实现 `PixelRect`、`Session.ViewportRect`。
- 按远端 AutomationName 映射 Driver 命令；未知 Driver 本地拒绝。
- 覆盖两 Driver 的严格协议测试。
- 排除 Viewport Screenshot、隐式缩放和修改 Find/Tap。

已完成根包实现和协议测试：通过统一 Execute Script 链发送
`mobile: viewportRect`，对 XCUITest 与 UiAutomator2 执行精确 Driver 门禁，并对
四个整数像素字段执行原点、正面积和端点溢出校验。结果不缓存、不参与现有
Find/Tap，也不建立 Screenshot 像素平面关联。

### DP-070 通用显式等待

- 实现最小 `wait.Until`。
- context 控制总期限；轮询间隔明确。
- 条件可表达继续、成功和失败。
- 排除 Session Timeout 修改、业务条件和 Session 恢复。

已完成 `wait.Until`：条件先立即执行，`false, nil` 按正轮询间隔继续，
`true, nil` 成功，非 nil error 立即原样返回；context 终止间隔等待和后续轮询。
实现不发送远端命令、不修改 Session 超时、不自动重试或恢复 Session，并补充
了本地单元测试与显式等待语义文档。

### DP-071 Element 显式等待

- 实现 `wait.Element`、`wait.Elements`，只调用公共 Find API。
- 只重试明确为暂态的未找到结果，并保留最终错误。
- 记录 Implicit Wait 与显式轮询叠加风险。
- 排除 stale 自动恢复和 Element 自动重定位。

已完成 `wait.Element` 与 `wait.Elements`：两者接受根包 Session 或 Element
作为查找作用域（并允许满足相同签名的本地 finder），先立即调用公共 Find API，
并按正的显式间隔继续轮询。
只有 `CodeElementNotFound` 和合法空集合可继续；stale、Session 丢失、参数、
响应格式及传输错误立即返回。根包 context 错误即使发生在 Find 调用内部，也会
与此前未找到诊断一并保留；本地 finder malformed 结果返回普通本地契约错误，
不伪造远端 Code/Delivery。Elements 仅有空集合时返回 context 结果；实现不修改
隐式或命令超时，不自动重新定位或恢复 Element，并补充了 stub 与协议回归测试。

### DP-080 Pull Logs 设计

确定并写入 `docs/design.md`：

- 开放 Log Type；
- Log Entry、时间字段和未知字段；
- 单次读取上限；
- Driver 消费缓存语义；
- Writer 形式是否有实际价值。

排除 Streaming Logs 和 BiDi。

已完成 Pull Logs 设计：根包使用开放的 `LogType`，通过 Appium 3 精确的
`/session/{id}/se/log/types` 和 `/session/{id}/se/log` 路由提供一次性读取；
`LogEntry` 严格要求 Unix epoch 毫秒 `int64`、`level` 和 `message`，未知字段
递归保存在独立 `Extra` 中并保持 Entry 顺序。DP-081 将增加独立的
`MaxLogResponseBytes`（默认 32 MiB），超限或任一条目格式错误都不返回部分结果。
设计不假设 Driver 读取会清空或保留缓存，不做本地缓存、游标、轮询、合并、去重、
自动重试或 Runtime Discovery 门禁；结构化集合不增加 `Writer`/JSONL 交付形式。
真实 Driver 消费语义和版本组合仍需在兼容性验证中单独记录。

### DP-081 Pull Logs 实现

- 实现 Log Types 和按类型读取。
- 增加 `MaxLogResponseBytes`。
- 严格解码集合和条目，不假设读取会清空缓存。
- 排除自动轮询、合并、去重和持续订阅。

## 第三阶段：Context 与通用设备交互

### DP-090 Web Context 几何设计

确定并写入 `docs/design.md` 与 `docs/coordinate-system.md`：

- Native/Web Context 识别；
- DOM Element Rect 与浏览器 viewport；
- Web Context 下 Find/Tap；
- Context 切换后的本地状态；
- Hybrid/Safari 的版本和 Host 验证边界。

排除运行时代码。

### DP-091 Context API 实现

- 实现 Context 列表、当前 Context 和切换。
- 严格解码；切换失败不推测本地状态。
- 保持 Native 行为，实现 DP-090 的 Web 几何策略。
- 排除自动 Context fallback 和未设计的 Hybrid 发现。

### DP-100 Keyboard 语义设计

确定并写入 `docs/design.md`：

- 两 Driver 的键盘状态语义；
- “发送关闭请求”与“确认已关闭”；
- 公共入口和失败语义；
- 无关闭按钮等限制。

排除运行时代码、特殊键和 IME 管理。

### DP-101 Keyboard 实现

- 实现 DP-100 确认的公共入口。
- 覆盖两 Driver 请求、响应和失败。
- 不把 Driver 的尝试结果包装成确定事实。
- 排除自动输入恢复和 IME 管理。

### DP-110 应用放入后台

- 实现只放入后台且不自动恢复的操作。
- 恢复由现有 `ActivateApp` 显式执行。
- 排除定时恢复和通用 Back。

### DP-111 屏幕方向

- 实现 Portrait/Landscape 强类型及读取、设置。
- 严格解码且不缓存状态。
- 覆盖两 Driver；排除空间 Rotation。

### DP-120 活动 App ID

- 实现统一前台 App ID。
- 显式映射 iOS bundle ID 与 Android package。
- 不从 Capability 猜测；排除进程枚举和安装信息。

### DP-121 设备时间

- 实现可校验的设备时间结果。
- 不静默回退 Host 时间。
- 记录 Driver/Host 差异；排除时间和时区设置。

### DP-130 Deep Link

- 实现通用 Deep Link。
- 按远端 AutomationName 映射 iOS `bundleId` 与 Android `package`。
- 未知 Driver 本地拒绝。
- 排除浏览器历史、通用 Back 和页面自动断言。

## 第四阶段：兼容性与平台能力

### DP-140 兼容性矩阵结构

在 `docs/compatibility.md` 定义：

- Runtime Profile 和逐能力验证记录；
- SDK、Appium、Driver、WDA/UiAutomator2 Server、设备 OS/类型、Host OS、连接和启动方式；
- iOS 17.x macOS、iOS 18+ RemoteXPC 三 Host、低于 iOS 17 Legacy Lane；
- Android/UiAutomator2 对等结构；
- `Official`、`BestEffort` 与本项目 `Verified` 的区别。

不得把未实测组合标记为 `Verified`。

### DP-141 跨 Host Smoke

实际环境具备后：

- 使用同一公共 API smoke suite 分别验证 macOS、Windows、Linux。
- iOS 17 与 iOS 18+ 分开记录。
- 失败记录限制，不修改 SDK 伪造兼容。

### DP-150 XCUITest Picker Wheel

- 实现强类型方向和 offset。
- 校验 Session、Element 归属和 XCUITest Driver。
- 不重复普通 Swipe。

### DP-151 XCUITest Alert label

- 只实现标准 Alert 无法表达的 label 增量。
- 空 label 本地拒绝；Driver mismatch 零远端请求。
- 不重复根包 Accept/Dismiss。

### DP-152 XCUITest Simulated Location

- 实现 Get/Set/Reset 和稳定位置类型。
- 校验数值范围，明确 iOS、Driver/WDA 和 Host 条件。
- 不合并旧混合 GeoLocation API。

### DP-160 UiAutomator2 Driver 门禁

- 只使用远端确认的 AutomationName。
- 不 normalize、不探测、不调用 Healthy。
- mismatch 返回 `CodeUnsupported + DeliveryNotSent`。
- 不新增 Android 公共平台函数。

### DP-161 UiAutomator2 能力复审

评审 `UIA-002..004`：

- 真实使用场景和收益；
- 通用能力是否已有可靠替代；
- Host 可移植性；
- 通过项从 `Deferred` 调整为 `Accepted`。

同一任务不实现通过评审的能力。

## 第五阶段：BiDi 与持续事件

### DP-170 BiDi 模型设计

确定并写入 `docs/design.md`，必要时新增 ADR：

- Endpoint、连接建立和根 Session 所有权；
- command ID、响应和事件关联；
- 订阅及消费者语义；
- context、Session Close 和远端关闭；
- 单消息、队列和累计上限；
- 溢出结果、WebSocket 依赖和不自动重连边界。

排除 Streaming Logs 和平台监控实现。

### DP-171 BiDi 核心实现

- 实现 `internal/bidi`、Fake BiDi Server 和 Session 绑定的公共订阅入口。
- 不创建独立公共 BiDi Client。
- 实现有界消息、队列、取消和关闭。
- 覆盖关联、协议错误、溢出和并发关闭。
- 运行 `go test -race ./...`。
- 排除自动重连和具体监控。

### DP-172 Streaming Logs

- 实现开放事件类型以及订阅、取消、关闭和溢出。
- 与 Pull Logs 保持独立语义。
- 排除 XCUITest DVT typed events。

### DP-173 XCUITest System Monitor

- 实现 iOS 18+ 真机 RemoteXPC/BiDi 控制和 typed event。
- 记录第一帧、采样周期和非连续真值边界。
- 完成协议测试和三 Host 验证清单；真实结果只写入兼容性文档。

### DP-174 XCUITest Network Monitor

- 分层定义 interface、connection、traffic sample 和 serial 关联。
- 明确不是 HTTP 抓包。
- 完成协议测试、资源边界和三 Host 验证清单。

### DP-175 XCUITest Syslog / Crashlog

- 分开定义 syslog 与 crashlog 生命周期。
- 定义 typed event、未知字段、敏感数据和队列边界。
- 不把完整日志写入 Observer 或默认错误。
- 完成协议测试和三 Host 验证清单。

## 第六阶段：稳定版本收敛

### DP-190 稳定版本收敛

- 审查选定范围内的导出 API、GoDoc、命名、零值和包边界。
- 确认根包与平台包无重复能力，平台导出函数使用 `IOS` / `Android` 前缀。
- 完成命令、错误、坐标、兼容性和发布文档。
- 运行完整 `go test ./...`、`go test -race ./...` 和声明环境的 smoke suite。
- 未验证组合不作稳定承诺。
- 不为表面 API 完整加入未规划能力。
