package soluna_appium_client

import "time"

// Observer 用于观察客户端命令的执行过程。
//
// Observer 只用于日志、指标和诊断，不参与命令执行结果的决策。
// 回调方法没有返回值，客户端不会根据 Observer 的行为改变命令结果。
type Observer interface {
	OnCommandStarted(event CommandStartedEvent)
	OnCommandFinished(event CommandFinishedEvent)
}

// CommandStartedEvent 表示一条远端命令开始执行时产生的观测事件。
type CommandStartedEvent struct {
	// Operation 表示客户端定义的命令操作名称。
	Operation string

	// StartedAt 表示命令开始执行的时间。
	StartedAt time.Time

	// RequestBytes 表示编码后的请求体大小。
	//
	// 没有请求体时为零。
	RequestBytes int64
}

// CommandFinishedEvent 表示一条远端命令执行结束时产生的观测事件。
type CommandFinishedEvent struct {
	// Operation 表示客户端定义的命令操作名称。
	Operation string

	// Duration 表示命令从开始执行到结束所消耗的时间。
	Duration time.Duration

	// StatusCode 表示远端 HTTP 响应状态码。
	//
	// 没有收到 HTTP 响应时为零。
	StatusCode int

	// RequestBytes 表示编码后的请求体大小。
	//
	// 没有请求体时为零。
	RequestBytes int64

	// ResponseBytes 表示客户端实际读取到的响应体大小。
	//
	// 没有读取到响应体时为零。
	ResponseBytes int64

	// ErrorCode 表示命令结束时归一化后的错误类别。
	//
	// 命令成功时为空字符串。
	ErrorCode ErrorCode

	// Delivery 表示客户端能够确认的命令投递状态。
	Delivery DeliveryState
}
