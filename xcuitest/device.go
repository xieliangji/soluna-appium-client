package xcuitest

import (
	"context"
	"encoding/json"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
)

const (
	iosLockOperation        = "ios_lock"
	iosUnlockOperation      = "ios_unlock"
	iosIsLockedOperation    = "ios_is_locked"
	iosPressButtonOperation = "ios_press_button"
	iosShakeOperation       = "ios_shake"
)

// IOSButton 表示 XCUITest 在 iOS 上支持的物理设备按键。
type IOSButton string

const (
	IOSButtonHome       IOSButton = "home"
	IOSButtonVolumeUp   IOSButton = "volumeup"
	IOSButtonVolumeDown IOSButton = "volumedown"
	IOSButtonAction     IOSButton = "action"
	IOSButtonCamera     IOSButton = "camera"
)

// IOSLock 使用 XCUITest 锁定设备。
//
// autoUnlockAfter 为零时仅锁定设备，不自动解锁。
// 正数表示锁定后经过指定时间自动解锁。
// 负数属于无效参数。
func IOSLock(
	ctx context.Context,
	session *appium.Session,
	autoUnlockAfter time.Duration,
) error {
	if autoUnlockAfter < 0 {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: iosLockOperation,
			Message:   "auto unlock duration must not be negative",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	arguments := map[string]any{}

	if autoUnlockAfter > 0 {
		arguments["seconds"] = autoUnlockAfter.Seconds()
	}

	_, err := executeIOSDeviceCommand(
		ctx,
		session,
		iosLockOperation,
		"lock",
		arguments,
	)

	return err
}

// IOSUnlock 使用 XCUITest 解锁设备。
//
// 仅支持设备当前配置允许的简单解锁场景。
// 具体设备安全策略由 XCUITest Driver 决定。
func IOSUnlock(
	ctx context.Context,
	session *appium.Session,
) error {
	_, err := executeIOSDeviceCommand(
		ctx,
		session,
		iosUnlockOperation,
		"unlock",
		nil,
	)

	return err
}

// IOSIsLocked 判断当前设备是否处于锁定状态。
func IOSIsLocked(
	ctx context.Context,
	session *appium.Session,
) (bool, error) {
	value, err := executeIOSDeviceCommand(
		ctx,
		session,
		iosIsLockedOperation,
		"isLocked",
		nil,
	)
	if err != nil {
		return false, err
	}

	var locked *bool

	if err := json.Unmarshal(value, &locked); err != nil {
		return false, &appium.Error{
			Code:      appium.CodeResponseInvalid,
			Operation: iosIsLockedOperation,
			Message:   "is locked response must be a boolean",
			Delivery:  appium.DeliveryAcknowledged,
			Cause:     err,
		}
	}

	if locked == nil {
		return false, &appium.Error{
			Code:      appium.CodeResponseInvalid,
			Operation: iosIsLockedOperation,
			Message:   "is locked response must be a boolean",
			Delivery:  appium.DeliveryAcknowledged,
		}
	}

	return *locked, nil
}

// IOSPressButton 使用 XCUITest 模拟一次 iOS 物理按键操作。
//
// 部分按键具有设备或系统版本限制，
// 客户端只校验按键名称，具体设备支持情况由 Driver 判断。
func IOSPressButton(
	ctx context.Context,
	session *appium.Session,
	button IOSButton,
) error {
	if !validIOSButton(button) {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: iosPressButtonOperation,
			Message:   "unsupported iOS button",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	_, err := executeIOSDeviceCommand(
		ctx,
		session,
		iosPressButtonOperation,
		"pressButton",
		map[string]any{
			"name": string(button),
		},
	)

	return err
}

// IOSShake 使用 XCUITest 模拟摇动设备。
//
// 当前 XCUITest Driver 仅在 Simulator 上支持该能力。
// 客户端不会根据 Capabilities 猜测设备类型，
// 真机调用时由 Driver 返回明确错误。
func IOSShake(
	ctx context.Context,
	session *appium.Session,
) error {
	_, err := executeIOSDeviceCommand(
		ctx,
		session,
		iosShakeOperation,
		"shake",
		nil,
	)

	return err
}

// executeIOSDeviceCommand 校验 Session Driver 并执行 XCUITest 设备命令。
func executeIOSDeviceCommand(
	ctx context.Context,
	session *appium.Session,
	operation string,
	method string,
	arguments map[string]any,
) (json.RawMessage, error) {
	if err := requireXCUITestSession(
		operation,
		session,
	); err != nil {
		return nil, err
	}

	var args []any

	if arguments != nil {
		args = []any{arguments}
	}

	value, err := session.ExecuteScript(
		ctx,
		"mobile: "+method,
		args,
	)
	if err != nil {
		return nil, iosOperationError(
			operation,
			err,
		)
	}

	return value, nil
}

// validIOSButton 判断按键是否属于当前公开支持的 iOS 按键集合。
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
