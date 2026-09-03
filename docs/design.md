# soluna-appium-client SDK 设计

> 文档状态：Draft  
> 适用阶段：v0.x 至首个稳定版本  
> 技术基线：Go 1.26.5，Appium 3.x  
> 最后更新：2026-09-03

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

根目录 Go package name 固定为 `appium`；module path 仍为
`github.com/xieliangji/soluna-appium-client`。两者是独立的发布事实，平台包和
调用方都不应据此创建第二套 Client 类型。

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

根包提供两个固定路由的高级 Execute Script 入口：
`Session.ExecuteScriptWithOperation` 返回原始 `value`，
`Session.ExecuteScriptWithOperationAndDecode` 接收调用方的 `value` decoder。
两者的 `operation` 都是调用方提供的诊断 identity，只用于本地错误和 Observer，
必须匹配 ASCII 格式 `[a-z][a-z0-9_]{0,63}`，不参与 HTTP Method、Route、脚本
参数、Discovery、fallback 或 retry。第二个入口的 decoder 在统一
`executeCommand` decoder slot 中、`Observer.OnCommandFinished` 之前同步执行；
decoder 错误由根包统一映射并保留 HTTP StatusCode、Delivery 和 operation。平台包
可以用它实现强类型 Execute Method；这是跨包传递命令专有 decoder 的唯一受控边界，
不再提供更泛化的 execute hook。普通调用方不需要独立 identity 时继续使用
`Session.ExecuteScript`。这两个入口都不是任意 Method/Route 的 Raw API。

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

本节中的 `Context` 默认指 Appium application context（例如 `NATIVE_APP`、
`WEBVIEW_xxx` 或 `CHROMIUM`）。W3C `browsing context` 仅用于解释 WebDriver
DOM 几何规范；它可以表示浏览器中的顶层页面或 frame，不代表 SDK 在本设计中
提供 frame/window switching API。

Session 级查找和 Element 级后代查找都先依据调用时取得的当前 Context
选择几何策略，再发送候选查找和几何命令。Context 读取只是策略选择用的快照，不是锁，也不与后续查找、
Rect 读取或点击组成原子事务；客户端不缓存该快照、不串行化 Session 命令，
Context 在任意一步发生变化时以远端实际响应为准。

只有需要按 Context 筛选几何的 `Find`、`FindElements`、`Element.Find`、
`Element.FindElements` 和 Element Tap 才执行这次 Context 快照。直接调用
`Element.Rect` 仍只发送标准 WebDriver Rect 命令并返回远端原始值，不因为当前
Context 未知而本地改写或拒绝；它也不隐式读取 viewport 或滚动偏移。

#### 7.2.1 Native Context

只有名称精确等于 `NATIVE_APP` 的 Context 才使用 Native 策略：

1. 获取远端返回的全部候选；
2. 保持远端顺序；
3. 读取当前 Window Rect；
4. 逐个读取候选 Rect；
5. 只保留与当前 Window 存在正面积交集的候选。

`Find` 返回第一个有效候选并立即停止；`FindElements` 返回全部有效候选。
Rect 获取失败时不静默跳过，也不返回部分结果。除 Context 策略选择所需的独立
快照外，该策略的候选/Rect 请求顺序、交集规则和错误语义保持既有 Native 行为；
因此实现会在这些命令前增加一次 `CurrentContext` 请求，但不会把该请求缓存或
替换为默认 Native 假设。

#### 7.2.2 Web Context

名称精确为 `WEBVIEW`、以 `WEBVIEW_` 开头且带有非空后缀，或精确为
`CHROMIUM` 时，使用 Web 策略。`CHROMIUM` 是 UiAutomator2 纯浏览器会话的
固定 Context 名称；Context 名称的后缀（页面 ID、Bundle ID、进程或其他 Driver
标识）只作为不透明字符串保留，客户端不解析或重写。

Web 查找仍然先通过 W3C plural element lookup 获取全部候选并保持远端顺序；
候选为空时沿用既有短路语义，不读取 viewport。存在候选时再在同一个 CSS 坐标
空间内：

1. 读取当前浏览器 layout viewport，原点固定为 `(0, 0)`，单位为 CSS pixel；
2. 逐个读取候选的 WebDriver Element Rect；
3. 将 Rect 的文档坐标减去该快照的 `scrollX`/`scrollY`，得到 viewport-relative
   的 CSS Rect；
4. 只保留与该 CSS viewport 存在正面积交集的候选。

WebDriver Get Element Rect 的 `X`/`Y` 按规范是相对于当前 browsing context
文档元素的绝对 CSS pixel 坐标；`Width`/`Height` 是元素 bounding rectangle
的 CSS pixel 尺寸。因而页面滚动时，原始 Rect 通常保持文档坐标，只有经
`scrollX`/`scrollY` 平移后的 viewport Rect 参与交集和点击。SDK 不把文档坐标
直接当作 viewport 坐标，也不把 CSS pixel 乘以 `devicePixelRatio`。若某个 Driver
返回与该 WebDriver 契约不同的坐标，客户端不从数值猜测、双重扣除或自动切换
另一种解释；该组合的差异必须在兼容性验证中记录。元素是 DOM 中存在的事实并
不等于当前 viewport 内存在可操作的几何；没有正面积交集时，`Find` 返回既有
`CodeElementNotFound`，`FindElements` 返回非 nil 空 slice。不自动滚动、展开
折叠容器、遍历或重算 iframe 边界，或以 JavaScript 重新定位元素；若调用方显式
切换了 browsing context，偏移和 Rect 仍以该上下文的远端命令事实为准。

