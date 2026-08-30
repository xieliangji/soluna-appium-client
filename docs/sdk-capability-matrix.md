# soluna-appium-client SDK 能力矩阵

> 文档状态：Active  
> 适用阶段：v0.x 至首个稳定版本  
> 技术基线：Appium 3.x  
> 最后更新：2026-08-30

## 1. 文档目的

本文档是 `soluna-appium-client` 公共能力范围和实施状态的唯一维护入口，用于回答：

- SDK 当前已经公开了哪些能力；
- 哪些能力已经确定纳入，但尚未实现；
- 哪些能力需要先解决架构前置问题；
- 哪些能力被延期或明确排除；
- 能力属于根包、平台包还是基础设施；
- 调用方通过哪一种公共入口使用该能力；
- 能力依赖哪类协议、设备版本和 Host 条件；
- 当前证据是单元测试、协议测试，还是真实环境验证。

本文档不是 Appium、XCUITest 或 UiAutomator2 的完整命令清单，也不以复制上游全部 API 为目标。

## 2. 三层事实边界

项目同时维护三类不同事实：

| 事实层 | 载体 | 回答的问题 |
|---|---|---|
| SDK 能力 | 本文档 | 这个 Go SDK 是否实现或计划实现该能力 |
| Runtime Discovery | 活动 Session 的 Commands / Extensions Catalog | 当前 Appium、Driver 和 Plugin 登记了什么 |
| 兼容性验证 | `docs/compatibility.md` | 哪个 Host、Appium、Driver、WDA、设备 OS 组合已经真实跑通 |

三者不能互相替代：

- 本文档标记“已实现”，不代表当前远端 Session 一定支持；
- Runtime Discovery 返回某个命令，不代表当前设备环境一定可执行；
- 某个组合实测通过，不代表其他版本或 Host 自动获得兼容承诺。

## 3. 公共入口与状态定义

### 3.1 公共入口

“公共入口”描述调用方如何使用能力，不表示 SDK 存在多个 Client 类型。所有 Session、Element、平台函数和事件流最终都绑定根包中的 `appium.Client` / `appium.Session` 对象模型。

| 公共入口 | 含义 |
|---|---|
| `Client method` | 根包 `appium.Client` 方法 |
| `Session method` | 根包 `appium.Session` 方法；返回值上的本地查询 helper 仍归于该入口 |
| `Element method` | 根包 `appium.Element` 方法 |
| `Platform function` | `xcuitest` 或 `uiautomator2` 中接收根包 Session/Element 的无状态函数 |
| `Platform function / Session stream` | 平台函数控制远端能力，并通过同一 Session 绑定的事件流交付数据 |
| `wait helper` | `wait` 包中轮询根包公共 API 的辅助函数 |
| `Session stream` | 与根包 Session 绑定的事件订阅或流 |
| `Client option / callback` | Client 配置或命令观测回调 |
| `Internal / test infrastructure` | 不直接作为业务运行时公共入口 |
| `Documentation` | 项目治理文档，不是运行时 API |

SDK 不定义 `xcuitest.Client`、`uiautomator2.Client` 或独立公共 BiDi Client。

### 3.2 SDK 状态

| 状态 | 含义 |
|---|---|
| `Implemented` | 公共 API、实现和必要协议测试已经存在 |
| `Accepted` | 已确定进入 SDK 范围，接口或实现尚未完成 |
| `Architecture` | 已进入 SDK 范围，但必须先完成跨领域架构设计 |
| `Deferred` | 保留在长期范围内，但不属于当前稳定版本优先项 |
| `Excluded` | 已明确不进入公共 SDK，除非重新进行架构评审 |

### 3.3 验证状态

| 状态 | 含义 |
|---|---|
| `Unit` | 只有本地单元测试 |
| `Protocol` | 已通过 Fake Server/Contract Test 锁定协议 |
| `Verified` | 已在 `docs/compatibility.md` 中登记真实环境验证 |
| `None` | 尚无可引用验证证据 |

一个能力只有同时具有公共 API、实现和必要协议测试时，才能标记为 `Implemented`。真实设备跑通后，只更新“验证状态”和 `docs/compatibility.md`，不改变 SDK 状态。跨领域实现规则和设计决策维护在 `docs/design.md`。

