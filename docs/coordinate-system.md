# WebDriver、Driver 像素几何与截图像素平面

> 文档状态：Active
> 适用阶段：v0.x 至首个稳定版本
> 技术基线：Go 1.26.5，Appium 3.x
> 对应设计项：`DP-060`、`DP-090`
> 最后更新：2026-09-02

本文档是 SDK 坐标空间的主要事实源。它先于 `VIS-006` 和 `CTX-001` 的运行时代码，
固定 `Rect`、`Point`、`PixelRect` 和 `ViewportRect` 的边界；不把不同 Driver 当前
实现中可以观察到的换算步骤变成客户端的隐式行为，也不把 Driver 报告的像素
几何自动绑定到某一次 Screenshot 的解码图像。

## 1. 结论

SDK 区分四个不能自动等同的概念：

| 概念 | 公共表示 | 单位和原点 | 主要用途 |
|---|---|---|---|
| Native WebDriver 几何空间 | `Rect`、`Point` | Native Driver 的 WebDriver 坐标单位；原点和轴方向由 WebDriver viewport 定义 | Native `WindowRect`、`Element.Rect`、W3C Actions |
| Web DOM/CSS 几何空间 | `Rect`、`Point` | 文档坐标和 viewport-relative 坐标均使用当前 Web browsing context 的 CSS pixel；文档原点与 viewport 原点通过 `scrollX`/`scrollY` 连接 | Web `Element.Rect`、Web Find/Tap 的几何判断、W3C Actions |
| Driver 像素几何 | `PixelRect` | 产生该值的 Driver 命令所报告的整数像素单位和原点；类型本身不标识具体像素平面 | `ViewportRect` 等 Driver 几何事实 |
| 具体截图像素平面 | 每个解码后的 Screenshot 产物自身拥有 | 该图像缓冲区的实际像素；原点为图像左上角，X 向右、Y 向下 | 图像解码、OCR/CV，以及兼容性确认后的裁剪 |

`Rect`/`Point` 和 `PixelRect` 不可互换。`Rect`/`Point` 在 Native 与 Web Context
下仍是同一组 Go 类型，但单位由当前 Context 决定；来自不同 Context 的快照不能
混用。Web Context 内还要区分 WebDriver Element Rect 的文档坐标和 Actions 使用
的 viewport-relative CSS 坐标；二者只通过同一快照的 `scrollX`/`scrollY` 平移
连接。尤其是名称都包含 viewport 时，W3C Actions 的 viewport、Web 浏览器 CSS
viewport 与 Appium `mobile: viewportRect` 返回的 Driver 像素 viewport 仍然属于
不同空间。`PixelRect` 也不自动等于任意一次
`Session.Screenshot` 或 `Element.Screenshot` 的裁剪坐标；两者是否共享同一
具体像素平面是兼容性事实。

`VIS-006` 规划的公共入口为：

```go
type PixelRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func (s *Session) ViewportRect(ctx context.Context) (PixelRect, error)
```

上述类型和方法由 `DP-061` 实现。本设计不在 `DP-060` 提前加入运行时代码。
`Session` 仍然是唯一拥有远端 Session 的对象；不创建平台 Client、平台
Session 或独立的坐标转换器。

## 2. WebDriver 几何空间

### 2.1 `Rect` 与 `Point`

- `Rect` 的 `X`、`Y`、`Width`、`Height` 使用 `float64`，保留远端 WebDriver
  返回的连续坐标语义。Native 与 Web 的单位分别由当前 Context 的契约定义；
  元素矩形可以有小数坐标，不能据此推断 Driver 像素几何或某一张截图的像素位置。
- `Point` 的 `X`、`Y` 使用 `int`，只用于 W3C Actions 的整数指针位置。
- 两种类型都以左上角为基准，X 轴向右、Y 轴向下；矩形按半开区域
  `[X, X+Width) × [Y, Y+Height)` 理解。
- `WindowRect` 属于 Native WebDriver 几何空间；`Element.Rect` 的单位和原点由
  当前 Context 决定。Native 中它与 Window Rect 处于同一 Driver 几何；Web 中
  `Element.Rect` 的 `x`/`y` 是相对文档元素的绝对 CSS pixel 坐标，供几何操作
  使用前必须按当前滚动快照平移到 viewport-relative 坐标。`WindowRect` 的字段
  是 WebDriver 的 window/viewport 事实，不是浏览器 CSS viewport 或截图的宽高
  证明。
- `Point` 使用 `origin: "viewport"` 发送时，坐标是当前 WebDriver viewport
  坐标；客户端不会先乘以设备 scale，也不会加减 status bar 偏移。

