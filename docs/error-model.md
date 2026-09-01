# Error、Delivery 与诊断数据

公共命令错误使用根包 `Error` 表达，并通过 `ErrorCode` 区分失败事实。
命令执行不会根据错误自动重试或恢复 Session。

## Alert 错误映射

标准 W3C Alert 远端错误映射如下：

| 远端 `value.error` | 公共错误码 | 说明 |
|---|---|---|
| `no such alert` | `CodeAlertNotFound` | 当前没有可操作的 Alert |

`CodeAlertNotFound` 与 `CodeElementNotFound`、`CodeElementStale` 一样，
保留稳定的领域事实；它不表示客户端已执行 Alert 探测，也不提供重试建议。

## Alert 成功值

- `AlertText` 接受 JSON string 或 `null`；返回值中的 `hasText` 区分两者，
  因此空字符串不会与缺失文本混淆。
- `AcceptAlert`、`DismissAlert`、`SetAlertText` 只接受 JSON `null` 成功值。
- 成功响应 value 类型不符合命令契约时返回 `CodeResponseInvalid`，
  Delivery 为 `DeliveryAcknowledged`。

## Timeouts 响应错误

`Session.Timeouts` 返回 `CurrentTimeouts`，并对 `command` 或 `implicit` 字段缺失、显式 `null`、负数、
非整数毫秒或超出 `time.Duration` 范围的字段返回 `CodeResponseInvalid`，
Delivery 为 `DeliveryAcknowledged`。显式零毫秒是合法值，不会被当作字段缺失。
Appium 3 的读取结果只建模 `command` 和 `implicit`，不推断 `script` 或
`pageLoad`。用于设置超时的既有 `Timeouts` 公共类型保持 `Script`、`PageLoad`
和 `Implicit` 字段，不因读取结果模型改变。

## Settings 响应与参数错误

`Session.Settings` 的成功 value 必须是 JSON object；`null`、数组、字符串或
其他非法 JSON 返回 `CodeResponseInvalid`，Delivery 为 `DeliveryAcknowledged`。
`Session.UpdateSettings` 收到 nil `Settings` 时在发送请求前返回
`CodeInvalidArgument`，Delivery 为 `DeliveryNotSent`；远端返回非 null 成功值时
同样返回 `CodeResponseInvalid`，Delivery 为 `DeliveryAcknowledged`。

## Screenshot 与 Element Screenshot 响应及流式交付错误

`Session.Screenshot`、`Session.ScreenshotTo`、`Element.Screenshot` 与
`Element.ScreenshotTo` 共享 Screenshot 专用响应上限
`Limits.MaxScreenshotResponseBytes`。HTTP 响应体超过该上限时返回
`CodeResponseTooLarge`，Delivery 为 `DeliveryAcknowledged`。成功 value 不是
JSON string 或 Base64 格式不合法时返回 `CodeResponseInvalid`；Writer 写入失败
时返回 `CodeOutputFailed`，Delivery 仍为 `DeliveryAcknowledged`，并通过 `Cause`
保留原始 Writer error。两种 `ScreenshotTo` 仍返回已经写入的字节数。

Element Screenshot 收到远端 `stale element reference`（映射为
`CodeElementStale`）或其他 W3C 命令错误时，沿用统一远端错误映射和 Delivery
语义；客户端不会自动重新定位或重试。

## Pull Logs 响应错误

`Session.LogTypes` 和 `Session.Logs` 的成功 value 必须分别是 JSON string 数组
和 `LogEntry` 数组。顶层 `null`、错误类型、非字符串 Log Type、缺失或类型错误
的 Entry 标准字段、非法 JSON、非法 UTF-8、时间戳非整数或超出 `int64` 范围时，
返回 `CodeResponseInvalid`，Delivery 为 `DeliveryAcknowledged`，且不返回部分结果。
未知 Entry 字段按 `docs/design.md` 的递归 `Extra` 规则保留；无法保留的值同样
按响应格式错误处理。

