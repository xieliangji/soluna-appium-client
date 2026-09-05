# soluna-appium-client 开发计划

> 文档状态：Active  
> 当前计划项：`DP-121`（已完成；下一项需显式选择）
> 最后更新：2026-09-04

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
| 13 | `DP-081` Pull Logs 实现 | `LOG-001..002` | Done | DP-080 |
| 14 | `DP-090` Web Context 几何设计 | `CTX-001` | Done | — |
| 15 | `DP-091` Context API 实现 | `CTX-001` | Done | DP-090 |
| 16 | `DP-100` Keyboard 语义设计 | `KBD-001..002` | Done | — |
| 17 | `DP-101` Keyboard 实现 | `KBD-001..002` | Done | DP-100 |
| 18 | `DP-110` 应用放入后台 | `NAV-001` | Done | — |
| 19 | `DP-111` 屏幕方向 | `NAV-002` | Done | — |
| 20 | `DP-120` 活动 App ID | `DEV-001` | Done | — |
| 21 | `DP-121` 设备时间 | `DEV-002` | Done | — |
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

补充评估：当前 `MaxScreenshotResponseBytes` 同时作为完整 wire 响应和解码后
截图数据的共同硬上限；`MaxRecordingResponseBytes` 同理。该保守资源策略暂不
拆分为两个公共字段，只有独立 decoded 配额成为明确需求时再单独设计迁移规则。

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

补充优化（不改变计划队列）：根包增加固定路由的高级
`Session.ExecuteScriptWithOperation` 与
`Session.ExecuteScriptWithOperationAndDecode` 入口，允许平台扩展保留独立的本地
operation identity，并让 typed value decoder 在统一执行链的 decoder slot 中完成，
使响应格式错误的 Error 与 Observer Finished 保持相同的 StatusCode、Delivery 和
operation。operation 采用 `[a-z][a-z0-9_]{0,63}` 格式，不开放任意 Method/Route，
也不引入 Discovery、fallback 或 retry。

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

补充评估：Find 的 Window 交集语义和单次候选快照保持不变；remote command
amplification 先通过基线数据评估，不在没有测量证据时替换查找算法。

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
合法 UTF-8 的 `LogType` 包括空字符串也原样透传；非法 UTF-8 无法无损编码为 JSON
string，在发送前按 `CodeInvalidArgument`/`DeliveryNotSent` 拒绝。可用集合是可能随
Driver、Capability、当前 Context 和其他 Session 状态变化的动态快照；`LogEntry` 严格要求 Unix epoch 毫秒
`int64`、`level` 和 `message`，未知字段递归保存在独立 `Extra` 中并保持 Entry 顺序。
DP-081 已增加独立的
`MaxLogResponseBytes`（默认 32 MiB），超限或任一条目格式错误都不返回部分结果。
设计不假设 Driver 读取会清空或保留缓存，不做本地缓存、游标、轮询、合并、去重、
自动重试或 Runtime Discovery 门禁；结构化集合不增加 `Writer`/JSONL 交付形式。
真实 Driver 消费语义和版本组合仍需在兼容性验证中单独记录。

### DP-081 Pull Logs 实现

- 实现 Log Types 和按类型读取（包括空及其他合法 UTF-8 `LogType` 的透传，由远端决定是否支持；非法 UTF-8 在发送前拒绝）。
- 增加 `MaxLogResponseBytes`。
- 严格解码集合和条目，不假设读取会清空缓存。
- 排除自动轮询、合并、去重和持续订阅。

已完成根包 Pull Logs 实现和协议回归测试：通过统一 HTTP 执行链发送精确的
`/se/log/types` 与 `/se/log` 请求，完整保留合法 UTF-8 的开放 Log Type、Entry 顺序、未知字段
及 `json.Number` 数字值，并执行标准字段、时间戳、UTF-8/surrogate 和整体成功校验。
`MaxLogResponseBytes` 使用 32 MiB 默认值并在完整 HTTP 响应边界生效；错误和
Delivery 继续沿用统一命令语义。实现不缓存、不轮询、不合并、不去重或重试，真实
Driver 消费语义和兼容性仍未验证。

## 第三阶段：Context 与通用设备交互

### DP-090 Web Context 几何设计

确定并写入 `docs/design.md` 与 `docs/coordinate-system.md`：

