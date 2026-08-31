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