未设计的 Context 名称（包括空字符串、`WEBVIEW_` 本身以及其他 Driver/Plugin
自定义名称）保持可由 Context API 读取，但不会被猜测为 Native 或 Web。
Context-sensitive Find/FindElements、Element Find/FindElements 或 Element Tap 在
完成该 Context 快照后即返回主体操作的 `CodeUnsupported`/`DeliveryNotSent`，不发送
候选查找、几何探测或动作请求；成功的 `CurrentContext` 探针只通过它自己的
Observer 事件保留，不改变主体操作的 Delivery；也不以 Native Window Rect 或
Web CSS viewport 作为隐式 fallback。

性能优化不改变上述快照语义。当前先通过 benchmark 或测试基础设施记录候选数、
Rect probe 数、首个可见候选位置和总耗时；在缺少基线前不改为
`FindElement` 加隐式 `FindElements` fallback，也不并发查询候选 Rect。

### 7.3 确定坐标点击

Native Context 下，Element 默认点击不依赖 Driver 的元素 click 语义，而是：

1. 重新读取 Window Rect；
2. 重新读取 Element Rect；
3. 计算正面积交集；
4. 选择交集区域中的确定坐标；
5. 使用根包 W3C Actions 点击。

默认位置为交集区域中心。指定比例时，比例基于交集区域计算，不基于 Element 原始 Rect。

Web Context 下，`Element.Tap` 和 `Element.TapInWindowIntersection` 保持同一个
确定坐标原则，但将 Window Rect 替换为当前浏览器 CSS viewport：

1. 重新确认当前 Context 为已识别的 Web Context；
2. 读取一次当前 CSS viewport 及其 `scrollX`/`scrollY`；
3. 重新读取 WebDriver Element Rect；
4. 将文档坐标 Rect 平移为 viewport-relative CSS Rect，并计算二者的正面积
   交集；
5. 在交集内按既有比例规则选择整数 CSS viewport 坐标；
6. 复用根包现有 W3C Actions 执行链发送该坐标。

`Point` 在 Web Context 中表示 CSS viewport 坐标；客户端不应用
`devicePixelRatio`、原生 status bar、orientation 或 `PixelRect` 偏移。现有
Actions 使用的 pointer 类型和请求路径不因 Context 自动切换；某个浏览器或
Driver 需要另一种输入源时，必须由单独的能力设计和兼容性验证解决，不能偷偷
改为 Element Click、JavaScript `click()` 或其他 fallback。当前页面滚动、缩放、
DOM 重排或 Context 变化可能发生在 Rect 与 Actions 之间；SDK 不重试、不恢复，
也不承诺点击一定命中元素。

`Session.Tap`、`LongPress` 和 `Swipe` 不读取 `ViewportRect`，也不为获得坐标
额外探测 Context。调用方负责在当前 Context 中提供正确单位的 `Point`；Native
行为保持不变。Find 成功同样不代表后续 Tap 可以使用旧坐标，每次 Tap 都重新
获取所需几何状态。

### 7.4 Element Screenshot

Element Screenshot 采用标准远端 Element Screenshot 语义，并提供内存返回和
`io.Writer` 两种交付方式。

该能力不承诺：

- 自动滚动元素；
- 自动恢复可见性；
- 自动处理 stale；
- 与完整截图按 Element Rect 本地裁剪完全一致；
- Element Rect 与截图像素直接一一对应。

### 7.5 Context API、识别和本地状态（DP-090）

DP-090 固定 Context 的协议模型和 Web 几何边界；Context 运行时方法已由
DP-091 实现。公共入口保持根包 `Session`，不创建 Context 对象、平台 Session
或第二套 Client：

```go
func (s *Session) Contexts(ctx context.Context) ([]string, error)
func (s *Session) CurrentContext(ctx context.Context) (string, error)
func (s *Session) SwitchContext(ctx context.Context, name string) error
```

上述方法只使用 Appium 3 当前注册的 Context 路由：`Contexts` 为
`GET /session/{sessionId}/contexts`，`CurrentContext` 为
`GET /session/{sessionId}/context`，`SwitchContext` 为
`POST /session/{sessionId}/context`。不改写或自动 fallback 到替代路由。

Context 名称是远端定义的开放 UTF-8 字符串。读取结果保留原始字符串、顺序和
重复项，不 trim、不做大小写折叠、不补前缀、不按列表位置推断类型。空字符串
如果由远端返回也按字符串事实保留；它不能被识别为可用的 Native/Web Context。
切换请求只发送调用方给出的确切名称（非法 UTF-8 在编码前按本地参数错误拒绝），
是否存在、是否可切换和切换后页面是否可用由远端决定。

