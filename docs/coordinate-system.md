# WebDriver、Driver 像素几何与截图像素平面

> 文档状态：Active
> 适用阶段：v0.x 至首个稳定版本
> 技术基线：Go 1.26.5，Appium 3.x
> 对应设计项：`DP-060`
> 最后更新：2026-08-30

本文档是 SDK 坐标空间的主要事实源。它先于 `VIS-006` 的运行时代码，固定
`Rect`、`Point`、`PixelRect` 和 `ViewportRect` 的边界；不把不同 Driver 当前
实现中可以观察到的换算步骤变成客户端的隐式行为，也不把 Driver 报告的像素
几何自动绑定到某一次 Screenshot 的解码图像。

## 1. 结论

SDK 区分三个不能自动等同的概念：

| 概念 | 公共表示 | 单位和原点 | 主要用途 |
|---|---|---|---|
| WebDriver 几何空间 | `Rect`、`Point` | Driver 的 WebDriver 坐标单位；原点和轴方向由 WebDriver viewport 定义 | `WindowRect`、`Element.Rect`、W3C Actions |
| Driver 像素几何 | `PixelRect` | 产生该值的 Driver 命令所报告的整数像素单位和原点；类型本身不标识具体像素平面 | `ViewportRect` 等 Driver 几何事实 |
| 具体截图像素平面 | 每个解码后的 Screenshot 产物自身拥有 | 该图像缓冲区的实际像素；原点为图像左上角，X 向右、Y 向下 | 图像解码、OCR/CV，以及兼容性确认后的裁剪 |

`Rect`/`Point` 和 `PixelRect` 不可互换。尤其是名称都包含 viewport 时，
W3C Actions 的 viewport 与 Appium `mobile: viewportRect` 返回的 Driver 像素
viewport 仍然属于不同空间。`PixelRect` 也不自动等于任意一次
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
  返回的连续坐标语义。元素矩形可以有小数坐标，不能据此推断 Driver 像素
  几何或某一张截图的像素位置。
- `Point` 的 `X`、`Y` 使用 `int`，只用于 W3C Actions 的整数指针位置。
- 两种类型都以左上角为基准，X 轴向右、Y 轴向下；矩形按半开区域
  `[X, X+Width) × [Y, Y+Height)` 理解。
- `WindowRect` 和 `Element.Rect` 都属于该空间。`WindowRect` 的字段是
  WebDriver 的 window/viewport 事实，不是截图的宽高证明。
- `Point` 使用 `origin: "viewport"` 发送时，坐标是当前 WebDriver viewport
  坐标；客户端不会先乘以设备 scale，也不会加减 status bar 偏移。

`Rect` 的现有行为保持不变：它承载 WebDriver 的连续值，不承担 Driver 像素
几何或具体截图像素平面的边界校验；引入 `PixelRect` 不会改变其 `float64`
形态，也不会把 `PixelRect` 的单位或边界规则附加到 `Rect`。对 `Rect` 的查找
和点击校验仍由现有 Element 行为负责。

### 2.2 现有调用的空间归属

| API/数据 | 空间 | 设计约束 |
|---|---|---|
| `Session.WindowRect` | WebDriver | 供 Native Element 查找和点击交集使用 |
| `Element.Rect` | WebDriver | 只描述 Driver 返回的元素几何 |
| `Session.Tap`、`LongPress`、`Swipe` | WebDriver | 通过 W3C Actions 发送整数 viewport 坐标 |
| `Element.TapInWindowIntersection` | WebDriver | 只计算 `WindowRect` 与 `Element.Rect` 的交集 |
| `Session.Screenshot`、`Element.Screenshot` | 具体图像产物 | 每次解码结果拥有自己的像素平面；不自动绑定 `Rect` 或 `PixelRect` |
| `Session.ViewportRect` | Driver 像素几何 | 报告当前 Driver 的 viewport pixel geometry；不改变上面任何 API，也不自动成为 Screenshot crop rectangle |

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
Web Context 的关系由 `DP-090` 单独定义。Screenshot 的实际像素宽高和像素平面
始终以该次解码图像自身为准。

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

`ViewportRect` 不参与现有 Native Find/Tap：

1. `Session.Find`/`FindElements` 继续使用 WebDriver `WindowRect` 和
   `Element.Rect` 的正面积交集；
2. `Element.Tap`/`TapInWindowIntersection` 继续在 WebDriver 坐标空间计算
   整数 `Point`，并通过 W3C Actions 的 viewport origin 发送；
3. `Session.Tap`、`LongPress` 和 `Swipe` 不读取 `ViewportRect`，也不因 scale、
   orientation 或 status bar 自动改写坐标；
4. `PixelRect` 不作为 Element Rect、Window Rect 或 Actions 的替代类型；
5. `Session.Screenshot` 不读取或校验 `ViewportRect`，`Session.ViewportRect` 也不
   触发截图或返回 Screenshot 引用；
6. 只有调用方已经为同一环境、Context、采集路径和稳定状态确认像素平面兼容时，
   才能自行把 `PixelRect` 用作 crop rectangle；SDK 不自动裁剪或保存该结论。

这条边界目前只覆盖 Native Context。Web Context 的 DOM rect、浏览器 CSS
viewport、页面滚动和设备 Screenshot 的关系留给 `DP-090` 单独设计；在该设计
完成前，不能把 `ViewportRect` 用作 DOM Element 的可见性或点击证明。

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
  混用，破坏现有 Native 几何和 Actions 契约。
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
