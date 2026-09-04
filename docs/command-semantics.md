# 命令语义

本文档记录进入 SDK 的公共命令请求、响应、副作用和失败语义。

## 标准 Alert

所有命令均复用根包 `Client` 的统一 HTTP 执行链。路径中的 Session ID
按 Endpoint 规则转义；命令不会自动等待、重试或探测 Alert 是否存在。

| API | HTTP | 路径 | 请求体 | 成功值 |
|---|---|---|---|---|
| `Session.AlertText` | GET | `/session/{sessionId}/alert/text` | 无 | JSON string 或 `null` |
| `Session.AcceptAlert` | POST | `/session/{sessionId}/alert/accept` | JSON object `{}` | `null` |
| `Session.DismissAlert` | POST | `/session/{sessionId}/alert/dismiss` | JSON object `{}` | `null` |
| `Session.SetAlertText` | POST | `/session/{sessionId}/alert/text` | `{"text":"..."}` | `null` |

请求体存在时发送 `Content-Type: application/json`。Accept/Dismiss 虽无参数，
仍发送 `{}`；空请求体不属于本 SDK 的标准 Alert 请求契约。

`AlertText` 返回 `(text string, hasText bool, error)`：

- JSON string（包括空字符串）返回 `hasText=true`；
- JSON `null` 返回 `text=""`、`hasText=false`；
- 其他 JSON 类型或非法 JSON 返回 `CodeResponseInvalid`。

成功响应必须是 W3C envelope，命令级 value 按上表严格解码。
远端 `no such alert` 是已确认收到响应的命令失败，Delivery 为
`DeliveryAcknowledged`，并映射为 `CodeAlertNotFound`。

## Session Timeouts

`Session.Timeouts` 每次读取 Appium 3 Get Timeouts 命令，并返回独立的
`CurrentTimeouts` 结果类型：

```text
GET /session/{sessionId}/timeouts
```

请求不带 body。成功值必须是同时包含 `command` 和 `implicit` 两个字段的
JSON object；字段值必须是非负整数毫秒。零是有效值，字段缺失或显式 `null`
均属于响应格式错误。整数值超出 `time.Duration` 可表示范围时同样返回
`CodeResponseInvalid`。读取结果不在 Session 本地缓存。SetTimeout 请求中的
`script`、`pageLoad`、`implicit` 字段不承诺会从该读取命令中原样返回。
用于设置超时的既有 `Timeouts` 类型仍保留 `Script`、`PageLoad` 和 `Implicit`
字段；读取结果的 `Command`、`Implicit` 不会改变该公共类型的语义。

## Session Settings

`Session.Settings` 和 `Session.UpdateSettings` 使用 Appium Session Settings
命令：

| API | HTTP | 路径 | 请求体 | 成功值 |
|---|---|---|---|---|
| `Session.Settings` | GET | `/session/{sessionId}/appium/settings` | 无 | JSON object |
| `Session.UpdateSettings` | POST | `/session/{sessionId}/appium/settings` | `{"settings":<增量字段对象>}` | `null` |

`Settings` 是开放的 `map[string]any`；键和值由 Driver 或 Plugin 定义。
客户端不维护白名单、不自动规范化值，也不把更新后的状态写入本地缓存。
GET 每次读取远端并返回独立的深拷贝。UpdateSettings 的外层请求体始终包含
`settings` 字段；空的非 nil `Settings` 会发送 `{"settings":{}}`。nil Settings
不是 JSON object，在请求发送前返回 `CodeInvalidArgument`。

GET 的成功 value 必须是 JSON object（包括空对象）；其他类型或非法 JSON
返回 `CodeResponseInvalid`，Delivery 为 `DeliveryAcknowledged`。

## Session Context（DP-091 已实现）

Context API 只使用根包 `Session`，并通过 Appium 3 当前注册的 Context 路由读取或
切换当前远端 Context：

| API | HTTP | 路径 | 请求体 | 成功 value |
|---|---|---|---|---|
| `Session.Contexts` | GET | `/session/{sessionId}/contexts` | 无 | JSON string 数组，解码为 `[]string` |
| `Session.CurrentContext` | GET | `/session/{sessionId}/context` | 无 | JSON string |
| `Session.SwitchContext` | POST | `/session/{sessionId}/context` | `{"name":"<context>"}` | `null` |