识别规则是区分大小写且有界的本地分类：

| Context 名称 | 分类 | 几何策略 |
|---|---|---|
| 精确 `NATIVE_APP` | Native | Window Rect 与 Native Element Rect 交集 |
| 精确 `WEBVIEW`、`WEBVIEW_` 加非空后缀或精确 `CHROMIUM` | Web | CSS layout viewport、滚动平移后的 DOM Element Rect 交集 |
| 其他名称 | Unknown | Context API 可读取；组合 Find/Tap 返回 `CodeUnsupported` + `DeliveryNotSent`，不自动选择策略 |

分类只用于选择已经设计的几何路径，不是 Runtime Discovery、Capability 或
Driver 成功保证。客户端不根据 `automationName`、`Contexts` 返回顺序、页面源、
窗口句柄、Bundle ID 或 Host 工具猜测 Web Context；Hybrid 中有多个 WebView
时也不自动选择“第一个”或回退到 Native。

Session 不保存 `currentContext`、Context 列表或 Context 到 Element 的绑定缓存。
`Contexts` 与 `CurrentContext` 每次都是远端快照；`SwitchContext` 的成功只表示
该次切换命令收到成功响应，不为后续命令建立永久本地状态。切换失败或
`DeliveryUnknown` 时客户端不回滚、不重放，也不猜测远端是否已经改变 Context；
调用方需要时可显式再次读取 `CurrentContext`。已有 Element 句柄不会被本地批量
标记 stale、重新定位或绑定到新 Context；在错误 Context 中使用时由远端返回
stale、no such element 或其他真实错误。

Context 切换、页面导航、orientation、系统栏和浏览器 UI 都可能使之前的 Rect、
viewport 或 Screenshot 快照失效。SDK 不为这些命令建立 Session 级串行器，也不
保证 Find/Rect/Actions/Screenshot 来自同一设备帧。需要稳定状态的调用方必须在
上层自行调度并保留命令顺序。

#### 7.5.1 Hybrid 与 Safari 的验证边界

Web Context 的协议设计不扩大真实环境兼容承诺。每个组合都要分别记录并验证：

- Appium 3、XCUITest 或 UiAutomator2 Driver、WDA/UiAutomator2 Server 版本；
- iOS/Android 版本、真机或模拟器、Safari/WebKit 或嵌入式 WebView/Chrome 版本；
- WebView 调试能力、Chromedriver 与 WebView 的匹配关系、相关 capability/setting；
- Context 列表和切换结果、CSS viewport 尺寸、页面滚动、orientation、缩放、
  status bar/键盘状态以及 Actions 点击结果；
- Appium Host OS、连接方式和截图采集路径。

iOS Safari 和 WKWebView 可能受 Web Inspector、WDA、Safari/WebKit 版本及
Host 条件影响；Android WebView/Chrome 可能受 UiAutomator2、Chromedriver 和
WebView 版本匹配影响。SDK 不安装、启动或探测这些组件，也不直接调用
`xcodebuild`、`simctl`、`adb`、`chromedriver` 或其他 Host 工具。iOS 17+ 是
XCUITest 主线，低版本仍按 Legacy Lane；macOS、Windows、Linux 以及任何具体
Safari/Android 版本只有在 `docs/compatibility.md` 记录真实结果后才可称为
`Verified`。当前 DP-090 不写入任何未经实测的兼容性结论。

### 7.6 Keyboard 状态与关闭请求（DP-100）

DP-100 只定义根包 `Session` 上的键盘语义，DP-101 才加入运行时代码。这里的
Keyboard 仅指当前 Driver 能观察或尝试关闭的软键盘；它不等同于 IME 的安装、选择
或配置，也不覆盖硬件键盘、文本输入、特殊键发送或截图像素中的键盘区域。

目标公共入口为：

```go
func (s *Session) KeyboardShown(ctx context.Context) (bool, error)
func (s *Session) DismissKeyboard(ctx context.Context) (bool, error)
```

两个入口都属于根包 `Session`，不创建平台 Session 或独立 Client。它们只使用
Appium 3 common command，并进入根包统一 HTTP 执行链。

两个命令的本地 `Error.Operation` 和 Observer `Started`/`Finished` identity 固定为
`keyboard_shown` 与 `dismiss_keyboard`，不随 wire route、Driver 名称或实现文件名
变化。

`KeyboardShown` 是一次不缓存的远端状态快照：

- 每次调用只发送一次 `GET /session/{sessionId}/appium/device/is_keyboard_shown`；
- `true` 只表示 Driver 在该次探测中报告键盘显示，`false` 只表示 Driver 在该次
  探测中报告未显示；两者都可能在响应后立即失效；
- `false` 是调用方能够取得的“当前未显示”快照，不是对屏幕像素或下一条命令的
  绝对保证。某些 Driver 会把内部查找失败也折叠为 `false`，SDK 不替它恢复或
  推断更强的事实；
- 客户端不预先读取 Context、Healthy、Discovery 或其他状态，也不切换 Context。

`DismissKeyboard` 是一次关闭请求，而不是关闭状态的断言：

