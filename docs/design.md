# soluna-appium-client SDK 设计

> 文档状态：Draft  
> 适用阶段：v0.x 至首个稳定版本  
> 技术基线：Go 1.26.5，Appium 3.x  
> 最后更新：2026-08-30

## 1. 文档职责

本文档承接 `docs/architecture.md` 下沉的跨领域设计内容，记录公共对象如何协作、命令和事件如何执行、状态如何处理、平台扩展如何接入，以及项目已经确认的设计决策。

本文档不承担以下职责：

- 能力是否已经实现及其优先级，以 `docs/sdk-capability-matrix.md` 为准；
- 单个命令的完整请求和响应契约，以 `docs/command-semantics.md` 为准；
- 真实版本组合的支持结论，以 `docs/compatibility.md` 为准；
- 对外发布和破坏性变更规则，以 `docs/release-policy.md` 为准。

本文档可以同时描述当前实现和已经接受的目标设计。尚未实现的部分必须在能力矩阵中保持 `Accepted` 或 `Architecture` 状态，不能因为设计已写入本文档就标记为已实现。

## 2. 统一 Client 模型

### 2.1 公共对象所有权

SDK 只公开根包中的 `appium.Client` 作为 Client 类型。

```text
appium.Client
    └── 创建 appium.Session
            └── 产生 appium.Element
```

连接多个 Appium Endpoint 时可以创建多个 `appium.Client` 实例，但不存在以下平行公共对象：

```text
xcuitest.Client
uiautomator2.Client
bidi.Client
xcuitest.Session
uiautomator2.Session
```

平台包和 `wait` 包只消费根包对象，不拥有独立连接或 Session 生命周期。

### 2.2 平台函数形态

平台扩展优先采用无状态函数：

```go
func Capability(
    ctx context.Context,
    session *appium.Session,
    // typed arguments
) error
```

元素级平台能力可以同时接收 Session 和 Element：

```go
func ElementCapability(
    ctx context.Context,
    session *appium.Session,
    element *appium.Element,
    // typed arguments
) error
```

平台包不得为了方法调用风格而包装根包 Session。这样可以避免产生第二套状态、关闭语义、配置和错误模型。

### 2.3 HTTP 与 BiDi

HTTP 和 WebDriver BiDi 可以使用不同的底层连接，但都绑定到同一个 `appium.Session`。BiDi 加入后，连接建立、订阅和关闭由根包公共抽象管理，平台包只负责特有命令参数和事件类型。

调用方不需要创建或维护独立 BiDi Client。

## 3. Session 生命周期

### 3.1 创建

创建 Session 时只信任远端成功响应中确认的 Session ID 和 Capabilities。调用请求中的原始 Capability 不能替代远端确认事实。

如果远端已经创建 Session，但后续响应解码或调用方 context 结束导致本地初始化失败，客户端应使用独立清理期限尽力删除远端 Session。

清理失败时，应把仅用于识别和显式关闭的 Session 句柄返回给调用方，不能让已经存在的远端 Session 完全不可见。

### 3.2 可用、丢失与关闭

Session 只维护本地可以确认的状态：

- 是否完成可用初始化；
- 是否已经确认关闭；
- 远端是否明确返回 Session 已不存在。

Session 不通过后台任务持续探测，也不推测网络错误后的远端状态。

`Healthy` 使用真实、只读、无副作用的 Driver 命令作为 operational probe。成功只说明探测时刻命令链可用，不保证下一条命令一定成功。

`Close` 应可重复调用。只有成功删除或远端明确返回 Session 不存在时，才把本地 Session 标记为已关闭。传输结果不确定时保留未确认状态。

### 3.3 恢复边界

SDK 不自动：

- 创建替代 Session；
- 重放 Capabilities；
- 恢复 App 前后台、页面、登录或业务状态；
- 重新定位 stale element；
- 重建已经断开的事件订阅。

这些操作需要业务知识，由调用方或 Soluna 上层执行器负责。

## 4. HTTP 命令执行模型

所有远端 HTTP 命令进入同一执行链：

```text
公共方法
    ↓
本地对象与参数校验
    ↓
请求编码
    ↓
Observer: started
    ↓
context + command timeout
    ↓
internal/wire transport
    ↓
W3C/Appium envelope 解析
    ↓
命令级 value 解码
    ↓
统一 Error / Delivery 映射
    ↓
Observer: finished
```

