# 发布与兼容性策略

> 文档状态：Draft
> 适用阶段：v0.x 至首个稳定版本

## 公共包名称

模块路径保持：

```text
github.com/xieliangji/soluna-appium-client
```

根包的 Go package name 为 `appium`，公共对象按
`appium.Client`、`appium.Session` 和 `appium.Element` 表达。package name
变更不改变 module path，但会影响未显式设置 import alias、直接引用旧包名的
调用方；该迁移必须在首个稳定版本前完成，不在稳定版本后通过兼容别名长期保留。

## v0.x 兼容边界

v0.x 的公共 API、错误 identity 和命令语义仍可能在设计补充或协议验证中发生
不兼容变化。每次此类变化都应同步更新能力矩阵、命令语义或设计决策，并在发布
说明中列出迁移影响。

## 验证状态

单元测试和协议测试不构成真实环境兼容承诺。具体 Appium、Driver、WDA、Host
和设备组合只有在 `docs/compatibility.md` 登记实测结果后，才可对外称为
`Verified`。
