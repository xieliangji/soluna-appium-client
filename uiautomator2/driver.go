package uiautomator2

import (
	"fmt"

	appium "github.com/xieliangji/soluna-appium-client"
)

const uiAutomator2AutomationName = "UiAutomator2"

// requireUiAutomator2Session 校验 Session 是否允许执行 UiAutomator2 专有命令。
//
// 校验只使用创建 Session 后远端确认的 automationName，
// 不使用原始 Capability，不规范化，也不发送远端探测请求。
// Session 的关闭状态仍由根包实际命令执行链校验。
func requireUiAutomator2Session(
	session *appium.Session,
	operation string,
) error {
	if session == nil || session.ID() == "" {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: operation,
			Message:   "session is not initialized",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	automationName := session.AutomationName()

	// 创建失败后仅用于清理的 Session 不具有 automationName。
	if automationName == "" {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: operation,
			Message:   "session is not usable for UiAutomator2 commands",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	if automationName != uiAutomator2AutomationName {
		return &appium.Error{
			Code:      appium.CodeUnsupported,
			Operation: operation,
			Message: fmt.Sprintf(
				"command requires automationName %q, got %q",
				uiAutomator2AutomationName,
				automationName,
			),
			Delivery: appium.DeliveryNotSent,
		}
	}

	return nil
}
