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
