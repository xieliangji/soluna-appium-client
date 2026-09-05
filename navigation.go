package appium

import (
	"context"
	"unicode/utf8"
)

const (
	backgroundAppOperation = "background_app"
	backgroundAppScript    = "mobile: backgroundApp"
	deepLinkOperation      = "deep_link"
	deepLinkScript         = "mobile: deepLink"
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

// DeepLink 请求 Driver 打开指定的 Deep Link。
//
// url 是要打开的 URI；appID 为空时由 Driver/操作系统选择目标应用。
// XCUITest 将非空 appID 作为 bundleId，UiAutomator2 将其作为 package。
// 空 url 或非法 UTF-8 参数会在本地被拒绝；其余字符串原样发送，由远端
// 校验 URI 和目标应用，不做 URL 编码、trim 或协议格式规范化。
// 目标 Driver 由创建 Session 后远端确认的 automationName 决定；未知
// automationName 会在发送请求前返回 CodeUnsupported。成功只表示 Driver
// 接受了本次请求，客户端不会断言应用已启动或页面已到达。
func (s *Session) DeepLink(
	ctx context.Context,
	url string,
	appID string,
) error {
	client, err := s.commandClient(deepLinkOperation)
	if err != nil {
		return err
	}

	arguments := struct {
		URL      string `json:"url"`
		BundleID string `json:"bundleId,omitempty"`
		Package  string `json:"package,omitempty"`
	}{URL: url}

	switch s.automationName {
	case xcuiTestAutomationName:
		arguments.BundleID = appID
	case uiAutomator2AutomationName:
		arguments.Package = appID
	default:
		return &Error{
			Code:      CodeUnsupported,
			Operation: deepLinkOperation,
			Message:   "deep link requires XCUITest or UiAutomator2 automationName",
			Delivery:  DeliveryNotSent,
		}
	}

	if url == "" {
		return &Error{
			Code:      CodeInvalidArgument,
			Operation: deepLinkOperation,
			Message:   "deep link URL is empty",
			Delivery:  DeliveryNotSent,
		}
	}
	if !utf8.ValidString(url) || !utf8.ValidString(appID) {
		return &Error{
			Code:      CodeInvalidArgument,
			Operation: deepLinkOperation,
			Message:   "deep link URL and app ID must be valid UTF-8",
			Delivery:  DeliveryNotSent,
		}
	}

	return executeScriptCommand(
		ctx,
		client,
		deepLinkOperation,
		s.id,
		deepLinkScript,
		[]any{arguments},
		decodeNullResponse,
	)
}
