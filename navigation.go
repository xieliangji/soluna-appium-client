package appium

import "context"

const (
	backgroundAppOperation = "background_app"
	backgroundAppScript    = "mobile: backgroundApp"
)

// BackgroundApp 请求将当前 App 放入后台且不自动恢复。
//
// 每次调用只发送一次 Appium background Execute Method，并固定使用负数 seconds。
// nil error 仅表示 Driver 返回了已接受的成功响应；客户端不会读取、缓存或
// 确认后续 App 状态。需要恢复时应显式调用 ActivateApp。
func (s *Session) BackgroundApp(ctx context.Context) error {
	arguments := []any{
		map[string]any{
			"seconds": int64(-1),
		},
	}

	return s.ExecuteScriptWithOperationAndDecode(
		ctx,
		backgroundAppOperation,
		backgroundAppScript,
		arguments,
		decodeNullResponse,
	)
}