- 每次调用只发送一次 `POST /session/{sessionId}/appium/device/hide_keyboard`，请求体
  固定为 JSON object `{}`；
- 不从 SDK 接受或发送 `strategy`、`key`、`keyCode`、`keyName`，也不发送 Back、
  Escape、tap-out、swipe、JavaScript click 或其他本地 fallback；Driver 内部采用的
  机制仍是远端实现事实；
- Appium 类型契约的成功 value 是 JSON boolean。SDK 严格校验该类型，并将原始
  Driver-reported boolean 作为第一个返回值交给调用方；`true` 和 `false` 都是已收到
  成功响应的情况。`false` 可能表示已经隐藏或没有发生可报告的转换，不能被提升为
  跨 Driver 的失败或最终状态；当 `error` 非 nil 时，第一个返回值是零值，调用方不得
  使用它推断远端状态；
- 成功只表示该次关闭请求得到成功响应，不表示键盘已经关闭。SDK 不自动等待、
  后置探测、轮询、重试、缓存结果或串行化 Session 命令。

需要确认关闭时，调用方必须显式分开两个动作：

```text
driverReported, err := DismissKeyboard(ctx)
    -> err == nil 时保留 Driver-reported boolean；仍只说明请求完成
KeyboardShown(ctx)
    -> false 才是该时刻 Driver 报告的未显示快照
```

第二次读取失败、返回 `true` 或在读取前发生状态竞争时，SDK 不把请求结果改写
为“已关闭”；调用方可以自行决定是否使用 `wait` 包进行有界的显式轮询。`Delivery`
仍只描述每个独立命令的投递事实。

关闭请求可能产生超出键盘可见性之外的应用副作用：Driver 内部的 Done、ESC 或
BACK 可能触发编辑器提交/Return 处理、页面导航、对话框关闭或其他应用逻辑；在
键盘状态发生竞争时也可能作用于已经重新获得焦点的页面。该能力只承诺发起一次
Driver 关闭请求并报告其响应，不承诺应用状态只发生键盘变化，也不提供事务回滚或
副作用隔离。

#### 7.6.1 两个目标 Driver 的状态差异

下表描述当前常见 Driver 实现的观测路径和已知局限，属于设计输入而不是对所有
版本组合的兼容性承诺。SDK 不直接调用这些 Driver 内部端点或 Host 工具。

| Driver | `KeyboardShown` 的典型来源 | `DismissKeyboard` 的典型行为 | 语义局限 |
|---|---|---|---|
| XCUITest | 查找 `XCUIElementTypeKeyboard`；找到时报告 `true`，未找到时报告 `false` | common `hide_keyboard` 通常转发到 WDA 的键盘 dismiss；部分版本会在候选键中加入 `done`，并在请求完成后返回 `true` | 查找异常可能被 Driver 折叠为 `false`；Done 可能触发应用的 Return/提交处理；返回 `true` 不构成独立的关闭后探测 |
| UiAutomator2 | 读取 Android 输入法服务报告的显示状态（常见实现使用 `mInputShown`） | Driver 可能先读取状态，再由内部平台机制发送 ESC/BACK 并等待消失；已隐藏时可返回 `false`，无法隐藏时可返回远端错误 | 输入法状态不等同截图像素；ESC/BACK 可能导航或改变应用状态；返回 `false`/`true` 的转换和等待范围受 Driver 版本影响 |

因此两者的布尔响应不能在 SDK 中统一解释为“已关闭”。即使 Driver 内部使用了
特殊键或等待，SDK 也不复制该策略、不依赖 `adb`/WDA，不提供备用路径。

当前源码基线中，Appium 3.6.0 的 XCUITest Driver 12.1.0 仍注册 common
`hide_keyboard` route，但该 Driver 方法已标记 deprecated。这只是上游路由存在性的
观察，不是本项目的真实设备兼容性结论；若后续版本移除或拒绝该 route，SDK 只返回
统一的远端 unsupported/command error，不改走 `mobile:` 或 WDA 内部 fallback。

#### 7.6.2 能力边界与验证

以下情况不由该能力掩盖或自动修复：

- 没有 Done/关闭按钮、键盘布局自定义、硬件键盘、第三方或 OEM 输入法、焦点
  丢失、动画尚未结束、键盘被应用重新打开以及 Driver/WDA/UiAutomator2 Server
  的版本差异；
- 当前 Context、前台 App、窗口状态和页面状态由调用方负责；SDK 不自动切换
  Context、恢复输入焦点或确认文本仍存在；
- SDK 不管理 IME capability/setting（包括输入法选择、启停和 `hideKeyboard`
  capability），不发送特殊键，也不把关闭失败转成 Back 或点击空白区域；
- 调用方必须把 Driver 可能发送 Done/ESC/BACK 所带来的提交、Return、导航或其他
  应用副作用纳入自己的断言和恢复边界；SDK 不承诺只改变键盘状态；
- 不维护键盘状态缓存，不在并发调用之间建立 Session 级锁，不自动重试或重建
  Session。`DeliveryUnknown` 时不重放关闭请求，也不猜测远端是否已经执行。