`LogType` 对合法 UTF-8 值完全开放透传，包括空字符串；客户端不因值为空或未曾出现在
`LogTypes` 结果中而本地拒绝，远端决定是否支持。Go string 中的非法 UTF-8 无法无损
编码为 JSON string，`Logs` 在发送前返回 `CodeInvalidArgument`/`DeliveryNotSent`，不
静默替换字符。可用 Log Type 集合是动态快照，可能随 Driver、Capabilities、当前 Context
或其他 Session 状态变化。两种读取都
使用独立的 `Limits.MaxLogResponseBytes`；完整 HTTP 响应体超过配置上限时返回
`CodeResponseTooLarge`，Delivery 为 `DeliveryAcknowledged`，不截断或交付部分
Entry。远端日志错误、传输错误和 context 取消继续沿用统一命令投递语义；客户端
不因读取看起来只读而自动重试、缓存或假设 Driver 已消费日志。

## 通用显式等待错误

`wait.Until` 不拥有独立的远端命令，因此不会生成或修改 Delivery 状态。条件
返回的 error 原样交还调用方，包含根包 `*Error` 时保留其错误码、Operation、
Delivery 与 Cause。Until 在轮询间隔、下一轮开始前或条件返回成功后观察到调用方
context 结束时，返回 `context.Canceled` 或 `context.DeadlineExceeded`；调用方可以
使用 `errors.Is` 判断。nil context、nil 条件和非正轮询间隔属于本地参数错误，
函数不会调用条件或发送远端请求。

## Element 显式等待错误

`wait.Element` 与 `wait.Elements` 不拥有独立的远端命令；每一轮查找产生的
请求和 Observer 生命周期都由根包 `Find`/`FindElements` 负责。wait 只把
`CodeElementNotFound`（以及 `FindElements` 的空集合结果）作为暂态结果，按
调用方指定的间隔继续轮询。它不根据错误文本、HTTP 状态或 Delivery 自行扩大
重试范围。

stale、Session 丢失、参数、响应格式、传输和其他远端错误在第一次出现时
立即原样交还调用方，保留根包 `*Error` 的错误码、Operation、Delivery 和
Cause；客户端不会自动重新定位或恢复 stale Element。

根包 Find API 从远端响应解码出不符合公共结果契约的 nil 成功值或 nil 集合
元素时，继续按根包命令语义返回 `CodeResponseInvalid` 与
`DeliveryAcknowledged`。wait 的 finder 参数采用结构化方法接口，因此任意
本地实现也可能返回 malformed 成功值；这属于本地 finder 契约错误，wait 立即
返回普通本地 error，不生成 `CodeResponseInvalid`，也不伪造任何 Delivery 状态
（`appium.DeliveryOf` 对此返回 `DeliveryUnknown`）。两者都不会被当作未找到
结果重试。

如果 `wait.Element` 或 `wait.Elements` 在 context 结束前至少收到一次
`CodeElementNotFound`，且间隔等待或最终 Find 调用因 context 结束，返回错误会
同时包含 context 结果与最后一次未找到错误。调用方可用 `errors.Is` 判断
`context.Canceled`/`context.DeadlineExceeded`；`appium.IsErrorCode` 遍历多错误树
中的所有结构化错误码，`appium.DeliveryOf` 报告排在主错误位置的 context 命令
Delivery。若 `wait.Elements` 的每一轮都只是合法空集合，则底层没有错误可保留，
context 结果单独返回。wait helper 不生成新的 Delivery 状态。

nil Writer 属于本地参数错误，返回 `CodeInvalidArgument`、Delivery 为
`DeliveryNotSent`，不会发送远端请求。调用方 context 在请求发送前结束时返回
相应的取消/截止时间错误并保持 `DeliveryNotSent`；响应收到后解码期间结束时
保持 `DeliveryAcknowledged`。客户端不因这些错误自动重试或恢复 Session。