GET 请求不带 body，也不发送 `Content-Type`；POST 始终发送只含 `name` 字段的
JSON object。Session ID 按统一 Endpoint 规则作为独立路径段转义。每次方法调用
只发送一次对应请求，不隐式先读取 Context 列表、Discovery、Healthy 或
Window Rect；Context 命令使用普通命令响应上限，不新增 Context 专用资源配额，
也不自动重试、回退或恢复页面状态。

DP-091 实现只以上述 `/session/{sessionId}/context` 与
`/session/{sessionId}/contexts` 作为请求目标，不改写或自动 fallback 到替代路由。

`Contexts` 的成功 value 必须是 JSON string array。数组顺序和重复项按远端保留，
空数组返回非 nil 空 slice；`null`、object、字符串或数组中的非 string 项均为
`CodeResponseInvalid`。所有 JSON string 都必须能严格解码为有效 UTF-8（包括
surrogate 校验）。`CurrentContext` 的成功 value 必须是 JSON string（空字符串也
按远端字符串事实保留），不能是 `null` 或其他 JSON 类型。
`SwitchContext` 的参数按调用方提供的确切 UTF-8 字符串编码，非法 UTF-8 在发送前
返回 `CodeInvalidArgument`/`DeliveryNotSent`；空字符串、大小写差异和未知名称不
在 SDK 内预先拒绝，由远端决定是否存在或可切换。成功 value 必须严格为 JSON
`null`。

三条命令的成功响应都必须先通过统一 W3C/Appium envelope 解码，再按上述 value
契约解码；收到 envelope 后的任何格式错误均为 `DeliveryAcknowledged`，不返回
部分 Context 快照。

Context 名称的本地几何分类区分大小写：精确 `NATIVE_APP` 为 Native，精确
`WEBVIEW`、带非空后缀的 `WEBVIEW_` 或精确 `CHROMIUM` 为 Web，其他名称为
Unknown。`CHROMIUM` 是 UiAutomator2 纯浏览器会话使用的固定 Context 名称；该
分类只选择 `docs/coordinate-system.md` 与 `docs/design.md` 已定义的几何路径，不是
Runtime Discovery 或能力成功保证。Context-sensitive Element Find/Tap 先使用一次
当前 Context 快照来选择路径，再发送候选查找或几何命令；客户端不缓存 current Context，不从列表顺序、
Capability、页面源或 Host 工具推测，也不在 Unknown Context 下隐式使用另一种
路径。Unknown Context 在该快照成功后即为主体 Find/FindElements、Element
Find/FindElements 或 Element Tap 操作返回 `CodeUnsupported` /
`DeliveryNotSent`，不发送候选查找、几何探针或 Actions。这里的 Delivery 只描述
主体操作；成功的 `CurrentContext` 探针仍由它自己的 Observer 事件记录。
Native/Web 的 Context-sensitive Find/Tap 都会为这次策略选择增加该 `CurrentContext`
请求；直接的 Element Rect 命令不因 Context 类型而增加该快照。Context 快照与
后续元素查找、Rect、viewport 探针和 Actions 不是原子事务；
竞争或页面滚动时沿用各底层命令的真实结果。

Web Context 的 Element Rect 使用 CSS pixel；按 WebDriver 契约，`x`/`y` 是文档
元素坐标，先减去同一次 viewport 探针取得的 `scrollX`/`scrollY`，再与由
`window.innerWidth`/`window.innerHeight` 形成的 layout viewport 做正面积交集；
不使用 Native `WindowRect`、`ViewportRect`、devicePixelRatio 或 Screenshot 像素。
Web 元素点击在平移后的交集内选择整数 CSS viewport 坐标并复用既有 W3C Actions；
不自动滚动、Element Click、JavaScript click、stale 重定位或 Context fallback。
Native Find/Tap 的既有 Window Rect 交集和 Actions 语义保持不变。