真实 Appium、Driver、WDA/UiAutomator2 Server、设备 OS、设备类型和 Host 组合仍
需单独记录在 `docs/compatibility.md`；DP-100 不产生 `Protocol` 或 `Verified`
证据。

## 8. 坐标与视觉产物

项目明确区分 Native WebDriver 几何、Web DOM/CSS 几何、Driver 像素几何和具体
图像产物：

```text
WebDriver 几何
    Rect / Point
    Window Rect
    Element Rect
    W3C Actions

Web DOM/CSS 几何
    CSS layout viewport
    DOM Element Rect（文档坐标，仍使用 Rect）
    CSS viewport Rect/Point（仍使用 Rect/Point）

Driver 像素几何
    PixelRect
    ViewportRect

具体图像产物
    Screenshot 自身的解码像素平面
    OCR / CV
```

W3C Touch Actions 的 `TouchAction` 只能由根包构造函数创建；其零值表示无效
动作，客户端在本地拒绝，不将其解释为坐标 `(0, 0)` 的移动。这样可以避免
调用方遗漏初始化时产生未预期的输入副作用。

这些概念不能通过相同 Go 类型混用。`PixelRect` 只承载 Driver 报告的整数像素
几何，不标识或自动绑定某一次 Screenshot 的解码像素平面。

Driver 返回的 scale、status bar、orientation 等只作为事实。除 Web Context 内按
同一快照应用 `scrollX`/`scrollY` 的 CSS 文档到 viewport 原点平移外，没有完整且
经过验证的转换模型前，SDK 不执行隐式缩放、设备像素偏移或方向修正。

`ViewportRect` 属于 Driver 像素几何，不替换 `WindowRect` 参与 Element 查找或
Actions，也不自动成为任一 Screenshot 的 crop rectangle。

`DP-060` 已将该边界细化在 [`docs/coordinate-system.md`](coordinate-system.md)：
`Rect`/`Point` 保持 WebDriver 几何语义，`PixelRect` 使用 Driver-reported integer
pixel geometry 语义；
XCUITest 和 UiAutomator2 的 `mobile: viewportRect` 结果按各自 Driver 的事实
承载，不由客户端再次应用 scale、density、status bar 或 orientation 变换。
`Session.ViewportRect` 由 DP-061 通过根包统一 Execute Script 链实现；SDK 每次
发起读取且不缓存返回值，严格校验非负原点、正面积、整数表示和端点溢出。Driver
内部可能缓存基础屏幕事实，刷新时机以远端实现为准。该结果不会进入现有 Native
Find/Tap，也不为 Web Context 的 DOM/CSS 几何或具体 Screenshot 像素平面提供
等价保证；Web Context 的 CSS layout viewport 与滚动策略已由 DP-090 在 §7.2、
§7.3 和 `docs/coordinate-system.md` §2.3 单独定义。能否用于裁剪属于带环境、
Context 和采集路径条件的兼容性事实。

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

当前 Session Screenshot 和 Element Screenshot 使用
`Limits.MaxScreenshotResponseBytes` 作为独立资源上限，同时约束 HTTP 响应体
读取和 Base64 解码后的截图数据。两者的便捷方法都通过对应的 `ScreenshotTo`
复用完整解码路径；Session 与 Element 还共用同一套流式解码和输出错误映射。

资源配置只在对应能力实现时加入公共 `Limits`，不提前暴露没有实际效果的字段。

## 10. Logs 与 WebDriver BiDi

### 10.1 Pull Logs

Pull Logs 是 Session 主动发起的一次性批量读取。它通过 Appium 3 的标准
Selenium 日志路由取得远端 Session 在调用时刻返回的快照，不是客户端自己的
Observer 日志，也不是持续事件流。所有请求都进入根包统一 HTTP 执行链；平台包
不创建独立 Client、Session 或 HTTP 通道。

#### 10.1.1 公共类型和入口

DP-081 已实现以下根包类型和 Session 方法：

```go
type LogType string

type LogEntry struct {
    Timestamp int64
    Level     string
    Message   string
    Extra     map[string]any
}

func (s *Session) LogTypes(ctx context.Context) ([]LogType, error)
func (s *Session) Logs(ctx context.Context, logType LogType) ([]LogEntry, error)
```

`LogType` 是开放的 Go `string` 标识符。可用集合是每次远端读取的动态快照，
可能受 Driver、Capabilities、当前 Context 以及其他 Session 状态影响；客户端不
把它固化为由 Driver 或 Capability 单独决定的稳定枚举。`LogTypes` 返回远端原始
顺序，数组中的每个合法 UTF-8 string（包括空字符串）都必须原样保留；空数组是
合法的非 nil 空 slice，重复项也不由客户端去重。客户端不根据平台或 Runtime
Discovery 推断可用类型，不做大小写折叠、trim、别名转换或其他协议值规范化。`Logs`
对合法 UTF-8 `LogType` 的内容不做本地业务拒绝，所有此类字符串（包括空字符串、
大小写或空白）原样放入 `type` 请求字段；未知类型或空字符串是否支持由远端决定。
Go string 中的非法 UTF-8 无法无损编码为 JSON string，因此在发送前返回
`CodeInvalidArgument`/`DeliveryNotSent`，而不是静默替换字符。

