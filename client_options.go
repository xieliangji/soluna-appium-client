package appium

import (
	"net/http"
	"time"
)

// ClientOptions 定义 Appium Client 的可选配置。
//
// ClientOptions 的零值可直接使用。
// 各字段为零值时，由 Client 使用对应的默认配置。
type ClientOptions struct {
	// HTTPClient 指定底层 HTTP Client。
	//
	// 为 nil 时使用客户端内部创建的默认 HTTP Client。
	// 提供的 HTTP Client 不应设置 Timeout，远端命令的超时统一由
	// context.Context 和 CommandTimeout 控制。
	HTTPClient *http.Client

	// CommandTimeout 表示普通远端命令的默认超时时间。
	//
	// 为零时使用客户端默认值，负数属于无效配置。
	// 如果调用方传入的 context 具有更早的截止时间，则优先使用
	// context 的截止时间。
	CommandTimeout time.Duration

	// ReadyProbeTimeout 表示 Appium Server 就绪探测的最大执行时间。
	//
	// 为零时使用客户端默认值，负数属于无效配置。
	ReadyProbeTimeout time.Duration

	// SessionCleanupTimeout 表示清理远端 Session 时允许使用的最大执行时间。
	//
	// 该超时主要用于 Session 创建过程中已经获得 Session ID，
	// 但后续初始化失败时执行的尽力清理。
	// 为零时使用客户端默认值，负数属于无效配置。
	SessionCleanupTimeout time.Duration

	// Limits 定义客户端处理远端数据时使用的资源边界。
	//
	// Limits 中为零的字段分别使用对应的默认值。
	Limits Limits

	// Observer 接收远端命令的执行观测事件。
	//
	// 为 nil 时不产生观测回调。回调在命令执行链中同步调用，耗时可能影响
	// 调用方 context 和整体延迟；实现必须并发安全、快速返回且不得 panic。
	// 客户端不会异步排队或恢复回调 panic。
	Observer Observer
}
