package xcuitest

import (
	"context"
	"encoding/json"
	"errors"
	"math"

	appium "github.com/xieliangji/soluna-appium-client"
)

const (
	iosSelectPickerWheelValueOperation = "ios_select_picker_wheel_value"
	iosSelectPickerWheelValueScript    = "mobile: selectPickerWheelValue"
)

// PickerWheelDirection 表示 Picker Wheel 选择值的方向。
type PickerWheelDirection string

const (
	// PickerWheelNext 选择下一个值。
	PickerWheelNext PickerWheelDirection = "next"
	// PickerWheelPrevious 选择上一个值。
	PickerWheelPrevious PickerWheelDirection = "previous"
)

// PickerWheelOffset 表示 Picker Wheel 点击位置相对控件高度的比例。
// 有效范围为 (0, 0.5]；零值无效，不使用 Driver 默认值。
// 较大的比例可能一次跳过多个值，较小的比例可能无法使值改变。
type PickerWheelOffset float64

type pickerWheelSelectionRequest struct {
	ElementID string               `json:"elementId"`
	Order     PickerWheelDirection `json:"order"`
	Offset    PickerWheelOffset    `json:"offset"`
}

// IOSSelectPickerWheelValue 按指定方向请求改变原生 Picker Wheel 的值。
//
// element 必须属于 session；direction 只能是 PickerWheelNext 或
// PickerWheelPrevious；offset 必须是有限的 (0, 0.5] 数值。客户端只发送
// 一次固定 Execute Method 请求。nil 或未初始化的 Session/Element 返回参数错误。
// 元素类型和是否改变值由远端判断，成功不保证恰好移动一项或达到某个目标值。
// SDK 不附加等待、重试或 Swipe；WDA 内部可能等待观察值改变。
// 具体 iOS、Driver/WDA 和 Host 条件见兼容性文档，客户端不探测或推断版本支持。
func IOSSelectPickerWheelValue(
	ctx context.Context,
	session *appium.Session,
	element *appium.Element,
	direction PickerWheelDirection,
	offset PickerWheelOffset,
) error {
	if err := requireXCUITestSession(session, iosSelectPickerWheelValueOperation); err != nil {
		return err
	}
	if !element.BelongsTo(session) {
		return pickerWheelArgumentError("element does not belong to session")
	}
	if direction != PickerWheelNext && direction != PickerWheelPrevious {
		return pickerWheelArgumentError("unsupported picker wheel direction")
	}
	value := float64(offset)
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || value > 0.5 {
		return pickerWheelArgumentError("picker wheel offset must be a finite value in (0, 0.5]")
	}

	return session.ExecuteScriptWithOperationAndDecode(
		ctx,
		iosSelectPickerWheelValueOperation,
		iosSelectPickerWheelValueScript,
		[]any{pickerWheelSelectionRequest{
			ElementID: element.ID(),
			Order:     direction,
			Offset:    offset,
		}},
		decodePickerWheelValueNull,
	)
}

func pickerWheelArgumentError(message string) error {
	return &appium.Error{
		Code:      appium.CodeInvalidArgument,
		Operation: iosSelectPickerWheelValueOperation,
		Message:   message,
		Delivery:  appium.DeliveryNotSent,
	}
}

func decodePickerWheelValueNull(ctx context.Context, value json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !isJSONNull(value) {
		return errors.New("picker wheel response value must be null")
	}
	return nil
}