例如同一 Driver 在 Native 与 Web Context 下可能返回不同的 Log Type 集合，或
根据 Session 内其他状态改变 getter 的结果；这类差异由远端快照表达，SDK 不合并、
补齐或提前推断。

该开放值域是读写入口之间的共同不变量：`LogTypes` 返回的任一合法 UTF-8 值（包括
空字符串）都可以不经修改直接传给 `Logs`，SDK 不制造业务白名单或格式之外的本地拒绝条件。

`LogEntry` 的三个标准字段均为必需字段：

| 公共字段（Wire key） | 公共类型 | Wire 语义和边界 |
|---|---|---|
| `Timestamp` (`timestamp`) | `int64` | Unix epoch 毫秒；必须是有限、可无损转换为 `int64` 的 JSON 整数，允许零和负值，不截断小数或猜测其他单位 |
| `Level` (`level`) | `string` | Driver 提供的原始级别文本；不建立固定级别枚举、不改大小写，空字符串也按远端事实保留 |
| `Message` (`message`) | `string` | Driver 提供的原始消息；空字符串合法，不在 SDK 内拆行、解析或脱敏 |

选择 `int64` 而不是 `time.Time` 是为了保留协议的原始毫秒事实。该字段没有
时区信息，调用方如需时间对象可显式使用
`time.UnixMilli(entry.Timestamp).UTC()`；SDK 不在解码时引入本地时区、纳秒
精度或时钟校正。超出 `int64` 范围、非整数、`null`、非有限值或其他 JSON
类型都属于响应格式错误。

`Extra` 保存 Entry 中除 `timestamp`、`level`、`message` 之外的未知字段。未知
字段的名称和值按 JSON 结构递归保留，不参与过滤、排序、合并或业务解释；未知
对象、数组和 `null` 均可保留。没有未知字段时 `Extra` 为 nil。实现必须对
`Extra` 以及其中的 map/slice 建立独立副本，调用方修改一个返回快照不能影响
同一次读取的其他 Entry 或后续调用。未知字段不能替代缺失的标准字段；未知
字段的数字应使用 `json.Number` 或等价的无损表示，不能静默转换为精度不足的
`float64`；非法 JSON、非法 UTF-8 或无法解码的值使整个读取失败。

Entry 数组和数组内 Entry 保持远端顺序，不按时间排序、不合并、不去重。`LogTypes`
的成功 value 必须是 JSON string array；`null`、object、string、数组中的非 string
项和其他类型均无效。`Logs` 的成功 value 必须是 JSON array；`null`、object、string
和其他类型均无效。空数组是
合法结果。解码采用整体成功语义：任一 Entry 缺字段、类型错误或数值越界时，
整个调用返回 `CodeResponseInvalid`/`DeliveryAcknowledged`，不返回已经解码的
部分 slice。

#### 10.1.2 HTTP 契约和资源边界

Appium 3 的标准路由和成功值如下；`/se/` 是路由的一部分，不回退到历史或
Driver 专用的 `/log` 别名：

| API | HTTP | 路径 | 请求体 | 成功 value |
|---|---|---|---|---|
| `Session.LogTypes` | GET | `/session/{sessionId}/se/log/types` | 无 | JSON string 数组 |
| `Session.Logs` | POST | `/session/{sessionId}/se/log` | `{"type":"<LogType>"}` | JSON `LogEntry` 数组 |

Session ID 按统一 Endpoint 规则作为独立路径段转义。GET 不带 body，也不发送
`Content-Type`；POST 始终发送包含 `type` 的 JSON object。通过本地校验后，每次
方法调用只发起一次对应 HTTP 请求，不隐式调用 `LogTypes`、Discovery、`Healthy`
或其他命令。

Pull Logs 使用独立的 `Limits.MaxLogResponseBytes` 资源类别，默认值为
`32 << 20`（32 MiB）；字段为零时采用该默认值，负数配置无效。该上限
按单次调用应用于完整 HTTP 响应体（包括 envelope），在传输读取边界执行；响应
超过上限时整体返回 `CodeResponseTooLarge`/`DeliveryAcknowledged`，不截断并且
不返回部分 Entry。`LogTypes` 和 `Logs` 都使用这一上限；它是每次读取的上限，
不是 Session 累积配额，也不等价于 Entry 数量上限。解码、未知字段复制和结果
构造过程仍需持续检查调用方 context。

调用方 context 在发送前结束、其他本地参数/请求构造失败时，沿用统一的
`CodeInvalidArgument` 或 context 错误与 `DeliveryNotSent` 语义；已尝试
请求但没有响应时是 `DeliveryUnknown`；收到响应后无论远端错误、格式错误还是
超限均是 `DeliveryAcknowledged`。Pull Logs 不新增专用错误码，不因读取看起来
只读而自动重试。

#### 10.1.3 Driver 的消费和缓存语义

