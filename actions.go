package soluna_appium_client

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	performActionsOperation = "perform_actions"
	releaseActionsOperation = "release_actions"
	tapOperation            = "tap"
	longPressOperation      = "long_press"
	swipeOperation          = "swipe"
)

// ActionSequence 表示一组连续执行的触摸指针动作。
//
// 每个 Sequence 对应一个独立的触摸指针。
// 多指操作可以通过同时提交多个 ActionSequence 实现。
type ActionSequence struct {
	id      string
	actions []TouchAction
}

// TouchAction 表示 ActionSequence 中的一个触摸动作。
//
// TouchAction 只能通过本包提供的构造函数创建，
// 调用方不需要感知 W3C Actions 的底层字段结构。
type TouchAction struct {
	kind     touchActionKind
	point    Point
	duration time.Duration
}

// touchActionKind 表示内部使用的触摸动作类型。
type touchActionKind uint8

const (
	touchActionMove touchActionKind = iota
	touchActionDown
	touchActionUp
	touchActionPause
)

// TouchSequence 创建一个触摸指针动作序列。
//
// id 用于区分同一次 W3C Actions 请求中的不同触摸指针。
// 单指操作只需要一个 Sequence，多指操作应为每个触摸指针使用不同的 id。
func TouchSequence(id string, actions ...TouchAction) ActionSequence {
	return ActionSequence{
		id:      id,
		actions: append([]TouchAction(nil), actions...),
	}
}

// MoveTo 创建一个移动到指定 viewport 坐标的触摸动作。
//
// duration 表示移动过程持续的时间。
// duration 为零表示立即移动，负数属于无效参数。
func MoveTo(point Point, duration time.Duration) TouchAction {
	return TouchAction{
		kind:     touchActionMove,
		point:    point,
		duration: duration,
	}
}

// TouchDown 创建一个按下触摸指针的动作。
func TouchDown() TouchAction {
	return TouchAction{
		kind: touchActionDown,
	}
}

// TouchUp 创建一个释放触摸指针的动作。
func TouchUp() TouchAction {
	return TouchAction{
		kind: touchActionUp,
	}
}

// Pause 创建一个保持当前触摸状态的暂停动作。
//
// duration 表示暂停持续的时间。
// duration 为零表示不等待，负数属于无效参数。
func Pause(duration time.Duration) TouchAction {
	return TouchAction{
		kind:     touchActionPause,
		duration: duration,
	}
}

// PerformActions 执行一组 W3C Touch Pointer Actions。
//
// 每个 ActionSequence 表示一个独立的触摸指针。
// 客户端固定使用 pointerType=touch 和 viewport 坐标系。
func (s *Session) PerformActions(
	ctx context.Context,
	sequences ...ActionSequence,
) error {
	return s.performActions(
		ctx,
		performActionsOperation,
		sequences,
	)
}

// ReleaseActions 释放当前 Session 中仍处于按下状态的输入源。
//
// 正常完成且已经包含 TouchUp 的动作序列通常不需要显式调用。
// 该方法主要用于调用方主动构造未释放输入源的低层动作场景。
func (s *Session) ReleaseActions(ctx context.Context) error {
	client, err := s.commandClient(
		releaseActionsOperation,
	)
	if err != nil {
		return err
	}

	command, err := wire.NewCommand(
		releaseActionsOperation,
		http.MethodDelete,
		"session",
		s.id,
		"actions",
	)
	if err != nil {
		return actionCommandDefinitionError(
			releaseActionsOperation,
			"release actions command definition is invalid",
			err,
		)
	}

	return client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		decodeNullResponse,
	)
}

// Tap 在指定 viewport 坐标执行一次点击。
func (s *Session) Tap(
	ctx context.Context,
	point Point,
) error {
	return s.performActions(
		ctx,
		tapOperation,
		[]ActionSequence{
			TouchSequence(
				"finger",
				MoveTo(point, 0),
				TouchDown(),
				TouchUp(),
			),
		},
	)
}

// LongPress 在指定 viewport 坐标执行一次长按。
//
// duration 表示指针保持按下状态的时间。
// duration 必须是非负且能够精确表示为整数毫秒的 time.Duration。
func (s *Session) LongPress(
	ctx context.Context,
	point Point,
	duration time.Duration,
) error {
	return s.performActions(
		ctx,
		longPressOperation,
		[]ActionSequence{
			TouchSequence(
				"finger",
				MoveTo(point, 0),
				TouchDown(),
				Pause(duration),
				TouchUp(),
			),
		},
	)
}

// Swipe 从 start 滑动到 end。
//
// duration 表示从起点移动到终点所持续的时间。
// duration 必须是非负且能够精确表示为整数毫秒的 time.Duration。
func (s *Session) Swipe(
	ctx context.Context,
	start Point,
	end Point,
	duration time.Duration,
) error {
	return s.performActions(
		ctx,
		swipeOperation,
		[]ActionSequence{
			TouchSequence(
				"finger",
				MoveTo(start, 0),
				TouchDown(),
				MoveTo(end, duration),
				TouchUp(),
			),
		},
	)
}