### 4.1 context 与 timeout

所有远端命令必须接收非 nil `context.Context`。调用方 context 的更早截止时间优先于 Client 默认命令超时。

内部命令超时只约束远端执行阶段。参数编码和响应值解码仍应检查调用方 context，避免大响应处理在调用方取消后继续运行。

Appium 3 的 Get Timeouts 响应按实际 `GetTimeoutsResult` 建模为 `command` 和
`implicit` 两个数值超时。SDK 不根据 SetTimeout 请求中可发送的 `script` 或
`pageLoad` 字段推断读取结果，也不将 W3C 理论模型强行补齐为 Appium 未返回的字段。
读取结果使用独立的 `CurrentTimeouts` 类型；既有用于设置超时的 `Timeouts`
公共类型继续保留 `Script`、`PageLoad` 和 `Implicit` 字段，以避免无关的 API 破坏。

### 4.2 命令投递状态

错误必须保留客户端能够确认的命令投递事实：

- `DeliveryNotSent`：确认未发送；
- `DeliveryUnknown`：请求已尝试，但无法确认远端是否接收或执行；
- `DeliveryAcknowledged`：已经收到远端响应。

投递状态不等于“可否重试”。尤其对点击、输入、滑动、脚本和应用控制等副作用命令，`DeliveryUnknown` 不能被自动重放。

### 4.3 重试

核心客户端不自动重试远端命令。

只读命令是否重试也不由传输层自行决定。显式等待或上层策略可以根据错误类型和业务场景重复调用，但必须保留调用方对总期限和副作用的控制。

## 5. 平台扩展设计

### 5.1 Driver 门禁

平台函数必须在发送远端命令前校验 Session 的 `AutomationName`。判断依据是远端创建 Session 后确认的值，不使用原始 Capability，也不进行大小写规范化。

Driver 不匹配时返回：

```text
CodeUnsupported
DeliveryNotSent
```

Session 为空、未初始化或仅用于清理时返回参数类错误。平台门禁不调用 `Healthy`，不为了检查 Driver 额外发送远端请求。

### 5.2 传输复用

平台命令应通过根包公开的标准能力进入统一执行链，例如 W3C Script Execution 或后续明确的公共协议方法。平台包不得直接创建 HTTP Client、拼接 Appium Endpoint 或绕过根包错误与限制模型。

平台特有事件也由根包建立的 BiDi 通道交付；平台包只定义事件解码和强类型结果。

### 5.3 源码命名

保存 Driver 常量和 Session 门禁的文件不应命名为 `client.go`，避免暗示平台包拥有独立 Client。目标命名为：

```text
xcuitest/driver.go
uiautomator2/driver.go
```

现有 `xcuitest/client.go` 可在不改变行为的独立重构中重命名。

## 6. Capabilities、Settings 与 Runtime Discovery

三者描述不同状态：

| 类型 | 状态性质 | 客户端策略 |
|---|---|---|
| Capabilities | Session 创建后远端确认的快照 | Session 内保存深拷贝，返回时再次复制 |
| Settings | Session 中可变的 Driver/Plugin 状态 | 不缓存；更新结果不确定时不推测最终状态 |
| Command/Extension Catalog | 查询时刻远端登记的能力快照 | 显式读取，不建立隐式 Session 缓存 |

Session Settings 使用开放的 `Settings map[string]any` 表达 Driver/Plugin 定义的
键值。`GET /session/{id}/appium/settings` 每次读取并返回独立快照；
`POST /session/{id}/appium/settings` 只发送调用方提供的增量字段。客户端不维护
Setting 白名单、不自动规范化值，也不根据一次更新推测后续读取结果。

### 6.1 Runtime Discovery Catalog 公共模型

DP-040 只确定 Runtime Discovery 的公共模型和跨领域边界；远端命令与解码实现
由 DP-041 完成。公共模型保留 Appium 3 `ListCommandsResponse` /
`ListExtensionsResponse` 的层级和三类真实 identity，不把不同协议对象压成一个
字符串：

