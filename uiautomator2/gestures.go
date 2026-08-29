package uiautomator2

import (
	"context"
	"errors"
	"strings"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
)

const (
	uiAutomator2AutomationName = "UiAutomator2"

	androidClickGestureOperation       = "android_click_gesture"
	androidDoubleClickGestureOperation = "android_double_click_gesture"
	androidLongClickGestureOperation   = "android_long_click_gesture"
	androidDragGestureOperation        = "android_drag_gesture"
)

// AndroidClickGesture 使用 UiAutomator2 在指定坐标执行点击。
//
// 这是 Android/UiAutomator2 专有手势。
// 与根包的 W3C Tap 使用不同的 Driver 实现。
func AndroidClickGesture(
	ctx context.Context,
	session *appium.Session,
	point appium.Point,
) error {
	return executeAndroidGesture(
		ctx,
		session,
		androidClickGestureOperation,
		"clickGesture",
		map[string]any{
			"x": point.X,
			"y": point.Y,
		},
	)
}

// AndroidDoubleClickGesture 使用 UiAutomator2 在指定坐标执行双击。
func AndroidDoubleClickGesture(
	ctx context.Context,
	session *appium.Session,
	point appium.Point,
) error {
	return executeAndroidGesture(
		ctx,
		session,
		androidDoubleClickGestureOperation,
		"doubleClickGesture",
		map[string]any{
			"x": point.X,
			"y": point.Y,
		},
	)
}

// AndroidLongClickGesture 使用 UiAutomator2 在指定坐标执行长按。
//
// duration 为零时不显式发送 duration，
// 由 UiAutomator2 Driver 使用其默认值。
//
// 非零 duration 必须为正数且能够精确表示为整数毫秒。
func AndroidLongClickGesture(
	ctx context.Context,
	session *appium.Session,
	point appium.Point,
	duration time.Duration,
) error {
	millis, specified, err := androidOptionalDurationMilliseconds(
		androidLongClickGestureOperation,
		duration,
	)
	if err != nil {
		return err
	}

	arguments := map[string]any{
		"x": point.X,
		"y": point.Y,
	}

	if specified {
		arguments["duration"] = millis
	}

	return executeAndroidGesture(
		ctx,
		session,
		androidLongClickGestureOperation,
		"longClickGesture",
		arguments,
	)
}

// AndroidDragGesture 使用 UiAutomator2 从 from 拖动到 to。
//
// speed 表示拖动速度，单位为像素/秒。
// speed 为零时不显式指定，由 UiAutomator2 Driver 使用默认速度。
// 负数属于无效参数。
func AndroidDragGesture(
	ctx context.Context,
	session *appium.Session,
	from appium.Point,
	to appium.Point,
	speed int,
) error {
	if speed < 0 {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: androidDragGestureOperation,
			Message:   "drag speed must not be negative",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	arguments := map[string]any{
		"startX": from.X,
		"startY": from.Y,
		"endX":   to.X,
		"endY":   to.Y,
	}

	if speed > 0 {
		arguments["speed"] = speed
	}

	return executeAndroidGesture(
		ctx,
		session,
		androidDragGestureOperation,
		"dragGesture",
		arguments,
	)
}

// executeAndroidGesture 校验 Session Driver 并执行一个 UiAutomator2 mobile gesture。
func executeAndroidGesture(
	ctx context.Context,
	session *appium.Session,
	operation string,
	method string,
	arguments map[string]any,
) error {
	if err := requireUiAutomator2Session(
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
		return androidOperationError(operation, err)
	}

	return nil
}

// requireUiAutomator2Session 校验当前 Session 是否属于 UiAutomator2 Driver。
//
// API 名称中的 Android 前缀负责在 API 表面表达平台属性；
// 此处的 automationName 检查只负责阻止运行时传入错误的 Session。
func requireUiAutomator2Session(
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
		uiAutomator2AutomationName,
	) {
		return &appium.Error{
			Code:      appium.CodeUnsupported,
			Operation: operation,
			Message:   "command requires UiAutomator2 session",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	return nil
}

// androidOptionalDurationMilliseconds 将可选 Duration 转换为
// UiAutomator2 gesture 使用的整数毫秒。
//
// 零表示不显式指定该参数，由 Driver 使用默认值。
func androidOptionalDurationMilliseconds(
	operation string,
	duration time.Duration,
) (millis int64, specified bool, err error) {
	if duration < 0 {
		return 0, false, &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: operation,
			Message:   "duration must not be negative",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	if duration == 0 {
		return 0, false, nil
	}

	if duration%time.Millisecond != 0 {
		return 0, false, &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: operation,
			Message:   "duration must be an exact number of milliseconds",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	return duration.Milliseconds(), true, nil
}

// androidOperationError 将底层 ExecuteScript 错误重新标记为具体的 Android 操作。
//
// Delivery、RemoteCode、StatusCode 等事实保持不变。
func androidOperationError(
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