- Native/Web Context 识别；
- DOM Element Rect 与浏览器 viewport；
- Web Context 下 Find/Tap；
- Context 切换后的本地状态；
- Hybrid/Safari 的版本和 Host 验证边界。

已完成 Web Context 几何设计：Context 名称按不透明 UTF-8 快照保留，仅精确
`NATIVE_APP`、`WEBVIEW`、带非空后缀的 `WEBVIEW_` 或精确 `CHROMIUM` 进入已定义
的 Native/Web 策略，其他名称保持 Unknown，不触发隐式 fallback。Native 继续使用
`WindowRect` 与 Element Rect 的 WebDriver 交集；Web 使用由
`window.scrollX`/`window.scrollY`/`window.innerWidth`/`window.innerHeight` 定义的
CSS layout viewport，将 WebDriver 文档相对 DOM Element Rect 平移到 viewport
坐标后计算正面积交集，不做 CSS/device-pixel、status bar、orientation 或
`PixelRect` 转换。Find/Tap 不自动滚动、点击
fallback、iframe 遍历或 stale 重定位；Context 列表、当前 Context 和切换不缓存，
切换结果不推测后续本地状态。Context API 只使用 Appium 3 当前注册的
`/session/{sessionId}/contexts` 与 `/session/{sessionId}/context` 路由，不改写或
fallback 到替代路由；Unknown Context 下组合
Find/Tap 返回主体操作的 `CodeUnsupported` + `DeliveryNotSent`，成功的
`CurrentContext` 探针只保留在自身 Observer 事件中。已记录 Appium 3、XCUITest/Safari/WKWebView、
UiAutomator2/Android WebView、Driver/WDA/Chromedriver、设备 OS、真机/模拟器和
Host OS 的分组合规性验证边界；真实结果仍需写入 `docs/compatibility.md`。

排除运行时代码。

### DP-091 Context API 实现

- 实现 Context 列表、当前 Context 和切换。
- 按 DP-090 固定的 Appium 3 裸 `/context(s)` 路由严格解码；切换失败或投递
  不确定时不推测本地状态。
- 保持 Native 行为，实现 DP-090 的 Web CSS viewport/DOM Rect 几何策略。
- 排除自动 Context fallback 和未设计的 Hybrid 发现。

### DP-100 Keyboard 语义设计

确定并写入 `docs/design.md`：

- 两 Driver 的键盘状态语义；
- “发送关闭请求”与“确认已关闭”；
- 公共入口和失败语义；
- 无关闭按钮等限制。

排除运行时代码、特殊键和 IME 管理。

已完成设计（2026-09-03）：

- 确定根包 `Session.KeyboardShown` 与 `Session.DismissKeyboard` 两个公共入口，
  使用 Appium 3 common `is_keyboard_shown` / `hide_keyboard` 路由并复用统一执行链；
- 将关闭定义为一次请求，将最终状态确认定义为调用方随后显式读取
  `KeyboardShown`；不缓存、不自动探测、等待、轮询、重试或恢复；
- 记录 XCUITest 与 UiAutomator2 的状态探测、关闭实现和布尔返回差异；严格校验
  Driver boolean 并原样返回，但不把关闭响应包装成跨 Driver 的确定事实；
- 明确无特殊键/strategy、Back 或其他 fallback，且不管理 IME、Context 或关闭按钮；
- DP-101 已完成运行时代码和协议回归测试；真实兼容性验证仍待记录。

### DP-101 Keyboard 实现

- 实现 DP-100 确认的公共入口。
- `DismissKeyboard` 返回 `(bool, error)`，保留成功响应中的原始 Driver-reported
  boolean；该值不表示关闭后的最终状态，错误时返回值为零值且不可用于推断状态。
- 固定 `KeyboardShown` / `DismissKeyboard` 的 Error.Operation 与 Observer identity
  为 `keyboard_shown` / `dismiss_keyboard`。
- 覆盖两 Driver 请求、响应和失败；响应 decoder 必须在统一 `executeCommand` 链的
  decoder slot、`Observer.OnCommandFinished` 之前执行。
- 严格覆盖成功值 `true`、`false` 以及非法的 `null`、数字、字符串、对象、数组；
  使用 `json.RawMessage` 类型检查或带 nil 检查的 `*bool`，不直接解码到 Go `bool`。
