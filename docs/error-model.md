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

## Runtime Discovery 响应错误

DP-041 的 `Session.Commands` 和 `Session.Extensions` 对目录响应执行整体严格
解码。目录结构非法、条目标识符缺失、已知字段类型错误或未知字段无法解析时，
返回 `CodeResponseInvalid`，Delivery 为 `DeliveryAcknowledged`；不返回部分目录。
目录查询不会因为 `Supports` 结果而改变其他命令的错误或 Delivery 语义。

## Delivery

- `DeliveryNotSent`：调用方 context 已结束、参数无效或请求构造失败，
  客户端确认命令未发送；
- `DeliveryUnknown`：请求已尝试但没有收到 HTTP 响应，无法确认远端是否执行；
- `DeliveryAcknowledged`：已经收到远端 HTTP 响应，无论命令成功或失败。

Delivery 只描述投递事实，不代表命令是否可安全重试。

## 诊断数据

远端错误文本和 JSON 数据在进入公共 `Error` 前执行脱敏和大小限制。
公共错误文本不直接包含未处理的远端敏感数据。