## 4. 当前支持基线

- Go：1.26.5；
- Appium：3.x；
- XCUITest 主线设备：iOS 17+；
- XCUITest iOS 17.x：主要在 macOS Host 验证；
- XCUITest iOS 18+：在预安装/外部 WDA 与 RemoteXPC 条件满足时，纳入 macOS、Windows 和 Linux Host；
- iOS 17 以下：Legacy Lane，不定义主线 API 基线；
- UiAutomator2：目标 Android/Driver 组合以 `docs/compatibility.md` 的真实记录为准。

表中的“Host/版本约束”只记录能力本身的关键门槛。完整 Driver/WDA/Host 组合不得复制到本文档，应维护在 `docs/compatibility.md`。

## 5. 根包能力

### 5.1 Client、Session 与配置

| ID | 能力 / 目标 API | 公共入口 | 状态 | 机制 | Host/版本约束 | 验证 | 证据或下一步 |
|---|---|---|---|---|---|---|---|
| CORE-001 | `Client.Status` | Client method | Implemented | Appium HTTP Status | Appium 3 | Protocol | `status.go`, `status_test.go` |
| CORE-002 | 创建 Session | Client method | Implemented | W3C New Session | Appium 3；Driver 定义设备约束 | Protocol | `session.go`, `session_test.go` |
| CORE-003 | Session ID、Capabilities、AutomationName 快照 | Session method | Implemented | New Session response | Appium 3 | Protocol | `session.go`, `session_inspection_test.go` |
| CORE-004 | `Session.Healthy` | Session method | Implemented | `WindowRect` operational probe | Driver 必须支持 Window Rect | Protocol | `session.go`, `session_health_test.go` |
| CORE-005 | `Session.Close` 与创建失败清理 | Session method | Implemented | W3C Delete Session | Appium 3 | Protocol | `session.go`, `session_test.go` |
| CFG-001 | 设置 Script/PageLoad/Implicit Timeout | Session method | Implemented | W3C Timeouts | Driver 可能忽略不适用字段 | Protocol | `timeouts.go`, `timeouts_test.go` |
| CFG-002 | 读取当前 Timeouts | Session method (`CurrentTimeouts`) | Implemented | Appium 3 Get Timeouts (`command` / `implicit`) | 字段必须存在；值严格校验整数毫秒、非负和 `time.Duration` 溢出，拒绝 `null`；保留既有 `Timeouts` 设置类型 | Protocol | `timeouts.go`, `timeouts_test.go`, `docs/command-semantics.md` |
| CFG-003 | 读取 Session Settings | Session method (`Settings`) | Implemented | Appium `GET /session/{id}/appium/settings` | 返回值必须是 JSON object；键值由 Driver/Plugin 定义；每次远端读取并返回深拷贝 | Protocol | `settings.go`, `settings_test.go`, `docs/command-semantics.md` |
| CFG-004 | 增量更新 Session Settings | Session method (`UpdateSettings`) | Implemented | Appium `POST /session/{id}/appium/settings` | 只发送明确字段；不缓存、不维护白名单；nil Settings 在发送前拒绝 | Protocol | `settings.go`, `settings_test.go`, `docs/command-semantics.md` |
| DISC-001 | 读取 Command Catalog | Session method (`Commands`) | Implemented | Appium Runtime Discovery | Appium 3 | Protocol | `discovery.go`, `discovery_test.go`, `docs/command-semantics.md`；严格解码 `rest`/`bidi` 层级、必需 `base`/`driver`、可选 `plugins[name]` 与 HTTP/BiDi identity |
| DISC-002 | 读取 Extension Catalog | Session method (`Extensions`) | Implemented | Appium Runtime Discovery | Appium 3 | Protocol | `discovery.go`, `discovery_test.go`, `docs/command-semantics.md`；严格解码 `rest.driver`/`rest.plugins[name]` 与 Execute Method identity |
| DISC-003 | Catalog 本地 `Supports` helper | Session method | Implemented | 纯本地快照查询 | 不能视为执行成功保证 | Unit | `discovery.go`, `discovery_test.go`；按 HTTP、BiDi、Execute Method 分开的区分大小写精确匹配，Source 仅作 provenance |
| OBS-001 | 命令 Observer | Client option / callback | Implemented | Client-side lifecycle callbacks | 不等同远端 Logs | Protocol | `observer.go`, `observer_test.go` |