`SwitchContext` 收到成功响应只表示该次远端命令成功，不建立可长期信任的本地
current-context 状态；失败或 `DeliveryUnknown` 时不回滚、不重放，也不猜测远端
是否已经切换。已有 Element 句柄不因切换被本地批量失效或重新绑定，后续错误由
远端按 stale、no such element 或其他命令事实返回。

Web 几何使用一个内部、固定脚本的 viewport 探针，不新增公开的 Web viewport
方法或 Raw Command：

```text
POST /session/{sessionId}/execute/sync
script: "return {scrollX: window.scrollX, scrollY: window.scrollY, width: window.innerWidth, height: window.innerHeight}"
args:   []
operation: get_web_viewport
```

该 Execute Script 请求必须进入根包统一执行链；成功 value 必须是包含有限的
`scrollX`/`scrollY`、有限且为正的 `width`/`height` 的对象，且
`scrollX + width`、`scrollY + height` 仍须是有限值。`scrollX`/`scrollY` 是
文档坐标系中当前 layout viewport 左上角的 CSS pixel 偏移，不因暂时的负值、
RTL 或 Driver 的滚动实现而被客户端 clamp。该对象只在已识别 Web Context 的
Context-sensitive Find/Tap 中按需读取一次；客户端据此构造原点为零的 viewport
Rect；平移任一候选 Rect 后若坐标或端点不再是有限值，整个操作同样返回响应
格式错误，不返回部分结果。不缓存，不读取 `ViewportRect`、Window Rect 或 Host
工具。固定脚本返回的未知字段不参与几何计算。响应格式错误按
`CodeResponseInvalid`/`DeliveryAcknowledged` 处理；脚本、传输或 Context 竞争
错误沿用统一命令语义，且不返回部分元素或发送后续动作。

## Session Keyboard（DP-101 已实现）

Keyboard 能力只面向当前 Driver 的软键盘状态快照和一次关闭请求。公共入口属于
根包 `Session`：

| API | HTTP | 路径 | 请求体 | 成功值 |
|---|---|---|---|---|
| `Session.KeyboardShown` | GET | `/session/{sessionId}/appium/device/is_keyboard_shown` | 无 | JSON boolean（Driver 状态快照） |
| `Session.DismissKeyboard` | POST | `/session/{sessionId}/appium/device/hide_keyboard` | JSON object `{}` | JSON boolean（原始 Driver-reported 结果；非最终状态） |

GET 不带 body，也不发送 `Content-Type`；POST 始终发送空 JSON object 并设置
`Content-Type: application/json`。Session ID 按统一 Endpoint 规则作为独立路径段
处理。两条路径都是 Appium 3 common routes，不改写为 `mobile:` Execute Method，
也不调用 WDA、UiAutomator2 Server 或 Host 工具的内部路径。

两个命令的 `Error.Operation` 以及 Observer `Started`/`Finished` identity 固定为
`keyboard_shown` 和 `dismiss_keyboard`，分别对应状态读取和关闭请求；它们不使用
wire route 名称，也不随 Driver 或版本改变。

每次调用只发送一次对应请求，不隐式先读 `KeyboardShown`、Context、Discovery 或
Healthy，也不执行后置确认、等待、轮询、自动重试、fallback、IME 配置或 Context
切换。`KeyboardShown` 每次返回远端当前探测结果，不缓存；`false` 仅是该时刻
Driver 报告未显示，不能推断屏幕像素或后续命令状态。

`DismissKeyboard` 的公共签名为 `(bool, error)`。第一个返回值是成功响应中的原始
Driver-reported boolean；`true` 和 `false` 都表示收到了成功响应，不能解释为跨
Driver 的最终关闭事实，也不把 `false` 转换为命令失败。若 `error` 非 nil，该布尔
值为零值且不可用于推断状态。需要确认关闭时，调用方应在关闭请求成功后显式再次调用
`KeyboardShown`；必要的有界重复读取由调用方自行调度。SDK 不接受 `strategy`、
`key`、`keyCode` 或 `keyName` 参数，不发送特殊键或本地点击/滑动等替代动作。

