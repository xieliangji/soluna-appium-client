package xcuitest

import (
	"bytes"
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

	var info ScreenInfo

	err := session.ExecuteScriptWithOperationAndDecode(
		ctx,
		iosDeviceScreenInfoOperation,
		iosDeviceScreenInfoScript,
		nil,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			decoded, decodeErr := decodeIOSDeviceScreenInfo(
				ctx,
				value,
			)
			if decodeErr != nil {
				return decodeErr
			}

			info = decoded
			return nil
		},
	)
	if err != nil {
		return ScreenInfo{}, err
	}

	return info, nil
}

func decodeIOSDeviceScreenInfo(
	ctx context.Context,
	value json.RawMessage,
) (ScreenInfo, error) {
	if err := ctx.Err(); err != nil {
		return ScreenInfo{}, err
	}

	// Decode into an open map so JSON object keys are looked up with exact
	// protocol spelling. encoding/json's struct-field matching otherwise
	// accepts case variants such as "Scale" for the required "scale" field.
	var payload map[string]json.RawMessage

	if err := json.Unmarshal(
		value,
		&payload,
	); err != nil {
		return ScreenInfo{}, err
	}

	statusBarValue, ok := payload["statusBarSize"]
	if !ok || isJSONNull(statusBarValue) {
		return ScreenInfo{}, errors.New(
			"iOS device screen info does not contain statusBarSize",
		)
	}

	var statusBarPayload map[string]json.RawMessage
	if err := json.Unmarshal(
		statusBarValue,
		&statusBarPayload,
	); err != nil {
		return ScreenInfo{}, err
	}

	width, err := decodeIOSDeviceScreenInfoNumber(
		statusBarPayload,
		"width",
	)
	if err != nil {
		return ScreenInfo{}, errors.New(
			"iOS device screen info statusBarSize does not contain width",
		)
	}

	height, err := decodeIOSDeviceScreenInfoNumber(
		statusBarPayload,
		"height",
	)
	if err != nil {
		return ScreenInfo{}, errors.New(
			"iOS device screen info statusBarSize does not contain height",
		)
	}

	scale, err := decodeIOSDeviceScreenInfoNumber(
		payload,
		"scale",
	)
	if err != nil {
		return ScreenInfo{}, errors.New(
			"iOS device screen info does not contain scale",
		)
	}

	if err := ctx.Err(); err != nil {
		return ScreenInfo{}, err
	}

	return ScreenInfo{
		StatusBarSize: ScreenSize{
			Width:  width,
			Height: height,
		},
		Scale: scale,
	}, nil
}

func decodeIOSDeviceScreenInfoNumber(
	payload map[string]json.RawMessage,
	field string,
) (float64, error) {
	value, ok := payload[field]
	if !ok || isJSONNull(value) {
		return 0, errors.New("required number is missing")
	}

	var decoded float64
	if err := json.Unmarshal(value, &decoded); err != nil {
		return 0, err
	}

	return decoded, nil
}

func isJSONNull(value json.RawMessage) bool {
	return bytes.Equal(
		bytes.TrimSpace(value),
		[]byte("null"),
	)
}
