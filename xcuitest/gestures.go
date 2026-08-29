package xcuitest

import (
	"context"
	"errors"
	"strings"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
)

const (
	xcuitestAutomationName = "XCUITest"

	iosDoubleTapOperation             = "ios_double_tap"
	iosTouchAndHoldOperation          = "ios_touch_and_hold"
	iosTwoFingerTapOperation          = "ios_two_finger_tap"
	iosDragFromToForDurationOperation = "ios_drag_from_to_for_duration"
)

// IOSDoubleTap 使用 XCUITest 在指定坐标执行双击。
//
// 这是 iOS/XCUITest 专有手势，不属于跨平台 W3C Actions。
func IOSDoubleTap(
	ctx context.Context,
	session *appium.Session,
	point appium.Point,
) error {
	return executeIOSGesture(
		ctx,
		session,
		iosDoubleTapOperation,
		"doubleTap",
		map[string]any{
			"x": point.X,
			"y": point.Y,
		},
	)
}

// IOSTouchAndHold 使用 XCUITest 在指定坐标执行按住手势。
//
// duration 使用 Go 的 time.Duration 表示，并以浮点秒数发送给 XCUITest Driver。
// 负数属于无效参数。
func IOSTouchAndHold(
	ctx context.Context,
	session *appium.Session,
	point appium.Point,
	duration time.Duration,
) error {
	seconds, err := iosDurationSeconds(
		iosTouchAndHoldOperation,
		duration,
	)
	if err != nil {
		return err
	}

	return executeIOSGesture(
		ctx,
		session,
		iosTouchAndHoldOperation,
		"touchAndHold",
		map[string]any{
			"x":        point.X,
			"y":        point.Y,
			"duration": seconds,
		},
	)
}

// IOSTwoFingerTap 使用 XCUITest 在当前应用元素上执行双指点击。
//
// 当前接口不接受元素参数。
// 如果后续需要元素级双指点击，应单独增加显式的 Element 版本，
// 而不是通过 nil 或可选参数改变本方法语义。
func IOSTwoFingerTap(
	ctx context.Context,
	session *appium.Session,
) error {
	return executeIOSGesture(
		ctx,
		session,
		iosTwoFingerTapOperation,
		"twoFingerTap",
		map[string]any{},
	)
}

// IOSDragFromToForDuration 使用 XCUITest 从 from 拖动到 to。
//
// pressDuration 表示在起点按住多久后开始拖动，
// 不是拖动过程本身持续的时间。
//
// XCUITest Driver 要求该值位于 0.5 秒到 60 秒之间。
func IOSDragFromToForDuration(
	ctx context.Context,
	session *appium.Session,
	from appium.Point,
	to appium.Point,
	pressDuration time.Duration,
) error {
	if pressDuration < 500*time.Millisecond ||
		pressDuration > 60*time.Second {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: iosDragFromToForDurationOperation,
			Message:   "press duration must be between 0.5 and 60 seconds",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	return executeIOSGesture(
		ctx,
		session,
		iosDragFromToForDurationOperation,
		"dragFromToForDuration",
		map[string]any{
			"fromX":    from.X,
			"fromY":    from.Y,
			"toX":      to.X,
			"toY":      to.Y,
			"duration": pressDuration.Seconds(),
		},
	)
}

// executeIOSGesture 校验 Session Driver 并执行一个 XCUITest mobile gesture。
func executeIOSGesture(
	ctx context.Context,
	session *appium.Session,
	operation string,
	method string,
	arguments map[string]any,
) error {
	if err := requireXCUITestSession(
		operation,
		session,
	); err != nil {
		return err
	}

	_, err := session.ExecuteScript(
		ctx,
		"mobile: "+method,
		[]any{arguments},
	)
	if err != nil {
		return iosOperationError(operation, err)
	}

	return nil
}

// requireXCUITestSession 校验当前 Session 是否属于 XCUITest Driver。
//
// API 名称中的 IOS 前缀负责在编译期 API 表面表达平台属性；
// 此处的 automationName 检查只负责阻止运行时传入错误的 Session。
func requireXCUITestSession(
	operation string,
	session *appium.Session,
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
	if automationName == "" {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: operation,
			Message:   "session does not have an automation name",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	if !strings.EqualFold(
		automationName,
		xcuitestAutomationName,
	) {
		return &appium.Error{
			Code:      appium.CodeUnsupported,
			Operation: operation,
			Message:   "command requires XCUITest session",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	return nil
}

// iosDurationSeconds 将 Go Duration 转换为 XCUITest mobile gesture 使用的浮点秒数。
//
// XCUITest 此类接口本身接受浮点秒数，因此这里不进行整数毫秒截断。
func iosDurationSeconds(
	operation string,
	duration time.Duration,
) (float64, error) {
	if duration < 0 {
		return 0, &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: operation,
			Message:   "duration must not be negative",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	return duration.Seconds(), nil
}

// iosOperationError 将底层 ExecuteScript 错误重新标记为具体的 iOS 操作。
//
// Delivery、RemoteCode、StatusCode 等事实保持不变。
func iosOperationError(
	operation string,
	err error,
) error {
	var appiumErr *appium.Error
	if !errors.As(err, &appiumErr) {
		return err
	}

	cloned := *appiumErr
	cloned.Operation = operation

	return &cloned
}