Driver 为关闭请求选择的内部动作可能改变应用状态：例如 WDA 的 Done 或 Android
Driver 的 ESC/BACK 可能触发 Return/提交、导航、对话框关闭或其他应用逻辑。该命令
只承诺发起一次 Driver 请求并交付其响应，不承诺应用状态只发生键盘变化；键盘状态
竞争时也不提供回滚或副作用隔离。

Keyboard 实现的响应 decoder 在统一 `executeCommand` 的 decoder slot 中运行，并在
`Observer.OnCommandFinished` 之前完成。`true`、`false` 是唯一合法成功 token；必须
显式区分 `null`、数字、字符串、对象和数组（使用 `json.RawMessage` 类型检查，或
带 nil 检查的 `*bool`，不能直接把响应解码到 `bool` 以免 `null` 被当作 false）。
协议回归测试还必须覆盖 GET 不发送 `Content-Type`、POST 固定 `{}`、Session ID
独立路径段转义，以及 `DeliveryUnknown` 时不重放关闭请求。

两 Driver 的状态探测和关闭实现不同：XCUITest 常通过 `XCUIElementTypeKeyboard`
查询并将 WDA dismiss 结果映射到 common command；UiAutomator2 常读取 Android
输入法服务状态，并可能在 Driver 内部使用平台按键和等待。上述内部差异不改变
本 SDK 的路由和返回契约，真实版本组合仍需兼容性验证。

## Session Background App（DP-110 已实现）

`Session.BackgroundApp` 只请求把当前 App 放入后台，不接受 App ID 或定时时长，使用
Appium 3 正式的 `mobile: backgroundApp` Execute Method：

| API | HTTP | 路径 | 请求体 | 成功 value |
|---|---|---|---|---|
| `Session.BackgroundApp` | POST | `/session/{sessionId}/execute/sync` | `{"script":"mobile: backgroundApp","args":[{"seconds":-1}]}` | JSON `null` |

请求始终使用固定的 W3C Execute Script 路由并设置 `Content-Type: application/json`；
Session ID 按统一 Endpoint 规则作为独立路径段转义。`args` 是包含一个参数 object
的 JSON array，`seconds=-1` 使用 XCUITest 与 Android Driver 都定义的“不恢复”分支。
特别是不发送 `null`：Android Driver 会将其转换为零秒等待并重新激活 App，不满足
本能力边界。

每次调用只发送一次 Execute Method 请求，固定 Error/Observer identity 为
`background_app`。成功 value 严格为 JSON `null`，由统一
`ExecuteScriptWithOperationAndDecode` decoder 在 Observer Finished 之前校验；
`true`、`false`、数字、字符串、object 或 array 都返回 `CodeResponseInvalid` /
`DeliveryAcknowledged`。成功只表示 Execute Method 请求得到成功响应，不被 SDK
提升为后台最终状态断言。

客户端不预读或后读 `AppState`、Context、Discovery 或 Healthy，不缓存前后台状态，
也不自动调用 `ActivateApp`。成功响应与设备实际状态之间仍可能发生竞争；需要恢复
时调用方显式执行 `ActivateApp`，需要确认时显式读取 `AppState`。

Execute Method 不可用、被拒绝或远端返回 unsupported 时沿用统一远端错误，不自动
切换到 deprecated `/appium/app/background` compatibility route、WDA、UiAutomator2
Server、Host 工具或通用 Back。传输结果为 `DeliveryUnknown` 时不重放请求，也不推测
App 是否已经进入后台；不提供定时恢复、后台计时器或状态恢复任务。

## Session Orientation（DP-111 已实现）

Orientation 使用根包 `Session` 和 Appium 3 正式 Appium Device route：

| API | HTTP | 路径 | 请求体 | 成功 value |
|---|---|---|---|---|
| `Session.Orientation` | GET | `/session/{sessionId}/appium/device/orientation` | 无 | JSON string `"PORTRAIT"` 或 `"LANDSCAPE"` |
| `Session.SetOrientation` | POST | `/session/{sessionId}/appium/device/orientation` | `{"orientation":"PORTRAIT"}` 或 `{"orientation":"LANDSCAPE"}` | JSON `null` |

