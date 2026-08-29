package xcuitest

import (
	"context"

	appium "github.com/xieliangji/soluna-appium-client"
)

const (
	iosPressButtonOperation = "ios_press_button"
	iosPressButtonScript    = "mobile: pressButton"
)

// IOSButton 表示 XCUITest Driver 支持的 iOS 物理按键。
type IOSButton string

const (
	IOSButtonHome       IOSButton = "home"
	IOSButtonVolumeUp   IOSButton = "volumeup"
	IOSButtonVolumeDown IOSButton = "volumedown"
	IOSButtonAction     IOSButton = "action"
	IOSButtonCamera     IOSButton = "camera"
)

// IOSPressButton 模拟按下 iOS 设备的物理按键。
//
// Volume Up 和 Volume Down 仅支持真机。
// Action Button 需要 Xcode 15+、iOS 16+ 和受支持设备。
// Camera Button 需要 Xcode 16+、iOS 16+ 和受支持真机。
//
// 客户端只校验 button 是否属于 XCUITest Driver 当前定义的
// iOS 按键集合，不根据设备型号、iOS 或 Xcode 版本提前推断支持状态。
// 具体运行环境不支持该按键时，远端 Driver 错误会原样进入根包错误模型。
func IOSPressButton(
	ctx context.Context,
	session *appium.Session,
	button IOSButton,
) error {
	if err := requireXCUITestSession(
		session,
		iosPressButtonOperation,
	); err != nil {
		return err
	}

	if !validIOSButton(button) {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: iosPressButtonOperation,
			Message:   "unsupported iOS button",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	_, err := session.ExecuteScript(
		ctx,
		iosPressButtonScript,
		[]any{
			map[string]any{
				"name": string(button),
			},
		},
	)
	return err
}

func validIOSButton(button IOSButton) bool {
	switch button {
	case IOSButtonHome,
		IOSButtonVolumeUp,
		IOSButtonVolumeDown,
		IOSButtonAction,
		IOSButtonCamera:
		return true

	default:
		return false
	}
}