```go
type CatalogSourceKind string

const (
    CatalogSourceBase   CatalogSourceKind = "base"
    CatalogSourceDriver CatalogSourceKind = "driver"
    CatalogSourcePlugin CatalogSourceKind = "plugin"
)

type CatalogSource struct {
    Kind       CatalogSourceKind
    PluginName string // 仅 Kind == CatalogSourcePlugin 时有值
}

type CatalogParam struct {
    Name     string
    Required bool
    Extra    map[string]any
}

type CatalogMetadata struct {
    Command    *string
    Deprecated *bool
    Info       *string
    Params     []CatalogParam
    Extra      map[string]any
}

type HTTPCommand struct {
    CatalogMetadata
    Source CatalogSource
    Path   string
    Method string
}

type BiDiCommand struct {
    CatalogMetadata
    Source CatalogSource
    Domain string
    Name   string
}

type ExecuteMethod struct {
    CatalogMetadata
    Source CatalogSource
    Name   string
}

type HTTPCommandGroup struct {
    Entries []HTTPCommand
}

type BiDiCommandGroup struct {
    Entries []BiDiCommand
}

type ExecuteMethodGroup struct {
    Entries []ExecuteMethod
}

type CommandCatalog struct {
    Rest  *RestCommandCatalog
    BiDi  *BiDiCommandCatalog
    Extra map[string]any
}

type RestCommandCatalog struct {
    Base    HTTPCommandGroup
    Driver  HTTPCommandGroup
    Plugins map[string]HTTPCommandGroup
    Extra   map[string]any
}

type BiDiCommandCatalog struct {
    Base    BiDiCommandGroup
    Driver  BiDiCommandGroup
    Plugins map[string]BiDiCommandGroup
    Extra   map[string]any
}

type ExtensionCatalog struct {
    Rest  *RestExtensionCatalog
    Extra map[string]any
}

type RestExtensionCatalog struct {
    Driver  ExecuteMethodGroup
    Plugins map[string]ExecuteMethodGroup
    Extra   map[string]any
}
```

HTTP command 的 execution identity 是 `Method + Path`；BiDi command 的 execution
identity 是 `Domain + Name`；Execute Method 的 execution identity 是 `Name`。
`Command` 是远端条目的可选元数据，不能代替上述 identity；它缺失时仍是合法
条目。Path、Method、Domain、Name 以及 plugin map key 是协议对象的结构性标识，
必须非空并按原始字符串保存，不拼接、不 trim、不大小写折叠。

`CatalogSourceKind` 由响应所在的结构分支确定：`base`、`driver` 或
`plugins[pluginName]`。PluginName 是稳定事实，不能只用一个 plugin 枚举代替。
Source 不是远端条目里的自由字符串；未知的未来顶层 section 或分支字段放入
对应 `Extra`，客户端不得根据 endpoint、名称前缀或命令格式猜测来源。

`CatalogMetadata` 的 `Command`、`Deprecated`、`Info` 和 `Params` 均为可选字段：
字段缺失合法，显式空数组与缺失仍按 JSON 存在性区分；已知字段出现 `null` 或
其他错误类型时视为响应格式错误。`Params` 中每个对象的 `Name` 和 `Required`
是稳定字段，必须分别是非空 string 和 boolean；未知参数字段递归保存在该参数
的 `Extra` 中。`Command` 即使缺失，也不会影响 HTTP、BiDi 或 Execute Method
的结构性 identity。

`CommandCatalog.Rest`、`CommandCatalog.BiDi` 和 `ExtensionCatalog.Rest` 为可选
section：nil 表示响应中缺失，非 nil 的空结构表示远端明确返回了空 object，
两者不得混淆；section 若出现则必须是 JSON object，显式 `null` 或其他类型非法。
只要 section 存在，其 `Base` 与 `Driver` 就是必需的 JSON object；缺失任一字段
属于响应格式错误，显式空 object 则合法。`Plugins` 可缺失；其 map 的 nil 表示
缺失，非 nil 空 map 表示显式空 object；`plugins` 中每个 plugin value 必须是
JSON object，显式 `null` 或其他类型非法。Commands 不要求 Rest 或 BiDi 至少有一
个存在；Extensions 不要求 Rest 存在，因此空顶层 object 是合法的空目录，但
`{"rest":{}}`、`{"bidi":{}}` 或 Extensions 的 `{"rest":{}}` 均非法。