该正式 Appium Device route 以 Appium 3.7.0（`@appium/base-driver` 10.8.0）为
协议基线。Appium 3.6.0 及更早版本的裸 `/session/{sessionId}/orientation`
属于 legacy JSONWP route；SDK 不使用该入口，也不在正式 route 失败后自动降级。

GET 不带 body，也不发送 `Content-Type`；POST 发送只包含 `orientation` 的
JSON object 并设置 `Content-Type: application/json`。Session ID 按统一 Endpoint
规则作为独立路径段转义。两条命令的 Error/Observer identity 分别固定为
`get_orientation` 和 `set_orientation`。

`Orientation` 是 string 底层类型的有限公共枚举，只公开
`OrientationPortrait` (`PORTRAIT`) 和 `OrientationLandscape` (`LANDSCAPE`)。
读取值必须是上述两个精确大写 JSON string；`null`、其他 JSON 类型、
空字符串、小写、带空白、未知值、非法 UTF-8 或未配对 surrogate 均返回
`CodeResponseInvalid` / `DeliveryAcknowledged`，不交付部分或未知枚举值。

`SetOrientation` 的参数必须精确等于两个公共常量之一。零值、小写、
空白、别名、非法 UTF-8 或任意自行构造值在 JSON 编码和远程请求前返回
`CodeInvalidArgument` / `DeliveryNotSent`；客户端不做大小写、空白或
协议别名规范化。设置成功值严格为 JSON `null`，其他类型均为
`CodeResponseInvalid` / `DeliveryAcknowledged`。

每次 `Orientation` 都发送一次读取并返回远端快照；每次合法
`SetOrientation` 都发送一次设置，即使参数与当前方向相同。客户端不缓存、
预读、后置确认、等待、重试或串行化 Session 命令。设置成功不保证 App
接受某个物理旋转方向，也不保证随后读取仍为该值；需要时由调用方显式
再次读取。

命令不本地门禁 XCUITest 或 UiAutomator2，不读取 Runtime Discovery、Context、
Capabilities 或几何状态。远端 unsupported 沿用统一命令错误，不改走 WDA、
UiAutomator2 Server、Host 工具、Execute Method、deprecated
`/session/{sessionId}/orientation` 或 `/rotation` fallback。
`DeliveryUnknown` 时不重放设置请求或推测远程状态。本公共类型不区分
landscape left/right 和 portrait upside-down，也不表示 `/rotation` 的 `x/y/z`
空间旋转。Orientation 不自动转换 Rect、ViewportRect、Actions 或 Screenshot。

## Session Pull Logs（DP-081 已实现）

Pull Logs 只提供一次性的 Session 级批量读取。通过根包统一 HTTP 执行链实现以下
Appium 3 路由；`/se/` 是标准路由的一部分，不使用历史或
Driver 专用的 `/log` fallback：

| API | HTTP | 路径 | 请求体 | 成功值 |
|---|---|---|---|---|
| `Session.LogTypes` | GET | `/session/{sessionId}/se/log/types` | 无 | JSON string 数组，解码为 `[]LogType` |
| `Session.Logs` | POST | `/session/{sessionId}/se/log` | `{"type":"<LogType>"}` | JSON Entry 数组，解码为 `[]LogEntry` |

GET 不带 body，也不发送 `Content-Type`；POST 始终发送 JSON object。Session ID
按 Endpoint 规则作为独立路径段转义。通过本地校验后，每次调用只发送一次对应
请求，不隐式查询 Log Types、Discovery、Healthy 或其他命令。`LogTypes` 的数组
元素必须都是 JSON string（空字符串也合法）。`LogType` 是不做大小写、空白或
别名规范化的开放字符串；对合法 UTF-8 值，`Logs` 将其原样放入 `type` 字段，包括
空字符串，不按业务内容本地拒绝，其他未知类型由远端决定。非法 UTF-8 无法无损
编码为 JSON string，在发送前返回 `CodeInvalidArgument`/`DeliveryNotSent`。可用 Log Type
集合是每次读取的动态快照，可能随 Driver、Capabilities、当前 Context 或其他 Session
状态变化。因此 `LogTypes` 返回的任一合法 UTF-8 值（包括空字符串）都可以不经修改
直接作为 `Logs` 参数，SDK 不制造读取与按类型读取之间的值域不对称。