- 协议测试覆盖 GET 无 `Content-Type`、POST 固定 `{}`、Session ID 路径转义及
  `DeliveryUnknown` 不重放；兼容性验收记录 Driver 内部 Done/ESC/BACK 可能导致的
  Return、提交、导航、对话框关闭或其他应用副作用，不承诺应用状态只发生键盘变化。
- 当前 Appium 3.6.0/XCUITest Driver 12.1.0 的 common `hide_keyboard` route 仍存在
  但已 deprecated；若后续移除或拒绝该 route，只返回统一远端 unsupported/command
  error，不增加 `mobile:` 或平台内部 fallback。真实版本范围和设备结果写入
  `docs/compatibility.md`；当前能力矩阵状态为 `Implemented` / `Protocol`，真实
  版本与设备验证仍需单独记录。
- 排除自动输入恢复和 IME 管理。

已完成根包 Keyboard 实现和协议回归测试：`KeyboardShown` 与 `DismissKeyboard`
通过统一 HTTP 执行链发送精确的 Appium common routes，分别使用
`keyboard_shown` 与 `dismiss_keyboard` 作为 Error/Observer identity；GET 不带
请求体，POST 固定发送 `{}`。成功 value 严格限制为 JSON boolean，显式拒绝
`null`、数字、字符串、对象和数组，并在 Observer Finished 之前完成 decoder 校验。
实现不缓存、不重试、不 fallback、不管理 IME；真实 Driver、设备和 Host 组合仍未
验证，兼容性结果须单独写入 `docs/compatibility.md`。

### DP-110 应用放入后台

- 实现根包 `Session.BackgroundApp`，只放入当前 App 后台且不自动恢复。
- 使用 Appium 3 正式 `mobile: backgroundApp` Execute Method，通过固定的
  `POST /session/{sessionId}/execute/sync` 发送
  `{"script":"mobile: backgroundApp","args":[{"seconds":-1}]}`；不接受定时时长，
  也不以 `null` 表达无恢复。
- 固定 Error/Observer identity 为 `background_app`，成功 value 严格接受目标
  Execute Method 的 JSON `null`，其他类型按响应格式错误处理。
- 覆盖 XCUITest 与 UiAutomator2 请求/成功响应、Execute Script 请求体、Session ID
  路径转义、远端 unsupported、响应格式错误、请求前取消和 `DeliveryUnknown` 不重放。
- 恢复由现有 `ActivateApp` 显式执行。
- Execute Method 被 Driver 移除或拒绝时只返回统一远端错误，不回退到 deprecated
  HTTP compatibility route、Driver 内部端点或 Host 工具。
- 排除定时恢复、自动状态确认和通用 Back。

已完成根包 Background App 实现和协议回归测试。每次调用只通过统一 HTTP 执行链
发送一次固定负数 `seconds` 的 Appium `mobile: backgroundApp` Execute Method 请求；
客户端不读取或缓存 App 状态，不调度恢复、不重试也不 fallback。当前能力矩阵状态为
`Implemented` / `Protocol`；真实 Driver、设备与 Host 组合仍未验证，兼容性结果须
单独写入 `docs/compatibility.md`。

### DP-111 屏幕方向

- 实现根包 `Orientation` 强类型、`OrientationPortrait` /
  `OrientationLandscape` 常量以及 `Session.Orientation` /
  `Session.SetOrientation`。
- 使用 Appium 3 正式 Appium Device `GET/POST
  /session/{sessionId}/appium/device/orientation` 路由；GET 无请求体，POST
  固定发送 `{"orientation":"PORTRAIT"}` 或 `{"orientation":"LANDSCAPE"}`。
- 读取成功值严格限定为精确大写 JSON string `PORTRAIT` 或
  `LANDSCAPE`；设置成功值严格为 JSON `null`。非精确设置值在发送
  前返回 `CodeInvalidArgument` / `DeliveryNotSent`，不做大小写、空白或
  别名规范化。
- 固定 Error/Observer identity 为 `get_orientation` / `set_orientation`；响应
  decoder 在统一执行链中、Observer Finished 之前完成。
- 覆盖 XCUITest 与 UiAutomator2 的请求、成功响应、路径转义、无缓存
  读取、无效参数零请求、严格响应、远端失败、请求前取消和
  `DeliveryUnknown` 不重放。
- 不缓存或自动确认设置后状态，不探测 Driver/Context/Discovery，不执行
  坐标或截图方向换算；排除横屏左右、倒置竖屏选择与 `x/y/z`
  空间 Rotation API；不 fallback 到 deprecated
  `/session/{sessionId}/orientation`。

