package soluna_appium_client

// AppState 表示应用在设备上的当前运行状态。
//
// 数值与 Appium queryAppState 返回的协议值保持一致。
type AppState int

const (
	AppStateNotInstalled        AppState = 0 // 应用未安装
	AppStateNotRunning          AppState = 1 // 应用已安装但未运行
	AppStateBackgroundSuspended AppState = 2 // 应用在后台运行但处于挂起状态
	AppStateBackground          AppState = 3 // 应用正在后台运行
	AppStateForeground          AppState = 4 // 应用正在前台运行
)