`LogEntry` 必须是对象并同时包含 `timestamp`、`level`、`message`：

- `timestamp` 必须是可无损转换为 `int64` 的 JSON 整数，表示 Unix epoch 毫秒；
  允许零和负值，不接受小数、`null`、超范围或其他单位；
- `level` 和 `message` 必须是字符串，保留远端大小写和空值，不建立级别枚举；
- 其他字段递归保存在 `LogEntry.Extra`，不解释、不丢弃，且返回值必须是独立
  深拷贝。

成功 value 必须是 JSON array；`null`、object、string 和其他类型均无效。数组
顺序和重复项按远端保留；合法空数组返回非 nil 空 slice。响应解码是整体
成功语义：任一 Entry 或未知字段不符合 JSON/字段契约时返回
`CodeResponseInvalid`/`DeliveryAcknowledged`，不返回部分结果。

两个命令使用独立 `Limits.MaxLogResponseBytes`（DP-081 默认 32 MiB）限制完整
HTTP 响应体；超限返回 `CodeResponseTooLarge`/`DeliveryAcknowledged`，不截断。
该限制按调用计算，不是 Entry 数量或 Session 累积配额。客户端不缓存 Log Types、
Entry 或游标，不假设读取会清空 Driver 缓存，不自动轮询、分页、合并、过滤、去重
或重试；Driver 的消费结果需在具体兼容性组合中另行记录。当前不提供
`LogsTo(io.Writer)`、JSONL 或其他 Writer 交付形式。完整公共类型和取舍见
`docs/design.md` §10.1。

## Execute Script 与平台 operation identity

`Session.ExecuteScript`、`Session.ExecuteScriptWithOperation` 和
`Session.ExecuteScriptWithOperationAndDecode` 都发送同一个请求：

```text
POST /session/{sessionId}/execute/sync
```

后两个入口的 `operation` 只写入本地 `Error.Operation` 和 Observer 事件，不进入
请求体，也不参与 HTTP Method、Route、Discovery、fallback 或 retry。该参数是调用方
提供的诊断 identity，必须匹配 ASCII 格式 `[a-z][a-z0-9_]{0,63}`；无效时在本地
返回 `CodeInvalidArgument`/`DeliveryNotSent`，不会把原始 identity 放入错误或发送
请求；调用方仍需保持 identity 集合低基数且稳定。`ExecuteScriptWithOperation` 返回原始 `value`；
`ExecuteScriptWithOperationAndDecode` 将调用方 decoder 放在统一命令执行链的
response decoder 阶段，decoder 错误返回 `CodeResponseInvalid`（或相应 context/
output 错误），并保留收到响应时的 HTTP StatusCode、`DeliveryAcknowledged`、
operation 以及 Observer `Finished.ErrorCode`。这两个入口都不是任意 Method/Route 的
Raw Command API。普通调用方无需区分 identity 时继续使用 `ExecuteScript`；平台强
类型 Execute Method 应使用带 decoder 的入口。

## Session Screenshot

`Session.Screenshot` 和 `Session.ScreenshotTo` 使用同一条 W3C Screenshot
命令和 Base64 解码路径：

| API | HTTP | 路径 | 请求体 | 成功值 |
|---|---|---|---|---|
| `Session.Screenshot` | GET | `/session/{sessionId}/screenshot` | 无 | 解码后的 PNG 字节 |
| `Session.ScreenshotTo(io.Writer)` | GET | `/session/{sessionId}/screenshot` | 无 | 写入目标的解码后 PNG 字节数 |

请求不带 body，也不发送 `Content-Type`。远端成功 value 必须是 JSON
string，内容必须是紧凑的标准 Base64；空字符串表示零字节截图。截图命令的
响应体和解码后数据均使用 `Limits.MaxScreenshotResponseBytes` 上限。
`Screenshot` 通过内存 Writer 调用 `ScreenshotTo`，不维护另一套解码或校验逻辑。

