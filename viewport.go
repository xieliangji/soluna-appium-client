package soluna_appium_client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

const (
	getViewportRectOperation = "get_viewport_rect"
	viewportRectScript       = "mobile: viewportRect"
)

// ViewportRect 获取当前 Driver 报告的 viewport 像素几何快照。
//
// 该方法只支持创建 Session 后远端确认的 XCUITest 和 UiAutomator2
// automationName。返回值使用 Driver 报告的整数像素单位，不执行 scale、
// density、orientation 或 status bar 转换，也不绑定任何 Screenshot。
func (s *Session) ViewportRect(
	ctx context.Context,
) (PixelRect, error) {
	client, err := s.commandClient(
		getViewportRectOperation,
	)
	if err != nil {
		return PixelRect{}, err
	}

	// 两个 Driver 对外都通过同一个 mobile: viewportRect 脚本暴露能力；
	// 这里依据远端确认的 automationName 选择允许的 Driver 映射，
	// 不使用请求中的原始 Capability 或 Runtime Discovery 结果。
	switch s.automationName {
	case "XCUITest":
		// XCUITest Driver 的内部 Execute Method 为 getViewportRect。
	case "UiAutomator2":
		// UiAutomator2 Driver 的内部 Execute Method 为 mobileViewPortRect。
	case "":
		return PixelRect{}, &Error{
			Code:      CodeInvalidArgument,
			Operation: getViewportRectOperation,
			Message:   "session is not usable for viewport rect",
			Delivery:  DeliveryNotSent,
		}
	default:
		return PixelRect{}, &Error{
			Code:      CodeUnsupported,
			Operation: getViewportRectOperation,
			Message: fmt.Sprintf(
				"viewport rect is unsupported for automationName %q",
				s.automationName,
			),
			Delivery: DeliveryNotSent,
		}
	}

	var rect PixelRect

	err = executeScriptCommand(
		ctx,
		client,
		getViewportRectOperation,
		s.id,
		viewportRectScript,
		nil,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			decoded, decodeErr := decodePixelRect(
				ctx,
				value,
			)
			if decodeErr != nil {
				return decodeErr
			}

			rect = decoded
			return nil
		},
	)
	if err != nil {
		return PixelRect{}, err
	}

	return rect, nil
}

// decodePixelRect 严格解码 Driver 返回的 viewport 像素矩形。
//
// 四个已知字段必须是 JSON number，并且能够无损表示为当前平台的 int。
// 解码成功前不会返回部分构造的 PixelRect。
func decodePixelRect(
	ctx context.Context,
	value json.RawMessage,
) (PixelRect, error) {
	if err := ctx.Err(); err != nil {
		return PixelRect{}, err
	}

	// Decode into an open map so JSON object keys are looked up with exact
	// protocol spelling. encoding/json's struct-field matching otherwise
	// accepts case variants such as "Left" for the required "left" field.
	var payload map[string]json.RawMessage

	if err := json.Unmarshal(value, &payload); err != nil {
		return PixelRect{}, fmt.Errorf(
			"decode viewport rect response: %w",
			err,
		)
	}

	left, err := decodePixelRectInt(
		ctx,
		"left",
		payload["left"],
	)
	if err != nil {
		return PixelRect{}, err
	}

	top, err := decodePixelRectInt(
		ctx,
		"top",
		payload["top"],
	)
	if err != nil {
		return PixelRect{}, err
	}

	width, err := decodePixelRectInt(
		ctx,
		"width",
		payload["width"],
	)
	if err != nil {
		return PixelRect{}, err
	}

	height, err := decodePixelRectInt(
		ctx,
		"height",
		payload["height"],
	)
	if err != nil {
		return PixelRect{}, err
	}

	if left < 0 || top < 0 {
		return PixelRect{}, errors.New(
			"viewport rect contains a negative origin",
		)
	}
	if width <= 0 || height <= 0 {
		return PixelRect{}, errors.New(
			"viewport rect must have positive size",
		)
	}

	maxInt := int(^uint(0) >> 1)
	if left > maxInt-width || top > maxInt-height {
		return PixelRect{}, errors.New(
			"viewport rect endpoint overflows int",
		)
	}

	if err := ctx.Err(); err != nil {
		return PixelRect{}, err
	}

	return PixelRect{
		X:      left,
		Y:      top,
		Width:  width,
		Height: height,
	}, nil
}

// decodePixelRectInt 解码一个必须为 JSON number 的 int 字段。
func decodePixelRectInt(
	ctx context.Context,
	field string,
	value json.RawMessage,
) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if len(value) == 0 {
		return 0, fmt.Errorf(
			"viewport rect response does not contain %s",
			field,
		)
	}

	decoder := json.NewDecoder(
		bytes.NewReader(value),
	)
	decoder.UseNumber()

	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return 0, fmt.Errorf(
			"decode viewport rect %s: %w",
			field,
			err,
		)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return 0, fmt.Errorf(
				"viewport rect %s contains multiple values",
				field,
			)
		}
		return 0, fmt.Errorf(
			"decode viewport rect %s: %w",
			field,
			err,
		)
	}

	number, ok := decoded.(json.Number)
	if !ok {
		return 0, fmt.Errorf(
			"viewport rect %s must be a JSON number",
			field,
		)
	}

	parsed, err := strconv.ParseInt(
		number.String(),
		10,
		strconv.IntSize,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"viewport rect %s must be an integer representable by int: %w",
			field,
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return 0, err
	}

	return int(parsed), nil
}
