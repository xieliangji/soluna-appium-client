package appium

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	getOrientationOperation = "get_orientation"
	setOrientationOperation = "set_orientation"
)

// Orientation 表示 Appium 报告的屏幕方向分类。
//
// 该类型只区分竖屏和横屏，不表示横屏左右、倒置竖屏或空间 Rotation。
// 零值不是合法方向，作为 SetOrientation 参数时会在发送前被拒绝。
type Orientation string

const (
	// OrientationPortrait 表示 Appium PORTRAIT 屏幕方向。
	OrientationPortrait Orientation = "PORTRAIT"

	// OrientationLandscape 表示 Appium LANDSCAPE 屏幕方向。
	OrientationLandscape Orientation = "LANDSCAPE"
)

// Orientation 读取当前 Driver 报告的屏幕方向快照。
//
// 返回值只可能是 OrientationPortrait 或 OrientationLandscape。
// 每次调用都会读取远端，客户端不缓存之前的方向。
func (s *Session) Orientation(ctx context.Context) (Orientation, error) {
	client, err := s.commandClient(getOrientationOperation)
	if err != nil {
		return "", err
	}

	command, err := wire.NewCommand(
		getOrientationOperation,
		http.MethodGet,
		"session",
		s.id,
		"appium",
		"device",
		"orientation",
	)
	if err != nil {
		return "", commandDefinitionError(
			getOrientationOperation,
			"get orientation command definition is invalid",
			err,
		)
	}

	var orientation Orientation
	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decodeOrientation(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			orientation = decoded
			return nil
		},
	)
	if err != nil {
		return "", err
	}

	return orientation, nil
}

// SetOrientation 请求将当前屏幕设置为指定方向。
//
// orientation 必须是 OrientationPortrait 或 OrientationLandscape。
// 成功只表示远端接受了本次设置命令；客户端不缓存、重试或
// 自动再次读取方向。
func (s *Session) SetOrientation(
	ctx context.Context,
	orientation Orientation,
) error {
	client, err := s.commandClient(setOrientationOperation)
	if err != nil {
		return err
	}

	if !validOrientation(orientation) {
		return &Error{
			Code:      CodeInvalidArgument,
			Operation: setOrientationOperation,
			Message:   "orientation must be PORTRAIT or LANDSCAPE",
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		setOrientationOperation,
		http.MethodPost,
		"session",
		s.id,
		"appium",
		"device",
		"orientation",
	)
	if err != nil {
		return commandDefinitionError(
			setOrientationOperation,
			"set orientation command definition is invalid",
			err,
		)
	}

	request := struct {
		Orientation Orientation `json:"orientation"`
	}{
		Orientation: orientation,
	}

	return client.executeCommand(
		ctx,
		command,
		request,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		decodeNullResponse,
	)
}

// decodeOrientation 严格解码 Appium Orientation 命令的成功值。
func decodeOrientation(
	ctx context.Context,
	value json.RawMessage,
) (Orientation, error) {
	if ctx == nil {
		return "", errors.New("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	decoded, err := codec.DecodeJSONString(ctx, value)
	if err != nil {
		return "", fmt.Errorf("decode orientation response value: %w", err)
	}

	orientation := Orientation(decoded)
	if !validOrientation(orientation) {
		return "", errors.New(
			"orientation response value must be PORTRAIT or LANDSCAPE",
		)
	}

	return orientation, nil
}

// validOrientation 校验一个值是否属于 Appium 定义的二维屏幕方向集合。
func validOrientation(orientation Orientation) bool {
	switch orientation {
	case OrientationPortrait, OrientationLandscape:
		return true
	default:
		return false
	}
}