`ScreenshotTo` 返回已经写入目标 Writer 的字节数。Base64 解码失败时返回
`CodeResponseInvalid`；Writer 写入失败时返回 `CodeOutputFailed`，并保留原始
Writer error 作为 Cause。任一错误发生时，目标中可能已经存在部分数据；调用方
context 结束时返回相应的取消/截止时间错误。客户端不自动重试或恢复 Session。
Viewport Screenshot 和本地裁剪不属于本命令契约。

`StopRecordingTo` 使用相同的流式输出错误语义：录屏响应有效但目标 Writer
失败时返回 `CodeOutputFailed`，而不是 `CodeResponseInvalid`。

## Session ViewportRect

`Session.ViewportRect` 通过根包统一 Execute Script 链读取当前 Driver 的
viewport 像素几何快照：

```text
POST /session/{sessionId}/execute/sync
script: "mobile: viewportRect"
args:   []
```

只接受创建 Session 后远端确认的精确 `automationName` `XCUITest` 或
`UiAutomator2`。前者映射 Driver 内部的 `getViewportRect`，后者映射
`mobileViewPortRect`；两者对外仍使用上面的同一脚本名。未知 Driver 在发送前
返回 `CodeUnsupported`、Delivery 为 `DeliveryNotSent`，不调用 Runtime Discovery
或其他探测命令。

成功 value 必须是 JSON object，并同时包含 `left`、`top`、`width`、`height`
四个 JSON number 字段。字段值必须是有限、可无损表示为当前 Go `int` 的整数；
原点不得为负，宽高必须为正，且右/下端点不得发生 `int` 溢出。缺失、`null`、
错误类型、别名字段、小数、超范围或溢出均返回 `CodeResponseInvalid`、Delivery
为 `DeliveryAcknowledged`。未知字段可以存在但不会参与几何计算。

返回的 `PixelRect` 只承载 Driver 报告的像素几何，不缓存、不执行 scale/density、
orientation 或 status bar 转换，也不自动关联或裁剪 Screenshot；每次调用都会
重新发送一次 Execute Script 请求。

## Element Screenshot

`Element.Screenshot` 和 `Element.ScreenshotTo` 使用 W3C Element Screenshot
命令，并与 Session Screenshot 共用 Base64 解码、资源上限和错误语义：

| API | HTTP | 路径 | 请求体 | 成功值 |
|---|---|---|---|---|
| `Element.Screenshot` | GET | `/session/{sessionId}/element/{elementId}/screenshot` | 无 | 解码后的 PNG 字节 |
| `Element.ScreenshotTo(io.Writer)` | GET | `/session/{sessionId}/element/{elementId}/screenshot` | 无 | 写入目标的解码后 PNG 字节数 |

Session ID 和 Element ID 按 Endpoint 规则作为独立路径段转义。请求不带 body，
也不发送 `Content-Type`。远端成功 value 必须是 JSON string，内容必须是紧凑的
标准 Base64；空字符串表示零字节截图。响应体和解码后数据均使用
`Limits.MaxScreenshotResponseBytes` 上限。

Element Screenshot 只报告 Driver 的标准截图结果，不自动滚动元素、恢复可见性
或处理 stale 引用，也不承诺与完整截图按 Element Rect 本地裁剪等价。

## 通用显式等待（DP-070）

`wait.Until` 是客户端本地轮询辅助，不发送或修改任何 Appium 命令：

```text
wait.Until(ctx, interval, condition)
```

条件会先立即执行一次，并按以下结果决定流程；如果条件返回时 context
已经结束，即使返回 `true, nil` 也按 context 结束处理：

- `false, nil`：等待 `interval` 后继续下一轮；
- `true, nil`：成功返回 `nil`；
- 非 nil error：立即返回该错误，不自动重试或包装。

`ctx` 必须非 nil，`interval` 必须为正数，`condition` 必须非 nil。轮询间隔
受调用方 context 约束，condition 收到同一个 context 并负责在自身执行中遵守；
context 在下一轮开始前或间隔等待期间结束时，返回对应的 context 错误。
Until 不为条件创建后台 goroutine，不设置 Session Timeout，也不推断条件的业务
语义；长时间 Implicit Wait 与高频显式轮询叠加时，每轮条件调用仍可能等待远端
隐式超时。

## Element 显式等待（DP-071）

