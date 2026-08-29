package uiautomator2

import (
	"context"
	"encoding/json"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
)

const (
	androidLockOperation              = "android_lock"
	androidUnlockOperation            = "android_unlock"
	androidIsLockedOperation          = "android_is_locked"
	androidPressKeyOperation          = "android_press_key"
	androidOpenNotificationsOperation = "android_open_notifications"
)

// AndroidUnlockType 表示 UiAutomator2 支持的 Android 锁屏解锁类型。
type AndroidUnlockType string

const (
	AndroidUnlockTypePIN             AndroidUnlockType = "pin"
	AndroidUnlockTypePINWithKeyEvent AndroidUnlockType = "pinWithKeyEvent"
	AndroidUnlockTypePassword        AndroidUnlockType = "password"
	AndroidUnlockTypePattern         AndroidUnlockType = "pattern"
)

// AndroidUnlockStrategy 表示 UiAutomator2 使用的 Android 解锁策略。
type AndroidUnlockStrategy string

const (
	AndroidUnlockStrategyLockSettings AndroidUnlockStrategy = "locksettings"
	AndroidUnlockStrategyUIAutomator  AndroidUnlockStrategy = "uiautomator"
)

// AndroidUnlockOptions 定义 Android 设备解锁参数。
type AndroidUnlockOptions struct {
	// Type 表示当前设备使用的锁屏类型。
	Type AndroidUnlockType

	// Key 表示对应锁屏类型使用的 PIN、密码或图案值。
	Key string

	// Strategy 表示解锁实现。
	//
	// 为空时不显式指定，由 UiAutomator2 Driver 使用默认策略。
	Strategy AndroidUnlockStrategy

	// Timeout 表示等待解锁成功的最长时间。
	//
	// 为零时不显式指定，由 UiAutomator2 Driver 使用默认值。
	// 非零值必须为正数且能够精确表示为整数毫秒。
	Timeout time.Duration
}

// AndroidKeyCode 表示 Android KeyEvent keycode。
//
// 类型保持为整数以覆盖 Android 当前及未来定义的完整 keycode 集合。
// 常用按键提供命名常量，调用方也可以显式转换其他合法 keycode。
type AndroidKeyCode int

const (
	AndroidKeyHome       AndroidKeyCode = 3
	AndroidKeyBack       AndroidKeyCode = 4
	AndroidKeyVolumeUp   AndroidKeyCode = 24
	AndroidKeyVolumeDown AndroidKeyCode = 25
	AndroidKeyPower      AndroidKeyCode = 26
	AndroidKeyEnter      AndroidKeyCode = 66
	AndroidKeyMenu       AndroidKeyCode = 82
	AndroidKeySearch     AndroidKeyCode = 84
)

// AndroidLock 使用 UiAutomator2 锁定 Android 设备。
//
// autoUnlockAfter 为零时仅锁定设备，不自动解锁。
// 正数表示经过指定时间后由 Driver 自动解锁。
// 负数属于无效参数。
func AndroidLock(
	ctx context.Context,
	session *appium.Session,
	autoUnlockAfter time.Duration,
) error {
	if autoUnlockAfter < 0 {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: androidLockOperation,
			Message:   "auto unlock duration must not be negative",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	var arguments map[string]any

	if autoUnlockAfter > 0 {
		arguments = map[string]any{
			"seconds": autoUnlockAfter.Seconds(),
		}
	}

	_, err := executeAndroidDeviceCommand(
		ctx,
		session,
		androidLockOperation,
		"lock",
		arguments,
	)

	return err
}

// AndroidUnlock 使用 UiAutomator2 解锁 Android 设备。
func AndroidUnlock(
	ctx context.Context,
	session *appium.Session,
	options AndroidUnlockOptions,
) error {
	arguments, err := encodeAndroidUnlockOptions(options)
	if err != nil {
		return err
	}

	_, err = executeAndroidDeviceCommand(
		ctx,
		session,
		androidUnlockOperation,
		"unlock",
		arguments,
	)

	return err
}

// AndroidIsLocked 判断当前 Android 设备是否处于锁定状态。
func AndroidIsLocked(
	ctx context.Context,
	session *appium.Session,
) (bool, error) {
	value, err := executeAndroidDeviceCommand(
		ctx,
		session,
		androidIsLockedOperation,
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
			Operation: androidIsLockedOperation,
			Message:   "is locked response must be a boolean",
			Delivery:  appium.DeliveryAcknowledged,
			Cause:     err,
		}
	}

	if locked == nil {
		return false, &appium.Error{
			Code:      appium.CodeResponseInvalid,
			Operation: androidIsLockedOperation,
			Message:   "is locked response must be a boolean",
			Delivery:  appium.DeliveryAcknowledged,
		}
	}

	return *locked, nil
}