### 5.2 Element

| ID | 能力 / 目标 API | 公共入口 | 状态 | 机制 | Host/版本约束 | 验证 | 证据或下一步 |
|---|---|---|---|---|---|---|---|
| ELM-001 | `Session.Find` / `FindElements` | Session method | Implemented | W3C plural element lookup + Window intersection | Native Context 语义已定义 | Protocol | `element.go`, `element_find_test.go` |
| ELM-002 | Element 作用域 `Find` / `FindElements` | Element method | Implemented | W3C plural find from element + Window intersection | Native Context 语义已定义 | Protocol | `element.go`, `element_find_scope_test.go` |
| ELM-003 | `Element.Rect` / `Text` / `Attribute` | Element method | Implemented | W3C Element commands | Attribute 允许远端 `null` | Protocol | `element.go`, `element_test.go` |
| ELM-004 | `Element.Clear` / `SendKeys` | Element method | Implemented | W3C Element commands | Driver 输入法行为不同 | Protocol | `element.go`, `element_test.go` |
| ELM-005 | `Element.Tap` / `TapInWindowIntersection` | Element method | Implemented | Window/Element Rect + W3C Actions | 每次点击重新读取几何状态 | Protocol | `element.go`, `element_tap_test.go` |
| ELM-006 | `Element.Screenshot` | Element method | Accepted | W3C Element Screenshot | 不承诺自动滚动或与本地裁剪等价 | None | DP-051 实现；复用 `screenshot.go` 的资源与解码边界 |
| ELM-007 | `Element.ScreenshotTo(io.Writer)` | Element method | Accepted | 流式 Base64 解码 | 使用 Screenshot 专用上限 | None | DP-051 实现；复用 `screenshot.go` 的解码链 |
| ELM-008 | `Displayed` / `Enabled` / `Selected` | Element method | Excluded | Driver 状态查询 | 不能满足当前确定性语义要求 | None | 如未来重新引入需单独评审 |

### 5.3 视觉、页面与坐标

| ID | 能力 / 目标 API | 公共入口 | 状态 | 机制 | Host/版本约束 | 验证 | 证据或下一步 |
|---|---|---|---|---|---|---|---|
| VIS-001 | `Session.WindowRect` | Session method | Implemented | W3C Window Rect | WebDriver/Actions 坐标空间 | Protocol | `session.go`, `session_inspection_test.go` |
| VIS-002 | `Session.Screenshot` | Session method | Implemented | W3C Screenshot + shared streaming decoder | 使用 Screenshot 专用响应上限 | Protocol | `screenshot.go`, `session_inspection_test.go`, `screenshot_test.go` |
| VIS-003 | `Session.ScreenshotTo(io.Writer)` | Session method | Implemented | 流式 Base64 解码 | 返回已写入字节数；共享 Screenshot 上限 | Protocol | `screenshot.go`, `screenshot_test.go` |
| VIS-004 | Screenshot 专用资源上限 | Client option | Implemented | Client Limits | Session/Element Screenshot 共用；Element Screenshot 待 DP-051 | Protocol | `limits.go`, `client.go`, `screenshot_test.go` |
| VIS-005 | `Session.PageSource` | Session method | Implemented | W3C Page Source | 独立 Page Source 上限 | Protocol | `session.go`, `session_inspection_test.go` |
| VIS-006 | `Session.ViewportRect` / `PixelRect` | Session method | Accepted | Appium mobile viewport rect | 图像像素坐标，不参与 Element Tap | None | 先补 `coordinate-system.md` |
| VIS-007 | Viewport Screenshot | Session method | Deferred | Driver 截图后裁剪 | Driver/Host 图像依赖不同 | None | 先使用 Screenshot + ViewportRect |
| VIS-008 | 隐式坐标缩放或自动转换 | Internal / test infrastructure | Excluded | Client-side transform | 缺少完整方向/状态栏/Context 模型 | None | 只报告事实，不静默转换 |

