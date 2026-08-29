# soluna-appium-client

[English](README.md) | [简体中文](README.zh-CN.md)

面向 Soluna 生态的 Go Appium 客户端。

## 项目状态

本项目目前处于积极开发阶段，尚不建议用于生产环境。

首个版本将基于 Soluna 当前使用的 Appium/WebDriver Adapter 提取和演进。

公共 API 目前仍处于实验阶段，可能在后续版本中发生不兼容变更。

## 项目简介

`soluna-appium-client` 是一个以移动端自动化为核心、使用 Go 编写的 Appium 客户端。

项目旨在为 Go 应用提供一层可复用、可验证且行为明确的 Appium 客户端能力，通过 W3C WebDriver 协议和 Appium 扩展命令与 Appium Server 通信。

项目重点关注：

- 可预测的会话和命令执行语义；
- 明确的超时与取消处理；
- 结构化的 WebDriver 错误分类；
- 有界的请求与响应处理；
- 基于 W3C Actions 的移动端手势；
- 可在多个 Soluna 项目中复用的 Appium 客户端能力；
- 面向真实 Appium 环境的兼容性测试。

## 计划范围

第一阶段将优先实现 Soluna 当前已经使用的能力：

- Appium Server 就绪状态检查；
- WebDriver 会话创建与终止；
- 会话健康探测；
- 元素查找；
- 元素文本、属性和矩形信息获取；
- 元素内容清除与文本输入；
- 截图和页面源获取；
- 基于 W3C Actions 的点击、长按和滑动；
- 应用激活、终止和运行状态查询；
- 屏幕录制；
- 同步脚本和 `mobile:` 扩展命令执行；
- 结构化的传输层和 WebDriver 错误处理。

后续可能通过独立扩展包增加 XCUITest 和 UiAutomator2 的平台特有能力。

## 设计原则

- 客户端库只提供协议机制，不承载业务工作流策略。
- 逻辑会话恢复由调用方负责。
- 默认不自动重试具有副作用的命令。
- 所有远程命令均支持 `context.Context` 取消和截止时间。
- 平台特有能力不应无边界地扩张通用 API。
- 核心包不依赖具体 AI 服务商或测试框架。
- Appium Server、设备和应用的生命周期管理不属于客户端核心职责。
- 每一个已修复的兼容性问题都应沉淀为回归测试。

## 非目标

本项目不计划提供：

- 完整的测试框架；
- 测试用例编排；
- 断言和测试报告；
- Appium Server 的安装与进程管理；
- 设备发现与设备生命周期管理；
- 自动恢复应用业务状态；
- 自动恢复逻辑会话；
- 对失败交互命令的自动重试；
- 默认启用的 AI 自动操作能力。

这些职责应由 Soluna 或其他上层调用方承担。

## 安装

公共 API 尚未稳定，正式安装说明将在首个可用版本发布时补充。

后续可通过以下方式引入：

```bash
go get github.com/xieliangji/soluna-appium-client