已完成根包 Orientation 实现和协议回归测试。实现以 Appium 3.7.0
（`@appium/base-driver` 10.8.0）的正式 Appium Device route 为协议基线；每次
读取都返回远端快照，
每次设置只发送一次有副作用命令；成功响应不被提升为后续屏幕状态
保证。当前能力矩阵状态为 `Implemented` / `Protocol`；真实 Appium、Driver、
设备和 Host 组合仍未验证，结果须单独写入 `docs/compatibility.md`。

### DP-120 活动 App ID

- 实现根包 `Session.ActiveAppID(ctx) (string, error)`，统一返回前台 App ID
  快照；XCUITest 结果是 iOS bundle ID，UiAutomator2 结果是 Android package，
  Android 明确没有 focused package 时返回空字符串和 nil error。
- 按创建 Session 后远端确认的精确 `automationName` 显式映射 XCUITest
  `mobile: activeAppInfo` 与 UiAutomator2 `mobile: getCurrentPackage`，都通过固定
  W3C Execute Script route 和 `args: []` 进入统一执行链。
- XCUITest 只读取 active app info 中精确的非空 string `bundleId`，忽略且不公开
  pid、name、processArguments 与未知字段；UiAutomator2 只接受非空 JSON string
  或表示无焦点的 `null`。标识字符串都严格校验 UTF-8 和 surrogate，不 trim
  或规范化；空 JSON string 与 Android `null` 明确区分。
- 固定 Error/Observer identity 为 `get_active_app_id`；响应 decoder 在 Observer
  Finished 之前完成。未知 Driver 在请求前返回 `CodeUnsupported` /
  `DeliveryNotSent`。
- 覆盖两 Driver 的请求 body、路径转义、动态快照、严格响应、未知 Driver 零请求、
  Observer、远端失败、请求前取消与 `DeliveryUnknown` 不重放。
- 不从请求或响应 Capability 的 app、bundleId、appPackage 或 browserName 猜测；
  不读取 Discovery/Context/AppState，不枚举进程或安装信息，不调用 Host 工具；
  不 fallback 到 deprecated Android `current_package` HTTP route、WDA/UiAutomator2
  Server 内部端点或其他脚本。

已完成根包 Active App ID 实现和协议回归测试。当前能力矩阵状态为
`Implemented` / `Protocol`；现有 Appium 3.6.0、XCUITest Driver 12.1.0 与
UiAutomator2 Driver 8.2.0 的源码观察只作为协议依据，真实 Driver、设备和 Host
组合仍未验证，结果须单独写入 `docs/compatibility.md`。

### DP-121 设备时间

- 实现根包 `Session.DeviceTime(ctx) (time.Time, error)`，使用 Appium common
  Device Time GET route 返回设备时间快照。成功 value 精确校验为
  `YYYY-MM-DDTHH:mm:ss±HH:MM`，并保留数字 UTC 偏移和秒精度。
- 固定 `get_device_time` Error/Observer identity；每次只读取一次且不缓存、
  校正、重试或后置确认。无效响应返回 `CodeResponseInvalid` 和零值，
  不猜测其他时间格式。
- 不静默回退 Host 时间，不按 `automationName` 做 common command 门禁，不调用
  Host 工具或内部 Driver route；记录 XCUITest Simulator、XCUITest 真机和
  Android Driver 取值路径差异，排除时间和时区设置能力。

已完成根包 Device Time 实现和协议回归测试。当前能力矩阵状态为
`Implemented` / `Protocol`；上游源码观察仅作为协议依据，真实 Driver、
设备和 Host 组合仍未验证，不标记为 `Verified`。

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
- 建立 `Find` / `FindElements` 的 command amplification 基线，至少记录
  candidate count、Rect probes、首个可见候选位置和总耗时；不预设硬性能阈值，
  也不因基线任务改变现有查找语义。
- 对大录屏执行一次真实 memory benchmark，区分 `StopRecordingTo` 的解码后输出
  峰值与完整 wire response 缓冲成本；没有独立配额需求或实测证据时不改变当前
  media limit 模型。
- 运行完整 `go test ./...`、`go test -race ./...` 和声明环境的 smoke suite。
- 未验证组合不作稳定承诺。
- 不为表面 API 完整加入未规划能力。
