package wait

import (
	"context"
	"errors"
	"reflect"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
)

const (
	waitElementOperation  = "wait_element"
	waitElementsOperation = "wait_elements"
)

// Element 在调用方 context 允许的期限内等待一次元素查找成功。
//
// finder 可以是根包的 *appium.Session 或 *appium.Element，分别表示
// Session 级查找和元素后代查找；也可以是满足同一方法签名的本地实现。
// 查找会先立即执行；只有根包明确报告 CodeElementNotFound 的结果会被
// 当作暂态并按 interval 重试。其他错误（包括 stale、Session 丢失、参数、
// 响应和传输错误）会立即原样返回。finder 返回 nil 元素而没有错误属于
// 本地 finder 契约错误，不会伪造远端响应或 Delivery 状态。
//
// ctx 必须非 nil，interval 必须为正数；其他参数校验和 context 的语义与
// Until 相同。等待到 context 结束时，若已有暂态未找到错误，会通过
// errors.Join 同时保留 context 结果和最后一次查找错误；context 命令错误
// 作为主错误排在前面，调用方可以用 errors.Is 检查 context，用
// appium.IsErrorCode 检查错误树中的两个错误码。
// Element 不会重新定位已经返回的 Element，也不会恢复 stale 引用。
func Element(
	ctx context.Context,
	interval time.Duration,
	finder interface {
		Find(context.Context, appium.Locator) (*appium.Element, error)
	},
	locator appium.Locator,
) (*appium.Element, error) {
	var found *appium.Element
	var transientErr error

	err := Until(
		ctx,
		interval,
		func(ctx context.Context) (bool, error) {
			if isNilFinder(finder) {
				return false, invalidFinderError(
					waitElementOperation,
				)
			}

			candidate, findErr := finder.Find(ctx, locator)
			if findErr != nil {
				if isTransientNotFound(findErr) {
					transientErr = findErr
					return false, nil
				}

				return false, findErr
			}

			if candidate == nil {
				return false, invalidElementResultError()
			}

			found = candidate
			return true, nil
		},
	)

	if err != nil {
		return nil, preserveTransientError(err, transientErr)
	}

	return found, nil
}

// Elements 在调用方 context 允许的期限内等待多元素查找返回至少一个元素。
//
// finder 可以是根包的 *appium.Session 或 *appium.Element，分别表示
// Session 级查找和元素后代查找；也可以是满足同一方法签名的本地实现。
// 查找会先立即执行；空集合以及根包明确报告 CodeElementNotFound 的结果
// 会按 interval 重试。其他错误（包括 stale、Session 丢失、参数、响应和
// 传输错误）会立即原样返回。FindElements 返回包含 nil 元素的集合属于
// 本地 finder 契约错误，不会伪造远端响应或 Delivery 状态。
//
// ctx 必须非 nil，interval 必须为正数；其他参数校验和 context 的语义与
// Until 相同。若查找一直只返回空集合，底层 FindElements 没有产生错误，
// context 结束时返回 context 错误；若期间收到
// CodeElementNotFound，则同时保留 context 结果和最后一次查找错误。
// Elements 不会重新定位已经返回的 Element，也不会恢复 stale 引用。
func Elements(
	ctx context.Context,
	interval time.Duration,
	finder interface {
		FindElements(context.Context, appium.Locator) ([]*appium.Element, error)
	},
	locator appium.Locator,
) ([]*appium.Element, error) {
	var found []*appium.Element
	var transientErr error

	err := Until(
		ctx,
		interval,
		func(ctx context.Context) (bool, error) {
			if isNilFinder(finder) {
				return false, invalidFinderError(
					waitElementsOperation,
				)
			}

			candidates, findErr := finder.FindElements(ctx, locator)
			if findErr != nil {
				if isTransientNotFound(findErr) {
					transientErr = findErr
					return false, nil
				}

				return false, findErr
			}

			if len(candidates) == 0 {
				// 空集合是一次成功的查找结果，而不是一个错误。它不会
				// 覆盖此前由根包报告的未找到错误，以便超时结果保留
				// 最后一次有诊断价值的错误。
				return false, nil
			}
			for _, candidate := range candidates {
				if candidate == nil {
					return false, invalidElementsResultError()
				}
			}

			found = append(
				found[:0],
				candidates...,
			)
			return true, nil
		},
	)

	if err != nil {
		return nil, preserveTransientError(err, transientErr)
	}

	return found, nil
}

// isTransientNotFound 将根包明确的未找到错误标记为可轮询结果。
//
// wait 不根据 Delivery、错误文本或远端实现自行推断其他可重试类别。
func isTransientNotFound(err error) bool {
	return appium.IsErrorCode(err, appium.CodeElementNotFound)
}

// isNilFinder 处理 nil interface 和携带 nil 指针的结构化 finder，避免在
// 调用其方法时触发 panic。
func isNilFinder(finder interface{}) bool {
	if finder == nil {
		return true
	}

	value := reflect.ValueOf(finder)
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		return value.IsNil()
	default:
		return false
	}
}

// preserveTransientError 保留 context 结果，同时不吞掉最后一次未找到错误。
func preserveTransientError(waitErr error, transientErr error) error {
	if waitErr == nil || transientErr == nil {
		return waitErr
	}

	if !isContextTermination(waitErr) {
		return waitErr
	}

	return errors.Join(waitErr, transientErr)
}

// isContextTermination 识别 Until 返回的原始 context 错误，以及根包在
// Find 请求内部映射出的结构化 context 错误。
func isContextTermination(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		appium.IsErrorCode(err, appium.CodeCanceled) ||
		appium.IsErrorCode(err, appium.CodeDeadlineExceeded)
}

// invalidFinderError 创建未提供查找源时的本地参数错误。
func invalidFinderError(operation string) error {
	return &appium.Error{
		Code:      appium.CodeInvalidArgument,
		Operation: operation,
		Message:   "finder is nil",
		Delivery:  appium.DeliveryNotSent,
	}
}

// invalidElementResultError 报告本地 finder 返回的 nil 成功结果。
func invalidElementResultError() error {
	return errors.New(
		"wait.Element finder returned a nil element without an error",
	)
}

// invalidElementsResultError 报告本地 finder 返回的 nil 集合元素。
func invalidElementsResultError() error {
	return errors.New(
		"wait.Elements finder returned a nil element",
	)
}