`StopRecordingTo` 对合法录屏响应的 Writer 失败也返回 `CodeOutputFailed`，并保留
已经写入的字节数和原始 Writer error；Base64 或响应格式错误仍返回
`CodeResponseInvalid`。

## ViewportRect 响应与 Driver 门禁

`Session.ViewportRect` 只允许远端确认的 `XCUITest` 和 `UiAutomator2`。空、未初始化
或不可用于普通命令的 Session 返回 `CodeInvalidArgument`；其他
`automationName` 在发送前返回 `CodeUnsupported`，两者 Delivery 均为
`DeliveryNotSent`，且不会触发额外探测请求。

Viewport value 缺失 `left`、`top`、`width` 或 `height`，为 `null`、非 JSON number、
小数、非有限值、超出 `int` 范围、负原点、非正尺寸或端点溢出时，返回
`CodeResponseInvalid`，Delivery 为 `DeliveryAcknowledged`，不返回部分
`PixelRect`。远端 Driver、Session、传输和 context 错误继续使用统一命令映射；
客户端不自动重试、换算或缓存该结果。

## Runtime Discovery 响应错误

DP-041 的 `Session.Commands` 和 `Session.Extensions` 对目录响应执行整体严格
解码。目录结构非法、结构性 identity 缺失或为空、已知字段类型错误或未知字段
无法按 JSON 递归保留时，返回 `CodeResponseInvalid`，Delivery 为
`DeliveryAcknowledged`；
不返回部分目录。顶层 `rest`/`bidi`（Commands）和 `rest`（Extensions）可缺失，
但已存在 section 内的必需 `base`/`driver` child 缺失即为格式错误；显式空 child
object 合法，`plugins` 可缺失。`plugins` 存在时必须为 object，且每个 plugin value
必须为 object；显式 `null` 或其他类型均拒绝。`params` 每项必须是包含非空 `name`
string 与 `required` boolean 的对象；缺失字段、显式 `null` 或其他已知类型错误均
拒绝。
可选 section 与 `plugins` 的缺失和显式空 object 按 `docs/design.md` 的模型保留。
目录查询不会因为 `Supports` 结果而改变其他命令的错误或 Delivery 语义。

## Delivery

`IsErrorCode` 会检查普通 `Unwrap() error` 链和 `errors.Join` 的
`Unwrap() []error` 分支；`DeliveryOf` 在多错误树中返回第一个结构化
`*Error` 的 Delivery，调用方应先确定该主错误，再用 `IsErrorCode` 查询诊断
分支。

- `DeliveryNotSent`：调用方 context 已结束、参数无效或请求构造失败，
  客户端确认命令未发送；
- `DeliveryUnknown`：请求已尝试但没有收到 HTTP 响应，无法确认远端是否执行；
- `DeliveryAcknowledged`：已经收到远端 HTTP 响应，无论命令成功或失败。

Delivery 只描述投递事实，不代表命令是否可安全重试。

## 诊断数据

远端错误文本和 JSON 数据在进入公共 `Error` 前执行脱敏和大小限制。
公共错误文本不直接包含未处理的远端敏感数据。

`Error.Operation` 和 Observer 事件使用客户端定义或高级调用方提供且格式有界的
低基数 operation identity。平台 Execute Method 可以通过根包的高级 Execute Script
入口保留自身 identity（例如 `ios_press_button`）；该 identity 不改变实际 HTTP
Method、Route 或远端请求体。

高级 Execute Script 的 value decoder 在统一命令执行链中、Observer Finished 之前
运行。decoder 对平台响应格式的校验失败按 `CodeResponseInvalid` 归一化，并保留
实际 HTTP `StatusCode`、`DeliveryAcknowledged` 和同一个 `Operation`；因此调用方
错误与 Observer 看到的是同一条完整命令语义，不得在执行链返回后再单独构造响应
错误。