动态 path、method、domain、command name 和 execute method name 是已知结构键，
由 DP-041 展开为对应 Group 的 Entries；这些动态 key 全部属于协议 identity，不能
另行归入 Group 的未知字段。未知字段保存在目录、section、条目
`CatalogMetadata.Extra` 或参数 `Extra` 中，并对嵌套 map/slice 递归深拷贝。目录
返回不承诺 map 键顺序；客户端不依赖顺序，也不去重。无法解码为预期层级、必需
section child 缺失、已知元数据或参数字段类型错误，或结构性标识符为空时，整体
返回 `CodeResponseInvalid`，不返回部分目录。

`Session.Commands(ctx)` 和 `Session.Extensions(ctx)` 每次分别读取
`GET /session/{id}/appium/commands` 与
`GET /session/{id}/appium/extensions`，不建立 Session 缓存。GET 请求不带 body，
也不发送 `Content-Type`。每次返回的新目录及其所有子结构均独立；调用方修改
快照不会影响后续读取。

目录类型提供按 identity 分开的纯本地 helper：

```go
func (c CommandCatalog) SupportsHTTP(method, path string) bool
func (c CommandCatalog) SupportsBiDi(domain, name string) bool
func (c ExtensionCatalog) SupportsExecuteMethod(name string) bool
```

Helper 只执行区分大小写、逐字节相等的精确匹配，不 trim、不做大小写折叠、不接受
前缀、通配符、别名或拼接格式；空参数始终返回 false。Source 是 provenance，
不属于这些 Supports execution identity，因此 helper 不检查 Source、CatalogMetadata
或设备状态，也不表示实际命令一定能够成功。

Runtime Discovery 只回答“当前 Session 登记了什么”，不能推导：

- 当前设备 OS 满足命令最低版本；
- 当前是真机或模拟器；
- 当前权限、Context 或页面状态允许执行；
- Driver 实现没有运行时缺陷；
- 命令一定成功。

Catalog 上的 `Supports` helper 是纯本地快照查询。普通公共方法不以 Discovery 结果作为隐式前置门禁，不自动 fallback，也不改变远端实际错误。

## 7. Element 设计

### 7.1 引用模型

Element 只保存所属 Session 和远端 Element ID，不缓存：

- Locator；
- 文本和属性；
- Rect；
- 可见性；
- 截图。

Element stale 后不自动重新定位，因为 SDK 不保存足够的业务语义，也无法保证重新定位到同一业务对象。

### 7.2 查找作用域

Session 级查找和 Element 级后代查找使用相同的结果筛选原则：

1. 获取远端返回的全部候选；
2. 保持远端顺序；
3. 读取当前 Window Rect；
4. 逐个读取候选 Rect；
5. 只保留与当前 Window 存在正面积交集的候选。

`Find` 返回第一个有效候选并立即停止；`FindElements` 返回全部有效候选。Rect 获取失败时不静默跳过，也不返回部分结果。

该规则当前只对 Native Context 成立。引入 Web Context 前必须单独设计 DOM 元素坐标与浏览器 viewport 的关系，不能直接复用 Native 的 Window Rect 相交算法。

### 7.3 确定坐标点击

Element 默认点击不依赖 Driver 的元素 click 语义，而是：

1. 重新读取 Window Rect；
2. 重新读取 Element Rect；
3. 计算正面积交集；
4. 选择交集区域中的确定坐标；
5. 使用根包 W3C Actions 点击。

默认位置为交集区域中心。指定比例时，比例基于交集区域计算，不基于 Element 原始 Rect。

Find 成功不代表后续 Tap 可以使用旧坐标。每次 Tap 都重新获取几何状态。

### 7.4 Element Screenshot

Element Screenshot 采用标准远端 Element Screenshot 语义，并同时规划内存返回和 `io.Writer` 两种交付方式。

该能力不承诺：

- 自动滚动元素；
- 自动恢复可见性；
- 自动处理 stale；
- 与完整截图按 Element Rect 本地裁剪完全一致；
- Element Rect 与截图像素直接一一对应。

## 8. 坐标与视觉产物

项目明确区分两类坐标空间：

```text
WebDriver 几何
    Rect / Point
    Window Rect
    Element Rect
    W3C Actions

图像几何
    PixelRect
    Screenshot
    Viewport
    OCR / CV
```

两类坐标不能通过相同 Go 类型混用。

Driver 返回的 scale、status bar、orientation 等只作为坐标转换的辅助事实。没有完整且经过验证的转换模型前，SDK 不执行隐式缩放、偏移或方向修正。

