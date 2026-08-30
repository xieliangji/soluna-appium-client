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

`Session.Timeouts` 每次读取 Appium 3 Get Timeouts 命令：

```text
GET /session/{sessionId}/timeouts
```

请求不带 body。成功值必须是同时包含 `command` 和 `implicit` 两个字段的
JSON object；字段值必须是非负整数毫秒或显式 `null`。零是有效值，字段缺失
属于响应格式错误。非空整数值超出 `time.Duration` 可表示范围时同样返回
`CodeResponseInvalid`。读取结果不在 Session 本地缓存。SetTimeout 请求中的
`script`、`pageLoad`、`implicit` 字段不承诺会从该读取命令中原样返回。