// AndroidPressKey 使用 UiAutomator2 模拟 Android KeyEvent。
//
// longPress 为 true 时执行长按，否则执行普通按键事件。
// 高级 metastate、flags 和 input source 暂不暴露。
func AndroidPressKey(
	ctx context.Context,
	session *appium.Session,
	keycode AndroidKeyCode,
	longPress bool,
) error {
	if keycode < 0 {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: androidPressKeyOperation,
			Message:   "Android keycode must not be negative",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	arguments := map[string]any{
		"keycode": int(keycode),
	}

	if longPress {
		arguments["isLongPress"] = true
	}

	_, err := executeAndroidDeviceCommand(
		ctx,
		session,
		androidPressKeyOperation,
		"pressKey",
		arguments,
	)

	return err
}

// AndroidOpenNotifications 使用 UiAutomator2 打开 Android 通知栏。
//
// 如果通知栏已经打开，Driver 不执行额外操作。
func AndroidOpenNotifications(
	ctx context.Context,
	session *appium.Session,
) error {
	_, err := executeAndroidDeviceCommand(
		ctx,
		session,
		androidOpenNotificationsOperation,
		"openNotifications",
		nil,
	)

	return err
}

// executeAndroidDeviceCommand 校验 Session Driver 并执行 UiAutomator2 设备命令。
func executeAndroidDeviceCommand(
	ctx context.Context,
	session *appium.Session,
	operation string,
	method string,
	arguments map[string]any,
) (json.RawMessage, error) {
	if err := requireUiAutomator2Session(
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
		return nil, androidOperationError(
			operation,
			err,
		)
	}

	return value, nil
}

// encodeAndroidUnlockOptions 校验并编码 Android 设备解锁参数。
func encodeAndroidUnlockOptions(
	options AndroidUnlockOptions,
) (map[string]any, error) {
	switch options.Type {
	case AndroidUnlockTypePIN,
		AndroidUnlockTypePINWithKeyEvent,
		AndroidUnlockTypePassword,
		AndroidUnlockTypePattern:

	default:
		return nil, &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: androidUnlockOperation,
			Message:   "unsupported Android unlock type",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	if options.Key == "" {
		return nil, &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: androidUnlockOperation,
			Message:   "Android unlock key is empty",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	switch options.Strategy {
	case "",
		AndroidUnlockStrategyLockSettings,
		AndroidUnlockStrategyUIAutomator:

	default:
		return nil, &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: androidUnlockOperation,
			Message:   "unsupported Android unlock strategy",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	if options.Timeout < 0 {
		return nil, &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: androidUnlockOperation,
			Message:   "unlock timeout must not be negative",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	if options.Timeout%time.Millisecond != 0 {
		return nil, &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: androidUnlockOperation,
			Message:   "unlock timeout must be an exact number of milliseconds",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	arguments := map[string]any{
		"type": string(options.Type),
		"key":  options.Key,
	}

	if options.Strategy != "" {
		arguments["strategy"] = string(options.Strategy)
	}

	if options.Timeout > 0 {
		arguments["timeoutMs"] = options.Timeout.Milliseconds()
	}

	return arguments, nil
}
