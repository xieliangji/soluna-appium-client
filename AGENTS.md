# Repository Guidelines

## 工作原则

- 先确认实际任务、受影响的公共行为和对应能力矩阵项，再修改代码。
- 对可观察缺陷，优先通过公共入口或聚焦回归测试复现。
- 只完成当前任务要求的最小完整改动，不自动继续开发计划中的下一项。
- 在真正拥有该约束的边界修复问题，不在多个调用方堆补丁。
- 避免无关重构、格式化噪声、依赖变更、兼容层和预判式抽象。
- 代码、测试、设计、能力矩阵或计划发生冲突时，停止冲突部分并报告，不自行选择一个版本继续。

## 核心边界

- 公共对象模型只有 `appium.Client -> appium.Session -> appium.Element`。
- 不创建 `xcuitest.Client`、`uiautomator2.Client`、公共 `bidi.Client` 或平台 Session wrapper。
- `xcuitest` 的所有导出函数必须以 `IOS` 开头。
- `uiautomator2` 的所有导出函数必须以 `Android` 开头。
- 平台包只纳入通用层无法可靠替代的高价值能力，不重复包装根包能力。
- 所有 Appium HTTP 命令必须复用根包统一执行链；平台包不得自行创建 HTTP Client、拼接 Endpoint 或复制错误模型。
- 不公开任意 Method/Route 的 Raw Command API。
- 不增加自动命令重试、Session 重建、业务状态恢复、stale Element 自动重定位、隐式 fallback 或协议值自动规范化。
- Runtime Discovery 只报告快照，不作为普通命令的自动门禁、fallback 或成功保证。
- BiDi 即使使用独立内部连接，也必须绑定同一个根包 Session。
- 未经能力矩阵和设计明确接受，不得让 SDK 直接依赖 `xcodebuild`、`xctrace`、`simctl`、`devicectl`、`adb` 等 Host 工具。
- 未经任务明确批准，不新增第三方运行时依赖。

## 文档入口

每个维度只认一个主要事实源：

- 高层对象、包和运行通道：`docs/architecture.md`
- 详细设计与设计决策：`docs/design.md`
- 能力范围、状态与验证等级：`docs/sdk-capability-matrix.md`
- 实施顺序与验收条件：`docs/development-plan.md`
- Go 编码约定：`docs/go-conventions.md`
- 命令请求、响应、副作用与失败语义：`docs/command-semantics.md`
- Error、Delivery 与诊断数据：`docs/error-model.md`
- WebDriver 与截图坐标：`docs/coordinate-system.md`
- 真实版本与 Host 验证：`docs/compatibility.md`
- 公共 API 与发布兼容性：`docs/release-policy.md`

只阅读当前任务相关的权威文档，不默认加载整个文档树。开发计划只定义顺序，不授权一次执行全部剩余项目。

## 能力与兼容性门禁

- `docs/sdk-capability-matrix.md` 是公共能力是否进入 SDK 的唯一依据。
- `Implemented` 可维护；`Accepted` 仅在当前任务选中时实施。
- `Architecture` 先完成设计；`Deferred` 和 `Excluded` 不得自行实现。
- 不新增能力矩阵之外的公共 API。
- 只有 API、实现、校验、错误语义和必要协议测试均完成，才可标记为 `Implemented / Protocol`。
- 只有真实环境结果已写入 `docs/compatibility.md`，才可标记为 `Verified`。
- 普通实现任务不得为了让代码成立而改写架构、设计决策或 ADR；应报告设计阻塞。

## 代码与验证

修改 Go 代码前阅读 `docs/go-conventions.md`。

完成后至少运行：

```sh
gofmt -w <changed-go-files>
go test ./...
```

涉及并发、Session 状态、后台 goroutine、事件流或 BiDi 时，再运行：

```sh
go test -race ./...
```

只报告实际执行过的检查。协议测试通过不代表真实设备兼容。

## 安全与 Git

- 不提交凭据、Token、私钥、`.env`、客户数据、私有服务地址或真实设备采集物。
- 测试与文档使用合成 ID、Payload、日志、截图和录屏。
- 保留现有脱敏、严格解码和资源上限，不默认输出敏感请求或响应内容。
- 不覆盖用户未提交修改。
- 不执行破坏性 reset、amend、rebase、历史改写、无关 revert 或 force-push。
- 建分支、提交、推送和创建 PR 仅在当前任务明确要求时执行。

## 完成报告

说明实际改动、公共行为变化、文档或能力状态更新、已执行检查、未执行检查以及剩余风险。

## 文件维护

本文件只保留长期稳定、适用于整个仓库的强约束和文档入口。

只有子目录存在稳定且不同的规则时才新增嵌套 `AGENTS.md`；嵌套文件只写差异，不复制根规则。