`Rect` 的现有行为保持不变：它承载 WebDriver 的连续值，不承担 Driver 像素
几何或具体截图像素平面的边界校验；引入 `PixelRect` 不会改变其 `float64`
形态，也不会把 `PixelRect` 的单位或边界规则附加到 `Rect`。对 Native `Rect` 的
查找和点击校验仍由现有 Element 行为负责，Web `Rect` 的 viewport 交集规则见
下文 DP-090 设计。

### 2.2 现有调用的空间归属

| API/数据 | 空间 | 设计约束 |
|---|---|---|
| `Session.WindowRect` | WebDriver | 供 Native Element 查找和点击交集使用 |
| `Element.Rect` | 当前 Context 的 WebDriver 几何 | Native 返回 Driver 几何；Web 返回 WebDriver 规定的文档相对 CSS pixel Rect；不附带 Context 标签，也不自动读取滚动或 viewport |
| `Session.Tap`、`LongPress`、`Swipe` | 当前 Context 的 WebDriver viewport | 通过既有 W3C Actions 发送整数坐标；Web 调用方提供 CSS pixel，Native 行为不变 |
| Native `Element.TapInWindowIntersection` | Native WebDriver | 只计算 `WindowRect` 与 `Element.Rect` 的交集 |
| Web `Element.TapInWindowIntersection` | Web DOM/CSS | 先用同一滚动快照把 `Element.Rect` 平移到 viewport-relative CSS 坐标，再计算与 CSS layout viewport 的交集；不读取 `PixelRect` |
| `Session.Screenshot`、`Element.Screenshot` | 具体图像产物 | 每次解码结果拥有自己的像素平面；不自动绑定 `Rect` 或 `PixelRect` |
| `Session.ViewportRect` | Driver 像素几何 | 报告当前 Driver 的 viewport pixel geometry；不改变上面任何 API，也不自动成为 Screenshot crop rectangle |

### 2.3 Web Context 的 DOM/CSS 几何（DP-090）

Web Context 的几何边界以 WebDriver 和浏览器 CSSOM 的 viewport 语义为准，不借用
Native Window Rect 或 Driver 像素 viewport。本文中的 `Context` 默认指 Appium
application context；W3C `browsing context` 仅用于说明 DOM 几何所属的页面或
frame，不表示 SDK 提供 frame/window switching。Context 名称的本地识别规则为：
精确 `NATIVE_APP` 是 Native；精确 `WEBVIEW`、以 `WEBVIEW_` 开头且带非空后缀，
或精确 `CHROMIUM` 均为 Web；其他名称保持 Unknown，不触发隐式策略。
Unknown Context 的 Context-sensitive Find/Tap 在成功取得该快照后，主体操作返回
`CodeUnsupported` + `DeliveryNotSent`；成功的 `CurrentContext` 探针只保留在自身
Observer 事件中，不改变主体操作的 Delivery。

在已识别的 Web Context 中，浏览器 viewport 定义为当前 browsing context 的
**layout viewport**。一次固定的根包 Execute Script 同时读取
`window.scrollX`、`window.scrollY`、`window.innerWidth` 和
`window.innerHeight`，并在文档坐标系中构造：

```text
documentViewport = Rect{
    X:      scrollX,
    Y:      scrollY,
    Width:  innerWidth,
    Height: innerHeight,
}
viewportRect = Rect{X: 0, Y: 0, Width: innerWidth, Height: innerHeight}
```

`innerWidth` 和 `innerHeight` 是 SDK 明确定义的 layout viewport 尺寸来源（包括
浏览器对滚动条的既有处理），不是从 Native Window Rect、Screenshot 或
`document.documentElement.clientWidth/Height` 推导。四个脚本结果必须是有限数，
宽高必须为正，且文档 viewport 的右/下端点仍须有限；读取失败或返回无效几何时，
整个 Find/Tap 操作失败，不回退到 Window Rect。viewport-relative 原点固定为
页面 viewport 左上角，X 向右、Y 向下，单位为 CSS pixel。

