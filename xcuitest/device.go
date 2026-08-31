package xcuitest

import (
	"context"
	"encoding/json"
	"errors"

	appium "github.com/xieliangji/soluna-appium-client"
)

const (
	iosPressButtonOperation = "ios_press_button"
	iosPressButtonScript    = "mobile: pressButton"

	iosDeviceScreenInfoOperation = "ios_device_screen_info"
	iosDeviceScreenInfoScript    = "mobile: deviceScreenInfo"
)

// ScreenSize 表示 XCUITest 返回的屏幕尺寸。
// 单位保持 Driver 返回的坐标单位，不执行像素换算。
type ScreenSize struct {
	Width  float64
	Height float64
}

// ScreenInfo 表示 XCUITest Driver 返回的 iOS 屏幕信息。
//
// Scale 是截图像素与 XCTest 屏幕坐标之间的重要设备事实。
// 本类型只暴露当前协议中稳定定义的字段。
type ScreenInfo struct {
	StatusBarSize ScreenSize
	Scale         float64
}

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

	_, err := session.ExecuteScriptWithOperation(
		ctx,
		iosPressButtonOperation,
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

// IOSDeviceScreenInfo 获取当前 iOS 设备的屏幕信息。
//
// 返回值来自 XCUITest Driver 的 mobile: deviceScreenInfo。
// 客户端不会使用 Scale 自动修改 WindowRect、Element Rect、
// Screenshot 或 W3C Actions 的坐标语义。
func IOSDeviceScreenInfo(
	ctx context.Context,
	session *appium.Session,
) (ScreenInfo, error) {
	if err := requireXCUITestSession(
		session,
		iosDeviceScreenInfoOperation,
	); err != nil {
		return ScreenInfo{}, err
	}

	value, err := session.ExecuteScriptWithOperation(
		ctx,
		iosDeviceScreenInfoOperation,
		iosDeviceScreenInfoScript,
		nil,
	)
	if err != nil {
		return ScreenInfo{}, err
	}

	info, err := decodeIOSDeviceScreenInfo(value)
	if err != nil {
		return ScreenInfo{}, &appium.Error{
			Code:      appium.CodeResponseInvalid,
			Operation: iosDeviceScreenInfoOperation,
			Message:   "invalid iOS device screen info response",
			Delivery:  appium.DeliveryAcknowledged,
			Cause:     err,
		}
	}

	return info, nil
}

func decodeIOSDeviceScreenInfo(
	value json.RawMessage,
) (ScreenInfo, error) {
	var payload struct {
		StatusBarSize *struct {
			Width  *float64 `json:"width"`
			Height *float64 `json:"height"`
		} `json:"statusBarSize"`

		Scale *float64 `json:"scale"`
	}

	if err := json.Unmarshal(
		value,
		&payload,
	); err != nil {
		return ScreenInfo{}, err
	}

	if payload.StatusBarSize == nil {
		return ScreenInfo{}, errors.New(
			"iOS device screen info does not contain statusBarSize",
		)
	}

	if payload.StatusBarSize.Width == nil {
		return ScreenInfo{}, errors.New(
			"iOS device screen info statusBarSize does not contain width",
		)
	}

	if payload.StatusBarSize.Height == nil {
		return ScreenInfo{}, errors.New(
			"iOS device screen info statusBarSize does not contain height",
		)
	}

	if payload.Scale == nil {
		return ScreenInfo{}, errors.New(
			"iOS device screen info does not contain scale",
		)
	}

	return ScreenInfo{
		StatusBarSize: ScreenSize{
			Width:  *payload.StatusBarSize.Width,
			Height: *payload.StatusBarSize.Height,
		},
		Scale: *payload.Scale,
	}, nil
}
