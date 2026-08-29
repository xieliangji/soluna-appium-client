package soluna_appium_client

import "time"

// Timeouts 表示 WebDriver Session 当前使用的超时配置。
//
// 所有超时均使用 Go 的 time.Duration 表示。
// 具体的协议传输层负责在 time.Duration 与 WebDriver 使用的毫秒值之间转换。
type Timeouts struct {
	// Script 表示脚本执行超时时间。
	Script time.Duration

	// PageLoad 表示页面加载超时时间。
	PageLoad time.Duration

	// Implicit 表示元素查找使用的隐式等待超时时间。
	Implicit time.Duration
}