依据 [W3C WebDriver Get Element Rect](https://w3c.github.io/webdriver/#get-element-rect)，
`x`/`y` 在该 Context 中按规范解释为相对于文档元素的绝对 CSS pixel 坐标；
`width`/`height` 是元素 bounding rectangle 的 CSS pixel 尺寸。`Rect` 的小数值
原样保留。用于几何操作时，SDK 只做同一 CSS 空间内的平移：

```text
viewportElementRect = Rect{
    X:      elementRect.X - scrollX,
    Y:      elementRect.Y - scrollY,
    Width:  elementRect.Width,
    Height: elementRect.Height,
}
```

该平移不是 device-pixel 转换；SDK 不使用 `devicePixelRatio`、display density、
原生 status bar、orientation 或 `PixelRect` 做乘除、偏移、旋转或宽高交换。若
平移后的坐标或端点不是有限值，整个操作按响应格式错误处理，不返回部分结果。
若远端 Driver 返回与 WebDriver 文档坐标契约不同的值，客户端不从数值猜测、
双重扣除或自动切换另一种解释，具体组合必须在兼容性验证中记录。

Driver、Chromedriver 或 WebView setting 可能改变 Element Rect 的坐标基准；验证
记录必须包含这些 setting。DP-091 不替调用方设置、读取或修正该状态，未确认
坐标基准的组合不能宣称 Web Find/Tap 已兼容。

浏览器的 visual viewport、pinch zoom、页面缩放、滚动条、软键盘和浏览器 UI
可能使 CSS layout viewport 与实际触控可见区域不同。SDK 不读取这些事实来构造
隐式转换，也不宣称 CSS pixel 与 Screenshot 像素或 Native touch 像素等价；相关
差异只能通过具体 Safari/WebView、Driver、设备和 Host 组合验证。`getClientRects`
的多片段、CSS transform、遮挡、`visibility`、`pointer-events`、enabled 状态和
嵌套滚动容器不额外建模，使用远端 Element Rect 的单一矩形事实。

Web Find/FindElements 在该空间内采用与 Native 相同的顺序和整体成功原则：先
取得全部候选；候选为空时直接返回合法空结果，不读取 viewport。存在候选时再
读取一次 CSS viewport，按远端顺序读取候选 Rect，将其平移为 viewport-relative
Rect 后筛选与 `viewportRect` 的正面积交集；任何候选 Rect、滚动偏移或 viewport
解码失败都不返回部分结果。该判断只证明矩形与当前 browsing context 的 CSS
viewport 有正面积交集，
不证明元素可见、未被遮挡或最终会响应触控。SDK 不自动滚动到元素、不遍历或
重算 iframe 边界、不执行 DOM 重定位脚本；调用方显式切换 browsing context 后，
仍以该上下文的远端 Rect 和 viewport 事实为准。

Web Element Tap 使用平移后交集中的整数 CSS viewport 点，并复用现有半开矩形和
比例规则。整数化只发生在发送 Actions 前；若交集不包含可表示的整数点则本地
拒绝，不发送副作用命令。W3C Actions 的请求路径和既有 pointer 类型不因 Context
自动替换；浏览器要求 mouse pointer、Element Click 或 JavaScript click 时，必须
另行设计和验证，不能在本能力中隐式 fallback。

## 3. Driver 像素几何与 `PixelRect`

### 3.1 类型语义

`PixelRect` 是一个位于 Driver 所报告像素平面中的整数、轴对齐、半开矩形。
其公共字段沿用 `Rect` 的 `X`/`Y` 命名，但语义不同：

- `X`、`Y` 是产生该值的 Driver 命令所定义像素平面中的左、上偏移，不是
  WebDriver 点或 CSS 像素，也不自动表示某个解码图像缓冲区中的偏移；
- `Width`、`Height` 是像素数量，不是右、下坐标；
- 有效区域是 `[X, X+Width) × [Y, Y+Height)`；右边界和下边界本身不属于区域；
- 零值 `PixelRect{}` 不表示一个可用 viewport。远端返回的 viewport 必须有
  正面积，无法确定有效区域时返回错误；
- 类型不携带像素平面身份、Screenshot 引用、采集路径、截图编码、颜色通道、
  orientation、scale、status bar 或 Context 信息。需要这些事实时由调用方
  单独取得并记录。

`ViewportRect` 是 Driver 在一次调用时报告的 viewport pixel geometry 快照。
它不绑定调用前、调用中或调用后的任何一次 Screenshot；即使数值能够落入某张
截图的宽高，也不能仅凭这一点证明二者共享同一像素平面。只有在同一环境、
Context、采集路径和稳定状态下完成兼容性验证后，调用方才可以把该值当作对应
Screenshot 的裁剪矩形；SDK 不保存或推断这种关联。

上述 Native Context 语义不延伸为 Web Context 的 DOM/CSS viewport 语义；
Web Context 使用本文件 §2.3 定义的 CSS layout viewport。`ViewportRect` 仍然是
Driver 像素几何，不能替代该 CSS viewport。Screenshot 的实际像素宽高和像素
平面始终以该次解码图像自身为准。

### 3.2 不做别名或单位猜测

远端 `mobile: viewportRect` 的稳定字段是 `left`、`top`、`width`、`height`。
实现将它们映射到 `PixelRect.X`、`Y`、`Width`、`Height`；不接受 `x`/`y`、
CSS 单位或其他字段作为替代，也不根据数值大小猜测单位。未知 JSON 字段不
参与几何计算。字段数值或矩形尺寸与 Screenshot 恰好相同，也不作为像素平面
等价证明。

## 4. 两个移动 Driver 的 Viewport 语义

`mobile: viewportRect` 是 Appium Execute Method，不是 W3C Window Rect 的
别名。两个目标 Driver 使用相同的公开脚本名，但内部实现和输入事实不同；
`DP-061` 必须依据创建 Session 后远端确认的 `automationName` 选择映射。

| `automationName` | Driver 端 Execute Method | Viewport 计算事实 | 返回单位 |
|---|---|---|---|
| `XCUITest` | `getViewportRect` | 以 WDA screen info 的 status bar 和 scale、以及当前 WebDriver Window Rect 计算 | Driver 计算的 device-pixel geometry |
| `UiAutomator2` | `mobileViewPortRect` | 以当前 Android window/display 尺寸和 system bars 的 status bar 高度计算 | Driver 报告的 window/display pixel geometry |

表中的公式是对设计时上游 Driver 公开实现的行为模型，用来解释返回值的单位和
边界；它们不是 SDK 端重新计算 viewport 的算法。Driver 版本、Context 或设备
状态变化后，以远端实际响应为准。公式本身也不证明该几何与任一 Screenshot
采集路径共享像素平面；真实组合的结果再登记到 `docs/compatibility.md`。

### 4.1 XCUITest

当前 XCUITest Driver 的语义可以表达为：

```text
statusBarPixels = trunc(statusBarPoints * scale)
left            = 0
top             = statusBarPixels
width           = trunc(windowWidthPoints * scale)
height          = trunc(windowHeightPoints * scale) - statusBarPixels
```

其中：

- `WindowRect`、`Element.Rect` 和 W3C Actions 仍使用 XCTest/Driver 的逻辑
  坐标单位；
- `scale` 和 status bar 高度由 Driver 从 WDA screen info 得到，viewport 返回
  值已经是 Driver 选择的 device-pixel geometry；
- status bar 不存在或当前不可见时，Driver 可能报告零高度；客户端保留该
  事实，不自行补齐安全区、刘海、Home Indicator 或其他遮挡区域；
- `trunc` 是 Driver 当前实现的行为。客户端不重新计算、四舍五入或用另一套
  公式替换远端结果。

这组事实说明为什么不能把 XCUITest 的 `Rect` 直接当作 Driver 像素几何：同一
个 Driver 的 Window/Element 坐标和 ViewportRect 坐标可能相差 scale，并且
status bar 只在像素 viewport 中表现为顶部偏移。反过来，也不能只凭该公式就
把 ViewportRect 当作任一 WDA、MJPEG 或 Web Context Screenshot 的缓冲区索引。

### 4.2 UiAutomator2

当前 UiAutomator2 Driver 的语义可以表达为：

```text
left   = 0
top    = statusBarHeight
width  = currentWindowWidth
height = currentWindowHeight - statusBarHeight
```

`statusBarHeight` 来自 Driver 的 system-bars 查询，窗口尺寸来自 Driver 的
当前 window/display 查询。它们是同一次 Driver 方法内部采用的单位；客户端
不再读取 Android density/pixel ratio 后做除法或乘法。导航栏、手势 inset、
显示 cutout 和键盘是否影响尺寸，以该 Driver 返回的值为准，不能由 SDK 猜测。

因此 Android 的 `PixelRect` 也只是 Driver 报告值的严格承载。当前 Driver 的
Viewport Screenshot 实现可能把它用于裁剪，但这不是根包公共类型的跨版本承诺；
即使某一设备上它恰好与 Screenshot 的尺寸一致，也不构成跨 Android API、导航
模式、采集路径或 Driver 版本的像素平面等价证明。

### 4.3 orientation、status bar 与 scale 的事实边界

- **orientation**：方向是独立的 Session 状态。方向改变后，轴的物理含义、
  Window 尺寸、status bar 位置或 Screenshot 方向都可能改变；`ViewportRect`
  不携带方向字段，客户端不旋转、交换宽高或移动原点。调用方需要在稳定
  orientation 下重新读取所需快照；若 Driver 对基础屏幕事实有缓存，刷新时机仍由
  Driver 决定，SDK 不作实时性保证。
- **status bar**：`top` 只表达 Driver 在该次响应中扣除的 status bar 区域。
  它不是通用的“安全可交互区域”或所有系统 UI 的总高度；状态栏可显示、隐藏
  或随系统版本改变。客户端不通过 `IOSDeviceScreenInfo`、system-bars 或
  任何默认常量覆盖 ViewportRect 的返回值。
- **scale / density**：scale、pixel ratio 和 display density 是设备事实，不是
  SDK 的转换参数。XCUITest Driver 已在其 viewport 结果中应用自身的 scale；
  UiAutomator2 Driver 的结果使用其自身报告的 display 单位。客户端不执行
  `* scale`、`/ scale`、`* density` 或 `/ density`。这些事实只解释 Driver
  几何单位，不建立与具体 Screenshot 像素平面的等价关系。

`xcuitest.IOSDeviceScreenInfo` 返回的 `Scale` 和 `StatusBarSize` 仍是独立的
Driver 事实接口。读取这些字段不会改变 `WindowRect`、`Element.Rect`、
`ViewportRect` 或 Screenshot 的单位，不会证明 ViewportRect 与 Screenshot
共享像素平面，也不会在 Session 内形成可供后续命令复用的转换状态。

### 4.4 Web Context 的方向、系统栏与滚动

Web Context 的 CSS viewport 随浏览器 layout 状态变化。orientation、Safari 或
WebView 的地址栏/工具栏、软键盘和页面滚动都可能改变 `innerWidth`、
`innerHeight`、`scrollX`/`scrollY` 或 Element Rect；每次 Context-sensitive
Find/Tap 都重新读取同一份所需快照，并在 CSS 文档坐标内应用该快照的滚动平移，
不主动滚动、不缓存偏移，也不交换宽高、旋转原点、扣除 Native status bar 或应用
安全区常量。CSS layout viewport 不承诺等于 visual viewport、触控设备像素或任一
Screenshot 平面。pinch zoom、非 1 的 `devicePixelRatio` 和浏览器 UI 偏移属于
兼容性验证维度，SDK 不因这些状态自动缩放、偏移或重试。

## 5. `Session.ViewportRect` 的调用边界（DP-061 输入）

### 5.1 公共入口和请求

`Session.ViewportRect(ctx)` 是根包 Session 方法。每次调用都通过 Client 的
统一 HTTP/Execute Script 执行链发送：

```text
POST /session/{sessionId}/execute/sync
script: "mobile: viewportRect"
args:   []
```

Session ID 和请求体沿用 `Session.ExecuteScript` 的既有契约；平台包不创建
HTTP Client、不拼接 Endpoint，也不公开 Raw Command。两个 Driver 的内部
方法名只属于映射实现，不进入公共 API。

### 5.2 Driver 门禁与快照

- 只接受创建 Session 后远端确认的精确 `automationName`：`XCUITest` 或
  `UiAutomator2`。不做大小写折叠、trim、前缀匹配或别名推断。
- 空、未初始化或仅用于清理的 Session 在发送前返回参数错误；未知 Driver
  在发送前返回不支持错误。两者都具有 `DeliveryNotSent`。
- 不调用 `Commands`、`Extensions` 或其他 Runtime Discovery 结果作为门禁、
  fallback 或成功保证；Driver 能力不足时仍返回实际远端错误。
- 每次调用都由 SDK 发起一次新的 `mobile: viewportRect` 读取；SDK 不在 Session、
  Client 或 Element 中缓存结果。这里的“新快照”不等于 Driver 内部所有事实都
  实时刷新：Driver 可能缓存 scale、density 或 system-bar 查询，SDK 不绕过或
  强制刷新这些 Driver 缓存。方向、状态栏、窗口大小和页面状态变化是否已经反映
  在响应中，以该次远端命令及其 Driver 版本的实际语义为准。
- 调用不会修改 Session、orientation 或设备状态，不启动后台 goroutine，也不
  为了获得坐标额外调用 `Healthy`、Window Rect、Host 工具或平台包。

### 5.3 Context、错误和资源

ViewportRect 复用统一命令链的 context、timeout、Error 和 Delivery 语义：

- 调用方 context 为 nil、已取消或参数/Driver 门禁失败时，不发送请求并返回
  相应参数或取消错误；
- 收到远端响应后，`value` 不是符合本文件的对象、字段缺失、类型错误或数值
  校验失败时，返回 `CodeResponseInvalid`，Delivery 为
  `DeliveryAcknowledged`，不返回部分 `PixelRect`；
- 远端 Driver 错误、Session 丢失、传输失败和 context 截止时间沿用根包现有
  映射，不新增坐标专用错误码；
- 该结果很小，使用普通命令响应上限即可，不新增截图或图像资源上限；
- 成功只表示取得一份有效的 Driver 像素几何，不表示它已与某次 Screenshot
  建立关联，也不保证可以直接裁剪该图像；
- 客户端不自动重试、fallback、恢复 Session 或把返回错误改写为“不可见”。

ViewportRect、Screenshot、WindowRect 和 orientation 不是原子事务。并发调用
时 SDK 不保证它们来自同一设备帧；需要关联证据的调用方应自行安排顺序和
稳定状态，并保留各次读取的时间与版本事实。

## 6. 数值和结构校验

`DP-061` 解码成功 value 时必须整体校验，只有全部字段有效才构造 `PixelRect`。
校验规则如下：

| 字段/条件 | 要求 |
|---|---|
| value | 必须是 JSON object；`null`、array、string 和空 value 均无效 |
| `left`、`top`、`width`、`height` | 四个字段都必须存在，且必须是 JSON number；缺失、显式 `null`、string、boolean 均无效 |
| 数值表示 | 必须是有限、可无损转换为当前 Go `int` 的整数；不截断小数、不四舍五入、不接受 NaN/Inf |
| 原点 | `left >= 0` 且 `top >= 0` |
| 尺寸 | `width > 0` 且 `height > 0` |
| 端点 | `left + width` 和 `top + height` 必须在 `int` 范围内，且严格大于各自起点 |
| 未知字段 | 可以存在但不进入 `PixelRect`，不作为已知字段的替代或补全 |

例如，负原点、零或负尺寸、小数像素、超过 `int` 范围的数字、加法溢出、
字段缺失和错误别名都属于响应格式错误。校验不得通过静默 clamp、round、
scale 或 status bar 修正来“挽救”异常值。

`PixelRect` 的端点校验只针对整数表示范围，不与另一次 Screenshot 的实际图像
宽高做隐式跨命令比较。通过结构和数值校验只证明矩形可表示且自洽，不证明它与
某张截图共享像素平面；即使矩形完全落在图像边界内也不能据此推断等价。若
`right/bottom` 超出截图边界，这是兼容性事实或时序不一致，必须由调用方在验证
流程中记录，不能由 SDK 裁剪或移动矩形。

## 7. 与 Find、Tap 和 Screenshot 的明确隔离

`ViewportRect` 不参与 Native 或 Web Context 的 Element Find/Tap：

1. Native `Session.Find`/`FindElements` 继续使用 WebDriver `WindowRect` 和
   `Element.Rect` 的正面积交集；
2. Web `Session.Find`/`FindElements` 使用 §2.3 的 CSS layout viewport、滚动
   快照和由 `Element.Rect` 平移得到的 viewport-relative Rect 做正面积交集；不
   把 `WindowRect` 当作浏览器 viewport；
3. Native `Element.Tap`/`TapInWindowIntersection` 继续在既有 WebDriver 坐标
   空间计算整数 `Point`；Web 版本先将文档 Rect 平移到 CSS viewport 空间，再计算
   整数 `Point`；两者都通过根包 W3C Actions 的 viewport origin 发送；
4. `Session.Tap`、`LongPress` 和 `Swipe` 不读取 `ViewportRect`，也不因 scale、
   orientation、status bar、density 或 devicePixelRatio 自动改写坐标；调用方
   必须按当前 Context 提供正确单位；
5. `PixelRect` 不作为 Element Rect、Window Rect、CSS viewport 或 Actions 的替代
   类型；CSS pixel 与 Driver pixel 之间没有 SDK 内置转换；
6. `Session.Screenshot` 不读取或校验 `ViewportRect` 或 CSS viewport，
   `Session.ViewportRect` 也不触发截图或返回 Screenshot 引用；
7. 只有调用方已经为同一环境、Context、采集路径和稳定状态确认像素平面兼容时，
   才能自行把 `PixelRect` 用作 crop rectangle；SDK 不自动裁剪或保存该结论。

Web Context 的 DOM rect、CSS viewport 和页面滚动只在 §2.3 定义的 CSS 空间内
解释；文档 Rect 到 viewport Rect 的滚动平移不等于 device-pixel 转换，也不提供
与设备 Screenshot 像素平面或 Native `ViewportRect` 的等价证明。

## 8. 与 Screenshot 像素平面的兼容性验证

`PixelRect` 的类型契约不包含与 Screenshot 像素平面等价的保证。DP-060 只确定
验证方法，不宣称任何真实环境已经 `Verified`。对每个要支持的组合，应在同一
稳定 UI 状态下：

1. 记录 SDK、Appium、Driver、WDA 或 UiAutomator2 Server、设备 OS、真机/模拟器
   和 Host OS 版本，同时记录当前 Context、Screenshot 采集路径及影响采集的
   capability/setting，例如 MJPEG 或 Web Screenshot 模式；
2. 固定 Context、orientation 和页面状态，并连续读取 `ViewportRect` 与
   `Screenshot`，记录命令顺序和读取时间；
3. 解码 Screenshot，取得实际像素宽高；检查 `PixelRect` 是否为非负、正面积，
   并记录 `X+Width <= imageWidth`、`Y+Height <= imageHeight` 是否成立；
4. 使用位置已知的合成视觉标记核对左上偏移和区域边界；仅尺寸相同或边界检查
   通过，不足以证明二者共享同一像素平面；
5. 在 portrait/landscape、status bar 可见/隐藏以及适用的真机/模拟器条件下
   重复，分别核对 XCUITest 的 scale/status bar 影响和 UiAutomator2 的
   system-bars 影响；
6. 若确认二者共享像素平面，只把结论应用于已记录的环境、Context、采集路径和
   状态条件；若不共享，保留原始版本和结果作为兼容性差异，并禁止把该值用于
   Screenshot 裁剪或隐式换算；
7. 将真实结果写入 `docs/compatibility.md` 后，才能把能力的验证状态改为
   `Verified`。协议测试通过本身不改变该状态。

该流程是诊断和兼容性验证，不是普通命令的运行时门禁。SDK 不持久化验证结论，
也不要求每次业务调用同时读取 Screenshot。

### 8.1 Web Context 的兼容性验证（DP-090）

Web CSS 几何与 Actions 的组合必须按具体环境单独验证，不把 Native
`ViewportRect` 的结果或单一浏览器测试外推为 Web Context 保证。每个验证记录
至少包含：

| 维度 | 需要记录的事实 |
|---|---|
| 协议与 Driver | SDK、Appium 3、XCUITest/UiAutomator2、WDA/UiAutomator2 Server、必要的 Web Inspector 或 Chromedriver 版本 |
| 浏览器/容器 | iOS Safari/WebKit、WKWebView，或 Android WebView/Chrome 的版本与调试能力 |
| 设备与 Host | 设备 OS、真机/模拟器、Appium Host OS、连接方式 |
| Context 状态 | `Contexts` 原始列表、切换结果、当前 Context、是否存在多个 WebView，以及影响 Rect 坐标基准的 Driver/WebView setting |
| 几何与动作 | `innerWidth`/`innerHeight`、`scrollX`/`scrollY`、文档 Rect 到 viewport Rect 的平移、小数/滚动 Rect、orientation、缩放、系统栏/键盘状态和 Actions 实际命中结果 |

测试应在稳定页面中分别覆盖初始加载、页面滚动（含滚动偏移平移）、部分交集、零面积、DOM 重排
以及 Context 切换后的新快照，并记录命令顺序。Safari/WKWebView 的 Web Inspector、
WDA、Safari/WebKit 和 Host 条件，Android WebView/Chrome 的 Chromedriver 匹配，
都可能改变 Context 列表、CSS viewport 或 Actions 结果。SDK 不安装、启动或探测
这些组件，也不调用 `xcodebuild`、`simctl`、`adb`、`chromedriver` 等 Host 工具。
只有将真实结果写入 `docs/compatibility.md` 的具体组合才能标记 `Verified`；
DP-090 当前不提供任何未经实测的 Safari、Hybrid、设备 OS 或 Host OS 保证。

## 9. 被拒绝的方案

- **复用 `Rect` 表示像素区域**：同名字段仍会隐藏逻辑单位、scale 和半开整数
  边界，调用方容易把 Driver 像素几何传给 Actions，因此使用独立 `PixelRect`。
- **把 `PixelRect` 定义为具体 Screenshot buffer 的坐标**：ViewportRect 与
  Screenshot 是独立命令，截图还可能来自不同采集路径或 Web Context。公共类型
  只表达 Driver-reported integer pixel geometry，不携带或推断图像关联。
- **仅凭尺寸或边界相同推断像素平面等价**：矩形落入图像边界只是必要的数值
  条件，不证明原点、方向、缩放和采集路径一致；必须通过真实兼容性验证。
- **客户端自动乘除 scale/density**：两个 Driver 的 scale 应用位置不同，且
  orientation、status bar、Web Context 和截图方向尚未形成可验证的统一矩阵；
  静默转换会把猜测伪装成坐标事实。
- **从 WindowRect、ScreenInfo 或固定状态栏常量推导 viewport**：WindowRect
  属于 WebDriver 空间，ScreenInfo 只报告辅助事实，状态栏可变；只有 Driver 的
  `mobile: viewportRect` 结果才是本能力的直接输入。
- **缓存 ViewportRect 或与 Screenshot 自动绑定**：方向、系统栏、页面状态和
  截图采集路径可在两次命令之间变化；SDK 不建立跨命令快照或串行器。
- **在本地裁剪 Screenshot 或提供 Viewport Screenshot**：不同 Driver 的图像
  尺寸、方向和像素密度尚未有统一等价承诺，Viewport Screenshot 仍是 `VIS-007`
  的 Deferred 能力。
- **把 ViewportRect 接入 Find/Tap**：会把 Driver 像素几何与 WebDriver 坐标
  或 Web CSS 几何混用，破坏现有 Native/Web 几何和 Actions 契约。
- **把 Native WindowRect 当作 Web 浏览器 viewport**：移动浏览器的 window、layout
  viewport 和 visual viewport 可能不同；必须使用当前 Web Context 的 CSS viewport，
  不从 WindowRect、Screenshot 或固定设备常量推导。
- **把 CSS pixel 自动换算为 device pixel**：同一 CSS 空间内为处理页面滚动而
  应用 `scrollX`/`scrollY` 平移是必要的坐标原点转换，但
  `devicePixelRatio`、浏览器缩放、Driver 输入映射和截图采集路径并不构成跨版本
  统一模型；SDK 不乘除比例或补偿原生 status bar。
- **为 Web 元素自动滚动、点击 fallback 或重新定位**：这些动作会改变页面状态、
  命中语义或副作用边界；调用方需显式编排，SDK 不改用 Element Click、JavaScript
  click 或隐藏的 stale 恢复。
- **使用 `adb`、`simctl`、`xctrace` 等 Host 工具补齐坐标**：超出根包协议
  边界，也不能可靠代表远端 Driver 当前 Context 和窗口事实。

## 10. 对 DP-061 的实现输入

DP-061 需要据此完成最小运行时实现和协议测试：

- 在根包定义 `PixelRect` 和 `Session.ViewportRect`，GoDoc 明确它们承载 Driver
  报告的整数像素几何且不绑定具体 Screenshot，不增加平行平台对象；
- 以确认的 `automationName` 做精确 Driver 映射，未知值本地拒绝；
- 通过 `ExecuteScript` 的统一执行链发送 `mobile: viewportRect`，参数为空数组；
- 严格解码 `left`/`top`/`width`/`height` 并执行本文件的整数、范围和溢出校验；
- 覆盖 XCUITest、UiAutomator2、未知 Driver、取消/未发送、远端错误和非法
  数值响应；
- 保持现有 WindowRect、Find、Element Rect、Tap、Actions 和 Screenshot 行为
  不变，不加入 Screenshot 引用、隐式转换、缓存、重试或本地裁剪。

DP-060 本身不新增 Go 文件、公共 API、依赖、真实设备兼容性结论或自动坐标
转换。

## 11. 对 DP-091 的实现输入

DP-091 需要在根包统一执行链中实现 Context 命令和本设计确定的几何策略：

- 提供 `Session.Contexts`、`Session.CurrentContext` 和 `Session.SwitchContext`，
  分别使用 Appium 3 正式路由 `GET /session/{sessionId}/appium/contexts`、
  `GET /session/{sessionId}/appium/context` 和
  `POST /session/{sessionId}/appium/context`；严格解码 string/array/null，并
  原样保留 Context 名称、顺序和重复项。旧 MJSONWP `/context(s)` 路由不作为
  请求目标，也不增加兼容 fallback；
- 以当前远端 Context 的精确名称选择 Native、Web 或 Unknown；不依赖
  Runtime Discovery、Capability、列表顺序或自动 Context fallback；
- Native Find/Tap 的既有 Window Rect 交集、整数点和 Actions 请求保持不变；
  Web Find/Tap 使用一次 CSS layout viewport 探针（`window.scrollX` /
  `window.scrollY`、`window.innerWidth`/`window.innerHeight`），将 WebDriver
  文档相对 Element Rect 平移到 viewport CSS pixel 后计算交集；
- 对 viewport/Rect 解码执行整体成功校验，页面滚动、缩放、DOM 重排和 Context
  竞争时不缓存、不重试、不返回部分结果；Unknown Context 不发送替代几何请求；
- Context 切换不批量失效或重定位 Element，不维护 Session 级 current-context 缓存，
  也不建立 Session 命令串行器；
- 覆盖 `NATIVE_APP`、`WEBVIEW`、带非空后缀的 `WEBVIEW_`、精确 `CHROMIUM` 和
  Unknown Context，空/重复名称，滚动与小数 Rect，viewport 无效值，Context
  切换失败或 `DeliveryUnknown`、stale 以及未发送错误；Unknown Context 下的
  组合 Find/FindElements/Element Find/Element Tap 应为
  `CodeUnsupported` + `DeliveryNotSent`，且除成功的 `CurrentContext` 探针外
  不发送主体请求；
- 在协议测试之外，按 Safari/WKWebView、Android WebView/Chrome、Driver/WDA/
  Chromedriver、设备 OS、真机/模拟器和 Appium Host OS 组合记录真实兼容性。

DP-090 本身不新增 Go 文件、运行时请求、第三方依赖、Host 工具调用或真实
兼容性结论。