// performActions 编码并执行一组触摸指针动作。
func (s *Session) performActions(
	ctx context.Context,
	operation string,
	sequences []ActionSequence,
) error {
	client, err := s.commandClient(operation)
	if err != nil {
		return err
	}

	request, err := encodeActionSequences(
		operation,
		sequences,
	)
	if err != nil {
		return err
	}

	command, err := wire.NewCommand(
		operation,
		http.MethodPost,
		"session",
		s.id,
		"actions",
	)
	if err != nil {
		return actionCommandDefinitionError(
			operation,
			"perform actions command definition is invalid",
			err,
		)
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

// actionRequest 表示 Perform Actions 的 W3C 请求体。
type actionRequest struct {
	Actions []pointerActionSequence `json:"actions"`
}

// pointerActionSequence 表示一个 W3C Touch Pointer 输入源。
type pointerActionSequence struct {
	Type       string                  `json:"type"`
	ID         string                  `json:"id"`
	Parameters pointerActionParameters `json:"parameters"`
	Actions    []pointerAction         `json:"actions"`
}

// pointerActionParameters 表示 Pointer 输入源参数。
type pointerActionParameters struct {
	PointerType string `json:"pointerType"`
}

// pointerAction 表示编码后的单个 W3C Pointer Action。
type pointerAction struct {
	Type     string `json:"type"`
	Duration *int64 `json:"duration,omitempty"`
	X        *int   `json:"x,omitempty"`
	Y        *int   `json:"y,omitempty"`
	Origin   string `json:"origin,omitempty"`
	Button   *int   `json:"button,omitempty"`
}

// encodeActionSequences 校验并编码触摸动作序列。
func encodeActionSequences(
	operation string,
	sequences []ActionSequence,
) (actionRequest, error) {
	if len(sequences) == 0 {
		return actionRequest{}, actionArgumentError(
			operation,
			"actions must contain at least one sequence",
		)
	}

	ids := make(map[string]struct{}, len(sequences))
	encoded := make(
		[]pointerActionSequence,
		0,
		len(sequences),
	)

	for _, sequence := range sequences {
		if sequence.id == "" {
			return actionRequest{}, actionArgumentError(
				operation,
				"action sequence ID is empty",
			)
		}

		if _, exists := ids[sequence.id]; exists {
			return actionRequest{}, actionArgumentError(
				operation,
				"action sequence IDs must be unique",
			)
		}
		ids[sequence.id] = struct{}{}

		if len(sequence.actions) == 0 {
			return actionRequest{}, actionArgumentError(
				operation,
				"action sequence must contain at least one action",
			)
		}

		actions := make(
			[]pointerAction,
			0,
			len(sequence.actions),
		)

		for _, action := range sequence.actions {
			encodedAction, err := encodeTouchAction(
				operation,
				action,
			)
			if err != nil {
				return actionRequest{}, err
			}

			actions = append(actions, encodedAction)
		}

		encoded = append(
			encoded,
			pointerActionSequence{
				Type: "pointer",
				ID:   sequence.id,
				Parameters: pointerActionParameters{
					PointerType: "touch",
				},
				Actions: actions,
			},
		)
	}

	return actionRequest{
		Actions: encoded,
	}, nil
}

// encodeTouchAction 将公共 TouchAction 编码为 W3C Pointer Action。
func encodeTouchAction(
	operation string,
	action TouchAction,
) (pointerAction, error) {
	switch action.kind {
	case touchActionMove:
		duration, err := actionDurationMilliseconds(
			operation,
			action.duration,
		)
		if err != nil {
			return pointerAction{}, err
		}

		x := action.point.X
		y := action.point.Y

		return pointerAction{
			Type:     "pointerMove",
			Duration: &duration,
			X:        &x,
			Y:        &y,
			Origin:   "viewport",
		}, nil

	case touchActionDown:
		button := 0

		return pointerAction{
			Type:   "pointerDown",
			Button: &button,
		}, nil

	case touchActionUp:
		button := 0

		return pointerAction{
			Type:   "pointerUp",
			Button: &button,
		}, nil

	case touchActionPause:
		duration, err := actionDurationMilliseconds(
			operation,
			action.duration,
		)
		if err != nil {
			return pointerAction{}, err
		}

		return pointerAction{
			Type:     "pause",
			Duration: &duration,
		}, nil

	default:
		return pointerAction{}, actionArgumentError(
			operation,
			"touch action type is invalid",
		)
	}
}

// actionDurationMilliseconds 将动作时间转换为 W3C 使用的整数毫秒。
func actionDurationMilliseconds(
	operation string,
	duration time.Duration,
) (int64, error) {
	if duration < 0 {
		return 0, actionArgumentError(
			operation,
			"action duration must not be negative",
		)
	}

	if duration%time.Millisecond != 0 {
		return 0, actionArgumentError(
			operation,
			"action duration must be an exact number of milliseconds",
		)
	}

	return duration.Milliseconds(), nil
}

// actionArgumentError 创建 Actions 参数错误。
func actionArgumentError(
	operation string,
	message string,
) error {
	return &Error{
		Code:      CodeInvalidArgument,
		Operation: operation,
		Message:   message,
		Delivery:  DeliveryNotSent,
	}
}

// actionCommandDefinitionError 创建 Actions 命令定义错误。
func actionCommandDefinitionError(
	operation string,
	message string,
	cause error,
) error {
	if cause == nil {
		cause = errors.New("unknown command definition error")
	}

	return &Error{
		Code:      CodeInvalidConfig,
		Operation: operation,
		Message:   message,
		Delivery:  DeliveryNotSent,
		Cause:     cause,
	}
}