`ViewportRect` 属于截图像素空间，不替换 `WindowRect` 参与 Element 查找或 Actions。

## 9. 大型响应与二进制产物

Page Source、Screenshot、Element Screenshot、Recording、Pull Logs 和 BiDi Event 使用独立资源类别，不能全部复用普通命令响应上限。

二进制产物优先提供：

```text
便捷方法
    返回 []byte

流式方法
    写入 io.Writer
    返回已写入字节数
```

便捷方法应复用流式解码路径，避免出现两套 Base64 校验、上限和错误语义。

Base64 解码、context 结束或 Writer 写入失败时，Writer 可能已经包含部分数据。流式方法必须同时返回已写入字节数和错误；其中 Writer 交付失败统一映射为 `CodeOutputFailed`，不与远端响应格式错误混淆。

当前 Screenshot 使用 `Limits.MaxScreenshotResponseBytes` 作为独立资源上限，
同时约束 HTTP 响应体读取和 Base64 解码后的截图数据；`Session.Screenshot`
通过内存 Writer 复用 `Session.ScreenshotTo` 的完整解码路径。Element Screenshot
尚未在本计划项中实现。

资源配置只在对应能力实现时加入公共 `Limits`，不提前暴露没有实际效果的字段。

## 10. Logs 与 WebDriver BiDi

### 10.1 Pull Logs

Pull Logs 由 Session 主动请求一批日志。根包使用开放日志类型，不维护所有 Driver 的固定枚举。

公共 API 只报告远端返回的数据，不假设读取是否清空远端缓存。消费语义必须在命令文档和兼容性验证中记录。

### 10.2 Streaming Logs 与监控流

Streaming Logs、系统监控和网络监控通过 WebDriver BiDi 持续交付。根包负责通用连接、订阅、取消、消息关联、资源限制和关闭；平台包负责特有事件模型。

事件流必须满足：

- 与一个 Session 明确绑定；
- Session 关闭后停止交付；
- context 结束能够终止等待和读取；
- 单条消息和本地待消费队列有硬上限；
- 队列溢出、协议错误和连接关闭可观察；
- 不静默丢弃关键事件；
- 不自动重连。

自动重连可能造成事件丢失、重复订阅或跨越已经失效的 Session，因此由调用方决定是否建立新流。

### 10.3 Observer 边界

`Observer` 只观察 Go Client 自己发送的命令生命周期。它不承载设备日志、App 日志、Driver 日志或 BiDi 业务事件。

## 11. 显式等待

`wait` 包建立在根包公共 API 上，不进入传输层，也不修改 Session 配置。

显式等待应遵循：

- 总截止时间由调用方 context 决定；
- 轮询间隔明确且可配置；
- 只重试条件声明为暂态的结果；
- 参数错误、响应格式错误和 Session 丢失默认立即返回；
- 不吞掉最终一次有诊断价值的错误；
- 文档明确说明长时间 Implicit Wait 与高频显式轮询叠加的风险。

## 12. 并发模型

`Client` 和 Session 的本地状态必须具备内存安全性，但 SDK 不为同一 Session 自动串行化业务命令。

多个 goroutine 并发调用同一 Session 时，远端执行顺序不由 SDK 保证。需要严格顺序的调用方必须在执行层自行调度。

Session 关闭、事件订阅和取消必须能与普通命令并发安全地交互。事件流是否允许多个消费者，应由具体公共接口明确；不能依赖多个 goroutine 竞争读取来定义事件顺序。

## 13. Error、Delivery 与诊断数据

公共 `Error` 描述失败事实，不直接给出 `Retryable` 结论。

错误模型至少区分：

- 客户端配置错误；
- 调用参数错误；
- context 取消和截止时间；
- HTTP/WebSocket 传输失败；
- 远端响应或事件格式错误；
- 远端命令失败；
- Session 丢失；
- Element 不存在或 stale；
- 响应或消息超过限制；
- 事件流关闭或消费能力不足。

远端错误文本和数据在进入公共 Error 前必须执行大小限制与必要脱敏。默认日志不记录输入文本、页面源、截图、录屏、剪贴板、完整 Pull Log 或原始 BiDi 消息。