客户端不维护 Log Type、Entry、时间戳或远端缓存的本地副本，也不保存上次读取
位置、游标或水位。每次 `Logs` 调用只报告这一次远端响应；连续调用可能得到
空集合、相同 Entry、仅新增 Entry、重叠 Entry 或 Driver 定义的其他结果，均不
由 SDK 重新解释。Appium Driver 的 log getter 可以排空、截断、重置或保留其
缓存，但这些行为是 Driver-specific 事实而不是公共 API 保证；读取是否消费
缓存必须在具体 Driver/版本/设备组合的 `docs/compatibility.md` 记录。

因此 SDK 不做以下事情：

- 不把“读取成功”解释为“远端缓存已清空”；
- 不在 `DeliveryUnknown` 后重放读取，避免重复消费或改变 Driver 缓存状态；
- 不自动轮询、分页、合并、按时间过滤、去重或补齐缺失 Entry；
- 不因 Log Type 曾经出现在 `LogTypes` 中就对后续 `Logs` 调用做本地门禁；也不因
  LogType 为空而在本地拒绝。只有无法无损编码为 JSON string 的非法 UTF-8 值会在
  发送前被拒绝。

同一 Session 的并发 Pull Logs 不由 SDK 串行化；调用方需要稳定顺序或可审计的
消费点时，必须在上层自行调度并保存读取时间、Log Type 和返回快照。

#### 10.1.4 Writer 形式的取舍

当前不提供 `LogsTo(io.Writer)` 或写入 JSONL 的变体。Pull Logs 的公共结果是
经过校验的结构化 Entry 集合，不是二进制产物：

1. 要保证标准字段、时间范围和未知字段完整性，解码必须在交付前完成整体校验；
2. 直接把原始 JSON 写入 Writer 会暴露未定义的序列化/Raw Command 契约，并可能
   在发现后续 Entry 非法时留下无法标识为成功的部分输出；
3. 把结果重新编码为 JSON array 或 JSONL 会引入新的字段、时间和错误格式，不能
   仅视为传输层优化；
4. `MaxLogResponseBytes` 已为当前批量读取限定响应和结果规模，调用方可以在
   收到 `[]LogEntry` 后按自己的持久化格式写入 Writer。

若未来确有低峰值内存或逐条背压需求，应另行设计带明确交付格式和进度/错误
语义的迭代器或流；不得把它偷偷扩展为本能力的 Writer 或 Streaming Logs。

#### 10.1.5 DP-081 实现和验证记录

DP-081 已在根包 `logs.go` 中复用统一命令链，覆盖：

- 精确的 `/se/log/types` 与 `/se/log` 方法、路径、请求体和 Content-Type；
- 合法 UTF-8 的开放 Log Type（包括空字符串透传）、远端对未知类型或空字符串的实际结果和每次只发一个请求；非法 UTF-8 在发送前按本地参数错误拒绝；
- 空数组、顺序/重复项、标准字段、Unix 毫秒边界和未知字段深拷贝；
- 缺失/null/错误类型/小数/越界时间戳、非法 Entry 和整体无部分结果；
- `MaxLogResponseBytes` 的零值默认、负数配置、边界超限和 Delivery；
- context 取消、传输/远端错误以及不自动重试、不缓存、不轮询、不合并、不去重。

真实 Driver 的缓存消费、受 Driver/Capability/Context/Session 状态影响的 Log Type
集合和 Host/版本组合仍需单独验证并登记；
协议测试通过不会把 `LOG-001/002` 标成 `Verified`。

本设计明确排除 Streaming Logs、`/appium/events` 事件历史、BiDi、平台专用日志
包装、Host 工具采集、Raw Command API 和 Writer/JSONL 交付。

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

Observer 回调是同步调用：`OnCommandStarted` 在传输开始前执行，
`OnCommandFinished` 在命令链结束时执行。正常返回的回调不参与命令结果决策，
但回调耗时会影响调用方看到的 API 延迟；回调 panic 按 Go 调用栈传播，客户端不
提供异步队列、背压或 panic recovery。由于一个 `Client` 可以被多个 goroutine
共享，Observer 实现必须自行保证并发安全，并保持回调快速、非阻塞。

## 11. 显式等待

`wait` 包建立在根包公共 API 上，不进入传输层，也不修改 Session 配置。

DP-070 的最小公共契约为：

```go
func Until(
    ctx context.Context,
    interval time.Duration,
    condition func(context.Context) (done bool, err error),
) error
```

`Until` 先立即调用一次条件。条件返回 `false, nil` 表示继续，返回
`true, nil` 表示成功，返回非 nil error 表示失败并立即结束；条件错误原样
交还调用方。条件返回时若 context 已结束，context 结果优先于成功结果。
`interval` 必须为正数，并用于控制未完成检查之间的等待。
调用方 context 是唯一的总期限来源；等待间隔期间会响应取消，条件函数也会
收到同一个 context，并负责让自身执行遵守该 context。`Until` 不为条件另建
goroutine 强制中断，也不执行远端命令、不改变 Implicit/Command Timeout，
不对条件错误做自动重试或 Session 恢复。

DP-071 在此基础上提供两个查找专用 helper：