### 5.4 Actions、Alert 与输入

| ID | 能力 / 目标 API | 公共入口 | 状态 | 机制 | Host/版本约束 | 验证 | 证据或下一步 |
|---|---|---|---|---|---|---|---|
| ACT-001 | `PerformActions` / `ReleaseActions` | Session method | Implemented | W3C Touch Pointer Actions | Driver 必须支持 touch pointer | Protocol | `actions.go`, `actions_test.go` |
| ACT-002 | `Tap` / `LongPress` / `Swipe` | Session method | Implemented | W3C Actions 封装 | viewport 坐标 | Protocol | `actions.go`, `actions_test.go` |
| ACT-003 | 多指 ActionSequence | Session method | Implemented | 多个 W3C pointer source | 具体手势兼容性需实机验证 | Protocol | `actions.go`, `actions_test.go` |
| ALERT-001 | Alert 文本 | Session method | Implemented | W3C Get Alert Text | `no such alert` 映射为 `CodeAlertNotFound`；成功值为 JSON string 或 `null`，通过 `hasText` 区分 | Protocol | `alerts.go`, `alerts_test.go`, `docs/command-semantics.md` |
| ALERT-002 | Accept / Dismiss Alert | Session method | Implemented | W3C Alert commands | 不增加 `HasAlert` TOCTOU API；POST 请求体为 JSON `{}`；成功值严格为 JSON `null` | Protocol | `alerts.go`, `alerts_test.go`, `docs/command-semantics.md` |
| ALERT-003 | Set Alert Text | Session method | Implemented | W3C Set Alert Text | 仅含输入框 Alert 有效；POST 请求体含 `text`；成功值严格为 JSON `null` | Protocol | `alerts.go`, `alerts_test.go`, `docs/command-semantics.md` |
| KBD-001 | Keyboard Shown | Session method | Accepted | Appium common command | Driver 探测实现不同 | None | 新增 `keyboard.go` |
| KBD-002 | Dismiss Keyboard | Session method | Accepted | Appium common command | 需定义“尝试”与最终状态语义 | None | 先验证两 Driver 行为 |

### 5.5 Context、导航与设备状态

| ID | 能力 / 目标 API | 公共入口 | 状态 | 机制 | Host/版本约束 | 验证 | 证据或下一步 |
|---|---|---|---|---|---|---|---|
| CTX-001 | Context 列表、当前 Context、切换 Context | Session method | Architecture | Appium Context commands | 先定义 Web Context 的 Find/Rect 几何语义 | None | 独立 Context 里程碑 |
| NAV-001 | 将当前 App 放入后台且不自动恢复 | Session method | Accepted | Appium background command | 恢复由 `ActivateApp` 显式执行 | None | 新增 `navigation.go` |
| NAV-002 | 屏幕方向读取与设置 | Session method | Accepted | Appium Orientation | Portrait/Landscape 强类型 | None | 新增 `orientation.go` |
| NAV-003 | Deep Link | Session method | Accepted | Driver execute method / navigation | iOS 与 Android 参数和最低版本不同 | None | 根包按 AutomationName 映射 |
| NAV-004 | 通用 `Back` | Session method | Excluded | Driver-specific navigation | Android 物理 Back 与 iOS 启发式导航不等价 | None | 平台包可单独评审 |
| DEV-001 | 活动 App ID | Session method | Accepted | Driver active app/package info | iOS bundle ID / Android package | None | 定义统一只读结果 |
| DEV-002 | 设备时间 | Session method | Accepted | Appium common command | Driver/Host 获取路径不同 | None | 返回可校验时间事实，不猜格式 |

### 5.6 应用控制、录屏与脚本