`wait.Element` 和 `wait.Elements` 是建立在根包查找 API 上的客户端本地轮询：

```text
wait.Element(ctx, interval, finder, locator)
wait.Elements(ctx, interval, finder, locator)
```

`finder` 可以是根包的 `*appium.Session` 或 `*appium.Element`；前者执行
Session 级查找，后者执行元素后代查找。每一轮都会重新调用对应的公共
`Find` 或 `FindElements`，因此请求、Observer 事件、隐式等待和根包错误语义
都保持不变，wait 包不建立自己的 HTTP 通道。

两种等待都会先立即执行一次。`Element` 在获得非 nil 元素时成功；
`Elements` 只有在获得至少一个元素时成功，根包返回的空集合会继续轮询。
根包 Find API 自身如果违反响应契约，会按根包命令语义返回
`CodeResponseInvalid`/`DeliveryAcknowledged`；如果结构化 finder 直接返回 nil
成功值或包含 nil 元素的集合，则属于本地 finder 契约错误，wait 立即结束并不
生成远端错误码或 Delivery 状态。
只有根包明确标记为 `CodeElementNotFound` 的错误会被压低为暂态结果并按
`interval` 重试。stale、Session 丢失、参数、响应格式和传输错误均立即原样
返回，不根据错误文本或 Delivery 推断其他可重试类别。

调用方 context 是唯一的总期限来源，`interval` 必须为正数。若任一 helper 在
期限结束前已经收到过未找到错误，且间隔等待或最后一次 Find 调用因 context
结束而终止，返回结果同时保留 context 错误和最后一次未找到错误（可分别使用
`errors.Is` 与 `appium.IsErrorCode` 检查）；context 命令错误作为主错误，
`appium.DeliveryOf` 报告其 Delivery，`IsErrorCode` 会遍历错误树中的所有分支。
`Elements` 仅收到空集合时没有底层错误，期限结束返回 context 错误。等待不会
修改 Implicit/Command Timeout，不会自动重新定位已返回的 Element，也不会恢复
stale 引用。长时间 Implicit Wait 与高频显式轮询叠加时，每一轮 Find 仍可能先
等待远端隐式超时。

## Runtime Discovery（DP-041 实现契约）

`Session.Commands` 和 `Session.Extensions` 分别读取当前 Session 的 Appium
Runtime Discovery 目录：

| API | HTTP | 路径 | 请求体 | 成功值 |
|---|---|---|---|---|
| `Session.Commands` | GET | `/session/{sessionId}/appium/commands` | 无 | Appium 3 command catalog，解码为 `CommandCatalog` |
| `Session.Extensions` | GET | `/session/{sessionId}/appium/extensions` | 无 | Appium 3 extension catalog，解码为 `ExtensionCatalog` |

请求不带 body，也不发送 `Content-Type`。每次调用都重新读取远端，不缓存目录。
目录条目的三类 identity、`base`/`driver`/`plugins[pluginName]` 来源层级、可选
section 和未知字段遵循 `docs/design.md` 的 Catalog 模型。Commands 的 `rest` /
`bidi` 与 Extensions 的 `rest` section 都可缺失；缺失与显式空 object 必须区分。
只要 section 存在，Commands 的 `base`、`driver` 或 Extensions 的 `driver` 就
必须存在且为 JSON object；这些 object 可以为空，`plugins` 则可缺失。`plugins`
存在时必须是 object，且每个 plugin value 也必须是 object；显式 `null` 或其他
类型非法。因而顶层 `{}` 合法，而 `{"rest":{}}`、`{"bidi":{}}` 及 Extensions
的 `{"rest":{}}` 非法。Params 是对象数组，每项必须包含非空 string `name` 与
boolean `required`，并保留缺失与显式空数组的差异。HTTP、BiDi、Execute Method 分别按
`method+path`、`domain+name`、`name` 精确匹配；Source 仅记录 provenance，不参与
Supports 查询。非法目录结构、空的结构性标识符、必需 child 缺失或无法解码的
已知字段返回 `CodeResponseInvalid`，Delivery 为 `DeliveryAcknowledged`。本节定义
DP-041 的协议边界，DP-040 本身不增加运行时 API 或远端请求。
