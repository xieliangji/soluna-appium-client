package xcuitest

import (
	"fmt"

	appium "github.com/xieliangji/soluna-appium-client"
)

const xcuiTestAutomationName = "XCUITest"

// requireXCUITestSession 校验 Session 是否允许执行 XCUITest 专有命令。
//
// 校验只使用创建 Session 后远端确认的 automationName，
// 不使用调用方创建 Session 时提交的原始 Capability，
// 也不会执行大小写规范化或远端探测。
//
// Session 的关闭状态仍由根包实际命令执行链路负责校验，
// 平台包不会为了校验 Driver 主动发送额外请求。
func requireXCUITestSession(
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

	// 创建失败后仅用于清理的 Session 不具有 automationName，
	// 不能用于普通 Driver 命令。
	if automationName == "" {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: operation,
			Message:   "session is not usable for XCUITest commands",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	if automationName != xcuiTestAutomationName {
		return &appium.Error{
			Code:      appium.CodeUnsupported,
			Operation: operation,
			Message: fmt.Sprintf(
				"command requires automationName %q, got %q",
				xcuiTestAutomationName,
				automationName,
			),
			Delivery: appium.DeliveryNotSent,
		}
	}

	return nil
}
