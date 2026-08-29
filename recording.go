package soluna_appium_client

import "time"

// RecordingOptions 定义屏幕录制使用的跨平台通用参数。
type RecordingOptions struct {
	// TimeLimit 表示本次录屏允许持续的最长时间。
	//
	// 为零时不显式指定录屏时长，由远端 Driver 使用其默认行为。
	// 负数属于无效参数。
	TimeLimit time.Duration
}