| ID | 能力 / 目标 API | 公共入口 | 状态 | 机制 | Host/版本约束 | 验证 | 证据或下一步 |
|---|---|---|---|---|---|---|---|
| APP-001 | `ActivateApp` | Session method | Implemented | Appium common command | app ID 为 package/bundle ID | Protocol | `application.go`, `application_test.go` |
| APP-002 | `TerminateApp` | Session method | Implemented | Appium common command | Driver 终止语义不同但结果统一为 error | Protocol | `application.go`, `application_test.go` |
| APP-003 | `AppState` | Session method | Implemented | Appium Query App State | 严格校验 0–4 | Protocol | `application.go`, `application_test.go` |
| REC-001 | `StartRecording` | Session method | Implemented | Appium Screen Recording | Driver/Host 可能依赖外部工具 | Protocol | `recording.go`, `recording_test.go` |
| REC-002 | `StopRecording` / `StopRecordingTo` | Session method | Implemented | Base64 media response | 独立 Recording 上限 | Protocol | `recording.go`, `recording_test.go` |
| EXEC-001 | `ExecuteScript` | Session method | Implemented | W3C Execute Script | `mobile:` 扩展逃生口 | Protocol | `execute.go`, `execute_test.go` |

### 5.7 Logs、Events 与等待

| ID | 能力 / 目标 API | 公共入口 | 状态 | 机制 | Host/版本约束 | 验证 | 证据或下一步 |
|---|---|---|---|---|---|---|---|
| LOG-001 | 查询 Log Types | Session method | Accepted | Appium `/log/types` | 类型由 Driver/Capability 决定 | None | 新增 `logs.go` |
| LOG-002 | 按类型 Pull Logs | Session method | Accepted | Appium `/log` | 读取是否清空由 Driver 语义决定 | None | 增加 `MaxLogResponseBytes` |
| BIDI-001 | 通用 WebDriver BiDi 连接与命令关联 | Session stream | Architecture | WebSocket / BiDi | Appium/Driver 必须返回可用 Endpoint | None | 先设计 `internal/bidi` |
| BIDI-002 | 有界订阅、取消、背压和关闭 | Session stream | Architecture | BiDi Event Stream | 不自动重连 | None | 增加协议测试基础设施 |
| LOG-003 | 通用 Streaming Logs | Session stream | Architecture | WebDriver BiDi | 依赖 BIDI-001/002 | None | 与平台日志事件分层 |
| WAIT-001 | `wait.Until` | wait helper | Accepted | Client-side polling | 总期限由 context 控制 | None | `wait/wait.go` 当前为空 |
| WAIT-002 | `wait.Element` / `wait.Elements` | wait helper | Accepted | 重复调用公共 Find | 只重试声明为暂态的结果 | None | `wait/element.go` 当前为空 |

## 6. XCUITest 平台能力

XCUITest 能力必须同时说明最低 iOS、Driver/WDA、真机/模拟器和 Appium Host OS。详细版本组合维护在 `docs/compatibility.md`。

| ID | 能力 / 目标 API | 公共入口 | 状态 | 机制 | Host/版本约束 | 验证 | 证据或下一步 |
|---|---|---|---|---|---|---|---|
| XCUI-001 | `IOSPressButton` | Platform function | Implemented | `mobile: pressButton` / WDA | iOS；部分按键受设备和系统限制 | Protocol | `xcuitest/device.go`, `device_test.go` |
| XCUI-002 | `IOSDeviceScreenInfo` | Platform function | Implemented | `mobile: deviceScreenInfo` / WDA | 报告 scale 与 status bar，不自动换算 | Protocol | `xcuitest/device.go`, `device_test.go` |
| XCUI-003 | Picker Wheel 选择 | Platform function | Accepted | `mobile: selectPickerWheelValue` | iOS/XCUITest；通用 Actions 无稳定等价 | None | 优先平台候选 |
| XCUI-004 | 按 label 处理 Alert | Platform function | Accepted | XCUITest Alert extension | 依赖通用 Alert API 先完成 | None | 只暴露标准 Alert 无法表达的增量 |
| XCUI-005 | Simulated Location | Platform function | Accepted | WDA simulated location | iOS 16.4+；iOS 17 主要 macOS | None | 单独定义 Set/Get/Reset |
| XCUI-006 | System Monitor | Platform function / Session stream | Architecture | RemoteXPC DVT + BiDi | iOS 18+ 真机；跨 Host 候选 | None | 依赖 BIDI-001/002 |
| XCUI-007 | Network Monitor | Platform function / Session stream | Architecture | RemoteXPC DVT + BiDi | iOS 18+ 真机；跨 Host 候选 | None | 依赖 BIDI-001/002 |
| XCUI-008 | Syslog / Crashlog Typed Stream | Session stream | Architecture | RemoteXPC/BiDi | 具体类型和消费语义需验证 | None | 依赖 BIDI-001/002 |
| XCUI-009 | Battery Info | Platform function | Excluded | XCUITest execute method | UI 自动化价值不足 | None | 保留 `ExecuteScript` 逃生口 |
| XCUI-010 | `startPerfRecord` / `xctrace` | Platform function | Excluded | macOS Xcode tools | macOS-only | None | 不进入跨 Host 核心 SDK |
| XCUI-011 | Simulator-only API 集合 | Platform function | Excluded | simctl / Simulator framework | macOS Simulator only | None | 当前项目以真机为主 |

