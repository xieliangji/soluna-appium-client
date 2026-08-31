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

## Session Pull Logs（DP-080 设计契约，待 DP-081 实现）

Pull Logs 只提供一次性的 Session 级批量读取。DP-081 将通过根包统一 HTTP
执行链实现以下 Appium 3 路由；`/se/` 是标准路由的一部分，不使用历史或
Driver 专用的 `/log` fallback：

| API | HTTP | 路径 | 请求体 | 成功值 |
|---|---|---|---|---|
| `Session.LogTypes` | GET | `/session/{sessionId}/se/log/types` | 无 | JSON string 数组，解码为 `[]LogType` |
| `Session.Logs` | POST | `/session/{sessionId}/se/log` | `{"type":"<LogType>"}` | JSON Entry 数组，解码为 `[]LogEntry` |

GET 不带 body，也不发送 `Content-Type`；POST 始终发送 JSON object。Session ID
按 Endpoint 规则作为独立路径段转义。通过本地校验后，每次调用只发送一次对应
请求，不隐式查询 Log Types、Discovery、Healthy 或其他命令。`LogTypes` 的数组
元素必须都是 JSON string（空字符串也合法）。`LogType` 是不做大小写、空白或
别名规范化的开放字符串；`Logs` 将其原样放入 `type` 字段，包括空字符串，不在
本地按内容拒绝，其他未知类型由远端决定。可用 Log Type 集合是每次读取的动态
快照，可能随 Driver、Capabilities、当前 Context 或其他 Session 状态变化。
因此 `LogTypes` 返回的任一值（包括空字符串）都可以不经修改直接作为 `Logs`
参数，SDK 不制造读取与按类型读取之间的值域不对称。

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