```go
func Element(
    ctx context.Context,
    interval time.Duration,
    finder interface {
        Find(context.Context, appium.Locator) (*appium.Element, error)
    },
    locator appium.Locator,
) (*appium.Element, error)

func Elements(
    ctx context.Context,
    interval time.Duration,
    finder interface {
        FindElements(context.Context, appium.Locator) ([]*appium.Element, error)
    },
    locator appium.Locator,
) ([]*appium.Element, error)
```

`*appium.Session` 与 `*appium.Element` 都可以作为 `finder`，分别保留
Session 级和 Element 级查找作用域；满足相同方法签名的本地实现也可以作为
结构化 finder。两种 helper 都先立即调用一次公共 Find API；`Element` 在得到
非 nil 引用时成功，`Elements` 在得到非空集合时成功。根包 Find API 返回的
非法响应继续使用根包的 `CodeResponseInvalid`/Delivery 语义；本地 finder 返回
nil 成功值或包含 nil 元素的集合则是 finder 契约错误，不伪造远端错误码或
Delivery 状态。
空集合和 `CodeElementNotFound` 是唯一允许继续轮询的未找到结果；其他错误
立即原样返回。helper 不直接访问传输层、不增加新的命令、不修改 Session
Timeout，也不保存 Locator 或恢复 stale 引用。

当 `Element` 或 `Elements` 已记录暂态未找到错误，而 context 在下一轮前或
最后一次 Find 调用内部结束时，helper 保留 context 结果并通过多错误链保留
最后一次查找错误。context 命令错误排在主错误位置；`errors.Is` 可判断
`context.Canceled`/`context.DeadlineExceeded`，`appium.IsErrorCode` 会遍历
错误树读取 context 与未找到两个错误码，`appium.DeliveryOf` 报告主错误的
Delivery。若 `Elements` 的所有轮询都只是空集合，则没有可保留的根包错误，
直接返回 context 结果。

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

- WebDriver BiDi 公共订阅接口及背压模型；
- Streaming Log 与平台监控事件的公共/平台类型边界；
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
| AD-025 | Accepted | ViewportRect 使用独立的 Driver 像素几何 `PixelRect`；该类型不绑定具体 Screenshot buffer，Driver-specific 的 orientation、status bar、scale 只作为事实，不执行隐式转换，也不改变 Native Find/Tap | 防止 WebDriver、Driver pixel geometry 与具体图像平面混用，并把 Screenshot 裁剪关系留给显式兼容性验证 |
| AD-026 | Accepted | Pull Logs 对合法 UTF-8 `LogType` 使用完全开放透传（包括空字符串），非法 UTF-8 在 JSON 编码前拒绝；并使用严格标准 `LogEntry`；可用类型作为受 Driver/Capability/Context/Session 状态影响的动态快照；时间戳保留有符号 Unix 毫秒 `int64`，未知字段递归放入独立 `Extra`；每次读取有界且不缓存、不重试、不提供 Writer | 保留远端日志事实，避免把消费语义、序列化格式或持续订阅隐式加入批量读取 API |
| AD-027 | Accepted | 高级 Execute Script 入口使用根包固定路由；`operation` 是调用方提供且符合 `[a-z][a-z0-9_]{0,63}` 的本地诊断 identity，不开放任意 Method/Route | 允许平台和高级调用方保留低基数错误/Observer identity，同时限制可观测标签污染 |
| AD-028 | Accepted | 平台强类型 Execute Method 的 `value` decoder 必须在统一 `executeCommand` decoder slot 中运行，并在 `Observer.OnCommandFinished` 前完成 | 调用方错误与 Observer 保持相同的 Code、StatusCode、Delivery 和 operation，禁止执行链外的业务响应校验 |
| AD-029 | Accepted | Context 名称按不透明 UTF-8 字符串快照处理；仅精确 `NATIVE_APP`、`WEBVIEW`、带非空后缀的 `WEBVIEW_` 或精确 `CHROMIUM` 选择已定义几何策略；Context API 使用 Appium 3 当前注册的裸 `/context(s)` 路由，不改写或 fallback 到替代路由；Unknown 组合 Find/Tap 为 `CodeUnsupported` + `DeliveryNotSent`；Web 使用 CSS layout viewport，Session 不缓存 Context，不自动滚动、fallback、重定位或执行像素转换 | 让 Native 与 Web 的 Find/Tap 坐标语义可区分且可验证，同时保留 Hybrid、Safari、Driver-specific Context 的真实差异，并避免把前置探针的 Delivery 误归因给未发送的主体操作 |
| AD-030 | Accepted | Keyboard 状态读取与关闭请求使用根包 `Session` 的 Appium common routes；固定 `keyboard_shown` / `dismiss_keyboard` identity；关闭返回原始 Driver-reported boolean 但只表达一次请求，最终状态须由调用方显式再次读取；不提供特殊键、IME 管理、自动 fallback、轮询、重试或状态缓存 | 隔离 XCUITest 与 UiAutomator2 的探测/关闭差异，保留可观察响应并避免把该响应或单次 `false`/`true` 误当成跨 Driver 的确定事实，同时暴露 Driver 内部按键可能造成的应用副作用 |

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