## 7. UiAutomator2 平台能力

`uiautomator2` 目录当前只有占位文件，没有公开平台 API。平台能力应在完成通用能力后重新评审，不因为上游存在命令就批量复制。

| ID | 能力 / 目标 API | 公共入口 | 状态 | 机制 | Host/版本约束 | 验证 | 证据或下一步 |
|---|---|---|---|---|---|---|---|
| UIA-001 | UiAutomator2 Driver 门禁 | Internal / test infrastructure | Accepted | Session AutomationName | Host 通常可跨平台 | None | 与 XCUITest 门禁保持同一错误语义 |
| UIA-002 | Android 物理键 | Platform function | Deferred | `mobile: pressKey` | Android-specific | None | 仅在真实调用场景确认后纳入 |
| UIA-003 | Android 专有滚动/手势 | Platform function | Deferred | UiAutomator2 execute methods | W3C Actions 无稳定替代时才纳入 | None | 不批量复制 Gesture API |
| UIA-004 | Android 系统面板/通知 | Platform function | Deferred | UiAutomator2 execute methods | Android-specific | None | 需明确高价值场景 |

## 8. 基础设施能力

| ID | 能力 / 目标 API | 公共入口 | 状态 | 机制 | Host/版本约束 | 验证 | 证据或下一步 |
|---|---|---|---|---|---|---|---|
| INF-001 | 结构化 Error Code | Internal / test infrastructure | Implemented | Client error mapping | HTTP 路径已覆盖 | Protocol | `errors.go`, `command_error_test.go` |
| INF-002 | Delivery State | Internal / test infrastructure | Implemented | wire response facts | 副作用命令不自动重试 | Protocol | `errors.go`, `command_delivery_test.go` |
| INF-003 | 响应和远端错误大小限制 | Client option / callback | Implemented | `Limits` | Logs/BiDi 仍待扩展 | Protocol | `limits.go`, transport/codec tests |
| INF-004 | Base64 流式解码 | Internal / test infrastructure | Implemented | `internal/codec` | 已用于 Recording 和 Screenshot | Unit/Protocol | `internal/codec`, `recording.go`, `screenshot.go` |
| INF-005 | HTTP Contract Test 工具 | Internal / test infrastructure | Implemented | `contracttest` | Test-only | Protocol | `contracttest/` |
| INF-006 | BiDi Contract Test 工具 | Internal / test infrastructure | Architecture | Fake WebSocket/BiDi Server | Test-only | None | 与 BIDI-001 同步设计 |
| INF-007 | SDK 能力矩阵维护 | Documentation | Implemented | 本文档 | 每个公共能力变更必须更新 | None | `docs/sdk-capability-matrix.md` |
| INF-008 | 真实兼容性矩阵 | Documentation | Accepted | `docs/compatibility.md` | 当前文件待填充 | None | 按真实设备组合登记 |

## 9. 显式排除项

以下能力不因上游 Driver 已提供就自动进入 SDK：