标准 W3C Alert 的 `no such alert` 错误映射为独立的 `CodeAlertNotFound`，以保留“当前不存在可操作 Alert”这一稳定领域事实；不提供 `HasAlert` 探测接口，避免检查与操作之间的 TOCTOU 窗口。

## 14. 兼容性与 Host 跨平台设计

兼容性至少包含以下独立维度：

```text
SDK 版本
Appium 版本
Driver 版本
WDA / UiAutomator2 Server 版本
设备 OS
真机或模拟器
Appium Host OS
连接与启动方式
具体能力
```

平台能力可以是 iOS-only 或 Android-only，但 Host OS 仍是独立准入维度。依赖 Xcode、`xctrace`、`simctl`、`devicectl` 等单一 Host 工具的能力不能被描述为跨 Host。

当前产品边界为：

- Appium 3.x 是主协议基线；
- iOS 17+ 是 XCUITest 主线设备范围；
- iOS 17 以下进入 Legacy Lane，不反向限制主线 API；
- macOS、Windows 和 Linux 的具体支持结论只在真实兼容性矩阵中登记；
- 每个 XCUITest 平台能力都要单独记录最低 iOS、Driver/WDA、设备类型和 Host 条件。

支持状态建议使用：

- `Verified`：项目真实验证；
- `Official`：上游明确支持，但项目尚未实测；
- `BestEffort`：上游可能工作但未重点测试；
- `Unsupported`：上游明确不支持或必要机制缺失。

## 15. 源码组织

源码按行为域拆分，不把所有 Session 或 Element 方法集中在单个文件。

目标边界包括：

```text
session.go          Session 创建、元数据、健康和关闭
element*.go         元素引用、查找、读取、输入、点击和截图
screenshot.go       Session/Element Screenshot 公共处理
settings.go         Session Settings
discovery.go        Runtime Discovery
logs.go             Pull Logs
contexts.go         Context
alerts.go           Alert
keyboard.go         Keyboard
orientation.go      Orientation
navigation.go       Background、Deep Link 等导航行为
recording.go        Recording
timeouts.go         Timeouts
application.go      App 生命周期
actions.go          W3C Actions
internal/wire       HTTP 协议实现
internal/bidi       BiDi 协议实现
```

文件名可以随实现调整，但名称不能暗示不存在的公共抽象。例如平台 Driver 门禁文件不应长期命名为 `client.go`。

共享编码和响应校验使用私有 helper。不能为了平台包复用而扩大无必要的根包公共 API。

## 16. 测试设计

测试分为四层：

1. 单元测试：参数、编解码、本地状态和纯函数；
2. HTTP 协议测试：请求路径、请求体、响应、顺序和 Delivery；
3. BiDi 协议测试：连接、订阅、事件关联、取消、上限和关闭；
4. 真实兼容性测试：具体 Host、Appium、Driver、设备和能力组合。

一个能力只有具备公共 API、实现、必要校验、错误语义、资源边界和协议测试后，才能在能力矩阵中标记为 `Implemented`。

## 17. 待完成的设计专题

下列能力已经纳入 SDK 范围，但在实现前仍需要独立详细设计：

- Web Context 下 Element 查找、Rect 和可操作性几何；
- WebDriver BiDi 公共订阅接口及背压模型；
- Pull Log 的公共 Entry 类型与时间字段；
- Streaming Log 与平台监控事件的公共/平台类型边界；
- Viewport 与 Screenshot 像素坐标的验证方法；
- Runtime Discovery Catalog 的稳定 Go 类型。

专题完成后应更新本文档对应章节，并在 `docs/command-semantics.md` 中记录最终命令契约。

## 18. 设计决策索引

本节保存已经确认的设计决策。ID 一旦分配不复用；决策被替代时保留原记录并标记为 `Superseded`。

