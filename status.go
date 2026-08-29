package soluna_appium_client

// ServerStatus 表示 Appium Server 当前的运行状态。
type ServerStatus struct {
	// Ready 表示 Appium Server 当前是否能够接受新的 Session 创建请求。
	//
	// Ready 为 true 只表示 Server 当前处于可创建 Session 的状态，
	// 不保证后续 Session 创建一定成功。
	Ready bool

	// Message 表示 Appium Server 对当前状态的说明。
	Message string

	// Version 表示 Appium Server 的版本号。
	Version string
}
