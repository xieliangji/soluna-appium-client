# soluna-appium-client 兼容性验证矩阵

> 文档状态：Active
>
> 适用阶段：v0.x 至首个稳定版本
>
> 协议基线：Appium 3.x
>
> 最后更新：2026-09-05
>
> 当前实测记录：无；尚无 `Verified` 组合

## 1. 文档职责

本文档维护具体运行环境和逐能力实测结果。SDK 是否纳入或实现某个能力，以
[能力矩阵](sdk-capability-matrix.md) 为准；运行通道与职责边界以
[架构](architecture.md) 和[设计 §14](design.md#14-兼容性与-host-跨平台设计)
为准。上游文档、源码观察、Runtime Discovery、单元测试和协议测试不能替代
本项目的真实环境验证。

本次 DP-140 建立记录结构与待验证通道。实际环境运行属于 DP-141；本文不创建
测试套件或引入 Host 工具依赖，也不扩大公共 API 或兼容承诺。

兼容性验证范围仅限 iOS 和 Android 真机。iOS Simulator 与 Android Emulator
不在 SDK 支持范围内，不纳入本文的 Runtime Profile、Lane 或逐能力验证记录。

## 2. 支持依据与实测结果

上游支持判断与项目实测结果分别记录，避免把上游声明当成本项目验证。

| 上游支持判断 | 含义 | 必要依据 |
|---|---|---|
| `Official` | 上游明确支持该版本与环境条件；项目尚未实测时只能称为上游支持 | 上游文档/源码链接、版本或 commit、查阅日期及适用条件 |
| `BestEffort` | 上游表示可能工作，但未重点测试或无明确保障 | 同样记录上游出处及限制，不能因缺少证据而默认使用此值 |
| `Unsupported` | 上游明确不支持，或组合缺少必要机制 | 上游出处或具体缺失机制及证据 |
| `Unknown` | 尚未取得足以判断的上游依据 | 写明缺失信息；不推测支持程度 |

| 项目结果 | 含义 |
|---|---|
| `Verified` | 通过 SDK 公共入口在所关联的真实 Runtime Profile 上执行，指定场景符合既有命令语义，且结果与证据摘要已登记 |
| `Failed` | 已执行，但结果不符合该场景的预期；记录失败阶段、实际行为和限制 |
| `Blocked` | 环境或前置条件阻止执行；记录阻塞条件，不当作能力不支持 |
| `NotRun` | 尚未执行或没有可引用实测记录 |
| `NotApplicable` | 场景不适用于该 Profile；必须写明原因，不计为通过 |

`Official`、`BestEffort` 均不等于 `Verified`。一次失败也不自动证明上游
`Unsupported`；上游判断和实测结果不一致时保留两者与差异，不能隐藏失败。

`Verified` 只覆盖记录中的“Profile + 能力 + 场景 + 测试版本”，不能标在
整个 Lane 上。单项成功、Session 创建成功或命令已登记，均不能推出其他能力
成功。能力矩阵的验证列只有在此处有对应的 `Verified` 记录时才可升级，并链接
具体记录及范围；失败、阻塞和未执行记录不替代原有的 `Unit` / `Protocol`
证据。

## 3. Runtime Profile

Runtime Profile 是实际运行环境的快照。使用稳定的合成 ID，例如
`rp-ios-001` 或 `rp-android-001`，不使用设备序列号、UDID、主机名或地址作 ID。

Host 分成三个独立角色，不能把 SDK 能在 Windows/Linux 连接 macOS 上的 Appium
误记成 Windows/Linux Appium Host 验证：

- **SDK Host**：运行 Go 调用程序的系统。
- **Appium Host**：运行 Appium Server/Driver、与设备交互的系统；本文 Lane 的 Host 指此角色。
- **准备/构建 Host**：构建、签名、安装 WDA 或准备其他组件的系统；与 Appium Host 分开登记，即使它们是同一台机器。

### 3.1 必填字段与填写模板

下表可直接复制为一个 Profile。占位符只用于模板，不是实际环境证据。
除明确不适用的字段外，填写完整版本；不能以 `latest`、`3.x`、`iOS 18+`
代替实测版本。未知写 `Unknown`，不适用写 `N/A（原因）`。必要版本、环境条件
或证据缺失时不得标记相关能力为 `Verified`。

| 字段 | 填写内容 |
|---|---|
| Profile ID / Lane ID | 唯一合成 ID；关联 §4 的通道 |
| 登记时间 | ISO 8601 时间及 UTC 偏移 |
| SDK | tag 与 commit；未发布版本写 commit，有本地修改时附可复现补丁引用 |
| Go / SDK Host | Go 完整版本；SDK Host OS 版本/build、架构 |
| Appium / Node.js | Appium Server 与 Node.js 完整版本 |
| Driver | `xcuitest` / `uiautomator2` 包名、完整版本；自定义构建附 commit |
| WDA / UiAutomator2 Server | 实际运行的组件版本或 commit、构建标识与来源；不能从 Driver 版本推算 |
| Plugins | 实际启用的 Plugin 名称、版本和影响测试的配置；未启用写“无” |
| 设备 | OS 完整版本/build；Android 另记 API level；真机型号、架构与设备类型 |
| Appium Host | OS 完整版本/build、架构；容器/虚拟机另记宿主 OS 和镜像版本 |
| 准备/构建 Host | OS 版本、架构、承担的准备步骤及相关工具版本；已有制品也记录构建来源 |
| SDK → Appium 连接 | 同机或远程，HTTP/HTTPS、代理等拓扑；不用真实 Endpoint |
| Appium → 设备连接 | 真机 USB/网络连接；RemoteXPC Tunnel 另记提供方、工具/组件版本、运行 Host、启动方式与前置条件 |
| Appium 启动 | 启动方式、影响行为的服务端选项；使用脱敏配置与合成地址 |
| WDA / Server 启动 | Driver 构建/安装/启动、预安装后启动或外部进程启动/附加；明确每个步骤的执行方及是否复用已有进程 |
| Session 配置 | `Client.CreateSession` 的请求与远端确认配置；影响行为的 capabilities、settings、timeouts 和 SDK limits；只写合成值/必要配置摘要 |
| 测试 App / Context | 合成测试 App 的版本/commit；Native/Hybrid/Safari/Chrome，Web 场景补充 §3.2 信息 |
| 上游支持判断 | §2 的值，及出处链接、版本/commit、查阅日期、适用范围和限制 |
| 环境限制 | 已知未满足条件、未覆盖设备或连接路径，不从相邻 Profile 推断 |

任一版本、设备、Host、连接、启动方式或影响行为的配置变化，都创建新的
Profile ID，保留旧记录。相同 Profile 的多次执行使用不同验证记录 ID 和时间；
不覆盖既有失败或用后来成功抹去先前结果。

### 3.2 平台补充字段

| 场景 | 必须补充的信息 |
|---|---|
| XCUITest | Xcode 版本及所在 Host（涉及构建或启动时）；WDA 制品来源、签名/安装/信任/Developer Mode 等前置条件的满足情况，不记录凭据或配对数据 |
| iOS RemoteXPC | 预安装/外部 WDA 的准备和运行路径，Tunnel 的管理方及版本；分别说明构建 Host 与执行 Host 的依赖 |
| UiAutomator2 | UiAutomator2 Server 实际制品版本；Android SDK/platform-tools 等实际参与的组件版本和运行 Host；Server 安装/启动方式及设备授权状态 |
| iOS Web / Hybrid | Safari/WebKit 或嵌入式 WebView 的版本/来源、Web Inspector 条件及相关配置 |
| Android Web / Hybrid | Chrome/WebView 和 Chromedriver 版本、调试条件、版本匹配关系及相关配置 |

Host 工具字段仅记录外部环境事实。设备发现、WDA 构建/安装、adb 和 RemoteXPC
Tunnel 管理由外部环境承担；SDK 不安装、启动或探测这些工具，不增加自动
fallback、重试或 Session 恢复。

## 4. 待验证通道

以下表格是验证范围目录，**不是 Runtime Profile，也不是上游支持声明**。
实际 Profile 必须填写 §3 的精确环境；各行当前均无实测记录，上游支持判断
均为 `Unknown`。

### 4.1 iOS / XCUITest

| Lane ID | 设备范围 | Appium Host | 连接与启动条件 | 项目结果 |
|---|---|---|---|---|
| `ios17-macos` | iOS 17.x 真机主线 | macOS | 按实际 USB/网络与 WDA 准备、启动路径分别建 Profile | `NotRun` |
| `ios18-remotexpc-macos` | iOS 18+ 真机主线 | macOS | 预安装/外部 WDA 与 RemoteXPC 条件满足；记录各自启动方式 | `NotRun` |
| `ios18-remotexpc-windows` | iOS 18+ 真机主线 | Windows | 预安装/外部 WDA 与 RemoteXPC 条件满足；准备 Host 单独记录 | `NotRun` |
| `ios18-remotexpc-linux` | iOS 18+ 真机主线 | Linux | 预安装/外部 WDA 与 RemoteXPC 条件满足；准备 Host 单独记录 | `NotRun` |
| `ios-legacy` | 低于 iOS 17 的真机 Legacy Lane | 每个实际 Host 分开记录 | 精确记录旧 OS、Driver/WDA、连接和启动限制 | `NotRun` |

iOS 17.x 主要在 macOS 验证，不继承 iOS 18+ RemoteXPC 的三 Host 范围。
iOS 18+ 各 OS 版本与三个 Host 必须独立登记；“18+”只定义通道范围，不能由
一个版本跑通推导所有后续版本通过。Legacy Lane 不定义或反向限制主线 API 基线。

本项目不支持 iOS Simulator。Simulator 不建立 Runtime Profile 或 Lane，也不能
产生任何能力的 `Verified` 结果；能力矩阵中已排除的 Simulator-only API 仍不进入
公共 SDK。

### 4.2 Android / UiAutomator2

| Lane ID | 设备范围 | Appium Host | 连接与启动条件 | 项目结果 |
|---|---|---|---|---|
| `android-uia2-macos` | Android 真机 / UiAutomator2，OS 与 API level 待实测选定 | macOS | USB/网络、Server 准备与启动方式分别记录 | `NotRun` |
| `android-uia2-windows` | Android 真机 / UiAutomator2，OS 与 API level 待实测选定 | Windows | USB/网络、Server 准备与启动方式分别记录 | `NotRun` |
| `android-uia2-linux` | Android 真机 / UiAutomator2，OS 与 API level 待实测选定 | Linux | USB/网络、Server 准备与启动方式分别记录 | `NotRun` |

Android 与 iOS 使用相同 Profile/逐能力记录结构。本项目不支持 Android Emulator；
只登记真机的 OS/API level、型号和连接方式，Emulator 不建立 Profile 或验证记录。
此目录不预设 Android 最低支持版本，实际支持范围取决于具体 Driver/Server
条件和逐能力实测记录。

## 5. 逐能力验证记录

每条记录关联一个已登记的 Profile、一个能力矩阵 ID 和一个具体场景。
同一能力的多个公共方法、Context、参数分支分别写明覆盖范围，不能以测试名称
或一句“smoke 通过”代替实际覆盖信息。

### 5.1 记录模板

| 字段 | 填写内容 |
|---|---|
| Record ID / Profile ID | 唯一合成验证记录 ID；链接对应 Profile |
| 能力 ID / 公共入口 | 能力矩阵中的 ID 与实际执行的 Client/Session/Element 方法或平台函数 |
| 时间 / 执行版本 | ISO 8601 时间及 UTC 偏移；测试套件/脚本版本或 commit、用例名和执行命令 |
| 场景与前置状态 | 合成 App/数据、Context、参数分支与调用顺序；必要的方向、键盘、权限等条件 |
| 预期行为 | 链接现有命令语义/坐标等契约，写明要检查的返回值和可观察副作用 |
| 实际行为 / 项目结果 | §2 的结果；已完成的检查、次数与失败情况，不推测未观察到的状态 |
| 失败信息 | 失败阶段；SDK 返回错误时记录 Operation、Code、Delivery 和脱敏摘要；没有 SDK 错误时说明实际情况 |
| 证据 | 可追溯运行/报告引用及可审阅的文字摘要；成功与失败均需证据，引用失效或证据不足不能当作通过 |
| 限制 | 未覆盖方法、参数、Context 或前置条件；阻塞/不适用原因；重复运行关联到旧 Record ID |

`Failed`、`Blocked`、`NotApplicable` 应与成功记录一样保留具体范围。
环境在 Session 创建前失败时记录该阶段，被阻止执行的能力写 `Blocked`，
不能记成已调用后的失败。预期错误场景只有其实际 Error/Delivery 符合既有契约
才可记为 `Verified`，并明确只覆盖该错误场景，不能推出正常执行成功。

验证必须经过 SDK 公共入口；直接调用 WDA、UiAutomator2 Server 或 Host 工具的
结果只能补充环境诊断。Discovery 返回命令只证明目录快照，不能证明命令可执行。
副作用命令的成功响应按既有语义判断，不自动推导业务页面或最终设备状态。

### 5.2 场景补充信息

具体断言沿用能力的权威契约；以下信息用于解释结果适用范围，不新增命令行为：

| 能力/场景 | 验证记录需补充的事实 |
|---|---|
| Context / Find / Tap / 几何 / Screenshot | 实际 Context、方向、滚动/缩放、状态栏/键盘状态、CSS/Driver 像素空间和截图采集路径；坐标到点击结果的观察范围 |
| Keyboard / BackgroundApp / Orientation / DeepLink | 实际参数分支、前置状态、返回值与副作用观察；上游 route/Execute Method 的版本限制 |
| DeviceTime | 真机、返回的时间格式和 UTC 偏移；已知 Driver 取值路径，不把 Host 时间当作设备验证结果 |
| Pull Logs | 实际 LogType、相关配置和 Context、连续显式读取的结果及消费/清空行为观察；不提交原始日志 |
| XCUITest / UiAutomator2 平台能力 | 每项能力的最低 OS、Driver/WDA/Server、设备类型与 Host 条件的依据及本次满足情况 |

## 6. 实际记录与维护

### 6.1 Runtime Profiles

当前无实际 Runtime Profile。§4 的通道和 §3 的模板不计为环境记录。

### 6.2 能力验证结果

当前无逐能力实测结果；没有可对外称为 `Verified` 的组合。

后续登记时，在以上两节追加具备稳定锚点的 Profile 和 Record，并双向关联。
同一公共 API smoke suite 在各 Appium Host 的运行应记录同一套件版本；环境、
参数或用例覆盖不同则明确列出差异，不合并为“跨 Host 已通过”。

仅提交合成 ID/配置与不含敏感内容的事实摘要。不得提交凭据、Token、私钥、
配对材料、客户数据、私有服务地址或真实设备的原始日志、截图、录屏、页面源等
采集物。受控运行报告可用合成运行 ID 引用，文档保留可审阅的脱敏文字结论；
协议 fixture 使用合成材料，不能把真实采集物直接复制入仓库。

失败记录保留限制和未验证范围，不通过 SDK fallback、重试或修改既有命令
语义制造通过。环境/Profile 的变化要求重新记录；任何已验证结论都不自动扩展
到相邻版本、其他 Host 或矩阵之外的能力。