| ID | 状态 | 决策 | 主要影响 |
|---|---|---|---|
| AD-001 | Accepted | 项目以 Client、Session 和 Element 作为主要公共对象 | 其他能力围绕三个对象扩展，不引入平行核心模型 |
| AD-002 | Accepted | 只支持 Appium 3.x，不承担 Appium 2 兼容 | 协议和测试可以按 Appium 3 基线收敛 |
| AD-003 | Accepted | 最低 Go 版本为 Go 1.26.5 | 公共实现和 CI 以该版本为最低基线 |
| AD-004 | Accepted | 核心客户端不自动重试远端命令，也不自动重连事件流 | 调用方保留副作用和恢复控制 |
| AD-005 | Accepted | 不公开任意 Method/Route 的 Raw HTTP 扩展接口 | 所有正式能力必须进入统一协议与错误模型 |
| AD-006 | Accepted | 不提供云端 Appium 服务商专用 Header Provider | 云厂商认证由上层或自定义 HTTP Client 处理 |
| AD-007 | Accepted | 大型二进制产物优先同时提供内存返回和 `io.Writer` 方式 | 降低大截图和录屏的峰值内存 |
| AD-008 | Accepted | 不提供 Session 命令串行器 | 业务命令顺序由调用方执行层管理 |
| AD-009 | Accepted | Locator Strategy 不做旧名称兼容或自动规范化 | 协议值保持明确，错误尽早暴露 |
| AD-010 | Accepted | 公共 Error 只暴露经过限额和脱敏的远端数据 | 保留诊断价值并控制敏感信息和内存 |
| AD-011 | Accepted | Runtime Discovery 使用显式、无隐式缓存的快照 | 避免 Session、Driver 或 Plugin 变化后的缓存失真 |
| AD-012 | Accepted | XCUITest 与 UiAutomator2 使用独立平台包 | 根包保持通用，平台能力强类型隔离 |
| AD-013 | Accepted | 根包不公开巨型 Adapter 接口 | 调用方按使用场景定义最小接口 |
| AD-014 | Accepted | `docs/sdk-capability-matrix.md` 是 SDK 能力范围与状态的唯一维护入口 | 避免路线图散落在架构和 README |
| AD-015 | Accepted | HTTP 和 WebDriver BiDi 使用独立内部传输，但绑定同一 Session | 事件流不形成第二套公共客户端 |
| AD-016 | Accepted | Observer 与远端 Logs/Event Stream 是不同概念 | 命令遥测和设备/Driver 数据分别建模 |
| AD-017 | Accepted | Pull Logs 与 Streaming Logs 使用不同交付模型 | 不用单一 API 混淆批量读取和持续订阅 |
| AD-018 | Accepted | Element Screenshot 纳入根包，但不承诺自动滚动或与本地裁剪等价 | 只报告 Driver 的标准截图结果 |
| AD-019 | Accepted | 平台能力必须声明设备 OS、Driver/WDA、设备类型和 Appium Host OS | 能力价值与跨平台约束同时评审 |
| AD-020 | Accepted | iOS 17+ 为 XCUITest 主线设备范围，低于 iOS 17 按 Legacy Lane 维护 | 单台旧设备不限制主线 API 演进 |
| AD-021 | Accepted | Runtime Discovery 不作为普通命令的自动门禁、fallback 或成功保证 | 实际命令仍返回真实远端结果 |
| AD-022 | Accepted | SDK 只公开根包 `appium.Client`；平台包不定义 Client 或 Session wrapper | 调用方始终使用同一 Client/Session 对象模型 |
| AD-023 | Accepted | 架构文档只描述高层当前结构；详细规则和决策索引维护在设计文档 | 降低架构文档噪声并保持职责稳定 |
| AD-024 | Accepted | Runtime Discovery 按 Source provenance 与协议 execution identity 建模；未知字段递归保留，Supports 按 HTTP/BiDi/Execute Method 分开精确匹配 | 保留 Appium/Driver/Plugin 层级与真实命令身份，避免目录查询产生隐式能力推断 |

当某项决策需要完整记录背景、候选方案、权衡和迁移影响时，应新增：

```text
docs/adr/AD-XXX-<topic>.md
```

本表保留简短结论，并链接到对应 ADR。普通局部实现选择不需要创建 ADR。

## 19. 文档变更规则

以下变化需要更新本文档：

- 改变 Client、Session、Element 或平台扩展的协作方式；
- 改变命令执行、投递、重试或恢复语义；
- 改变 Capabilities、Settings、Discovery 的状态模型；
- 改变 Element 查找、点击或截图的跨命令语义；
- 改变坐标空间、大型产物、Logs 或 BiDi 的公共设计；
- 新增或替代设计决策；
- 改变兼容性分层或 Host 跨平台准入规则。

仅新增一个已经符合现有设计的普通命令时，不需要修改本文档；应更新能力矩阵、命令语义和测试。
