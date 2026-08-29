package soluna_appium_client

import "time"

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