| 能力 | 排除原因 |
|---|---|
| 任意 Raw HTTP Method/Route | 绕过统一错误、限制、Observer 和兼容边界 |
| 自动 Session 恢复 | 需要业务页面和登录状态知识 |
| 自动重试副作用命令 | 可能造成重复点击、输入或脚本执行 |
| 自动 stale element 重定位 | Element 不保存 Locator，且客户端不知道业务语义 |
| 通用 `Back` | iOS 与 Android 行为不等价 |
| `Displayed` / `Enabled` / `Selected` | 当前无法提供满足项目要求的确定性语义 |
| XCUITest Battery Info | 与 UI 执行主链价值弱 |
| XCUITest Instruments Perf Record | 依赖 macOS/Xcode，不满足跨 Host 主线 |
| Simulator-only 平台命令 | 当前真机优先，且不能跨 Host |
| 完整复制所有 Driver Execute Methods | 扩大维护面但不能提高核心执行可靠性 |

`ExecuteScript` 仍可用于调用方主动访问未封装扩展，但这不构成 SDK 对该扩展的兼容承诺。

## 10. 架构前置关系

```text
Screenshot 专用资源模型
    ├── Session.ScreenshotTo
    └── Element.Screenshot / ScreenshotTo

Web Context 几何模型
    └── Contexts / CurrentContext / SwitchContext

BiDi 基础设施
    ├── Streaming Logs
    ├── XCUITest System Monitor
    ├── XCUITest Network Monitor
    └── XCUITest Syslog / Crashlog Stream

通用 Alert API
    └── XCUITest 按 label 处理 Alert

Runtime Discovery Catalog 类型
    └── Catalog Supports helper
```

具有架构前置关系的能力不能通过临时 helper 绕开前置设计。

## 11. 当前建议实施顺序

1. Element 作用域 `Find` / `FindElements`；
2. 标准 Alert；
3. Session Settings；
4. Runtime Command / Extension Discovery；
5. Screenshot 专用上限、`ScreenshotTo` 和 Element Screenshot；
6. Viewport Rect 与坐标文档；
7. `wait` package；
8. Pull Logs；
9. Context 与 Web 几何模型；
10. Keyboard、Background、Orientation、Active App、Device Time、Deep Link；
11. BiDi 基础设施；
12. Streaming Logs、System Monitor 和 Network Monitor；
13. 重新评审 UiAutomator2 高价值平台能力。

实施顺序可以因真实项目需求调整，但状态变化必须同步更新本文档。

## 12. 维护规则

### 12.1 新增能力

新增或接受一个能力时：

1. 分配稳定 ID；
2. 判断属于根包、XCUITest、UiAutomator2 还是基础设施；
3. 标明调用方使用的公共入口；
4. 标记 SDK 状态；
5. 写明协议机制；
6. 写明最低设备 OS、Driver/WDA、真机/模拟器和 Host 限制；
7. 写明资源边界；
8. 写明测试证据或下一步；
9. 如改变高层结构，同步更新 `docs/architecture.md`；如改变跨领域实现规则或设计决策，同步更新 `docs/design.md`。

### 12.2 实现能力

状态从 `Accepted` 或 `Architecture` 改为 `Implemented` 时，必须同时存在：

- 公共 API；
- 具体实现；
- 参数和响应校验；
- 错误和 Delivery 语义；
- 必要资源限制；
- 单元或协议测试；
- GoDoc；
- 本文档中的代码与测试证据。

### 12.3 真实环境验证

真实环境跑通时：

1. 在 `docs/compatibility.md` 登记 Host OS、Appium、Driver、WDA、设备 OS、设备类型和连接方式；
2. 将本表验证状态改为 `Verified`；
3. 不把单一组合的结果描述成所有版本普遍支持；
4. 发现差异时补协议或回归测试。

### 12.4 上游变化

Appium、Driver 或 WDA 升级时，应检查：

- Endpoint 或 Execute Method 是否变化；
- 参数和返回值是否变化；
- 最低设备 OS 或 Host 条件是否变化；
- 命令是否被废弃；
- Runtime Discovery 返回结构是否变化；
- 日志读取或 BiDi 事件语义是否变化；
- 当前 `Implemented` 能力是否仍有有效测试证据。

### 12.5 文档审查

以下变更必须把本文档纳入同一 Pull Request 或 Commit：

- 新增、删除或重命名公共方法；
- 能力从根包移动到平台包，或反向移动；
- 公共入口类型变化；
- 能力状态变化；
- 兼容范围变化；
- 新增大型响应或事件流；
- 引入新的 Host OS 依赖；
- 明确排除此前计划的能力。
