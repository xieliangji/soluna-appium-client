package wait

import (
	"context"
	"errors"
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
// Session 级查找和元素后代查找。查找会先立即执行；只有根包明确报告
// CodeElementNotFound 的结果会被当作暂态并按 interval 重试。其他错误
// （包括 stale、Session 丢失、参数、响应和传输错误）会立即原样返回。
// Find 返回 nil 元素而没有错误也属于响应格式错误。
//
// ctx 必须非 nil，interval 必须为正数；其他参数校验和 context 的语义与
// Until 相同。等待到 context 结束时，若已有暂态未找到错误，会通过
// errors.Join 同时保留 context 结果和最后一次查找
// 错误；调用方可以分别使用 errors.Is 或 appium.IsErrorCode 检查两者。
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
	var terminalErr error

	err := Until(
		ctx,
		interval,
		func(ctx context.Context) (bool, error) {
			if finder == nil {
				terminalErr = invalidFinderError(
					waitElementOperation,
				)
				return false, terminalErr
			}

			candidate, findErr := finder.Find(ctx, locator)
			if findErr != nil {
				if isTransientNotFound(findErr) {
					transientErr = findErr
					return false, nil
				}

				terminalErr = findErr
				return false, findErr
			}

			if candidate == nil {
				terminalErr = invalidElementResultError()
				return false, terminalErr
			}

			found = candidate
			return true, nil
		},
	)

	if err != nil {
		// An error returned by the condition is the final command/result error and
		// must not be joined with an older transient result.
		if terminalErr != nil {
			return nil, terminalErr
		}

		return nil, preserveTransientError(err, transientErr)
	}

	return found, nil
}

// Elements 在调用方 context 允许的期限内等待多元素查找返回至少一个元素。
//
// finder 可以是根包的 *appium.Session 或 *appium.Element，分别表示
// Session 级查找和元素后代查找。查找会先立即执行；空集合以及根包明确
// 报告 CodeElementNotFound 的结果会按 interval 重试。其他错误（包括
// stale、Session 丢失、参数、响应和传输错误）会立即原样返回。
// FindElements 返回包含 nil 元素的集合也属于响应格式错误。
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
	var terminalErr error

	err := Until(
		ctx,
		interval,
		func(ctx context.Context) (bool, error) {
			if finder == nil {
				terminalErr = invalidFinderError(
					waitElementsOperation,
				)
				return false, terminalErr
			}

			candidates, findErr := finder.FindElements(ctx, locator)
			if findErr != nil {
				if isTransientNotFound(findErr) {
					transientErr = findErr
					return false, nil
				}

				terminalErr = findErr
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
					terminalErr = invalidElementsResultError()
					return false, terminalErr
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
		if terminalErr != nil {
			return nil, terminalErr
		}

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

// preserveTransientError 保留 context 结果，同时不吞掉最后一次未找到错误。
func preserveTransientError(waitErr error, transientErr error) error {
	if waitErr == nil || transientErr == nil {
		return waitErr
	}

	if !errors.Is(waitErr, context.Canceled) &&
		!errors.Is(waitErr, context.DeadlineExceeded) {
		return waitErr
	}

	return errors.Join(waitErr, transientErr)
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

// invalidElementResultError 防止不符合公共 Find 契约的 nil 成功结果泄漏。
func invalidElementResultError() error {
	return &appium.Error{
		Code:      appium.CodeResponseInvalid,
		Operation: waitElementOperation,
		Message:   "find returned a nil element without an error",
		Delivery:  appium.DeliveryAcknowledged,
	}
}

// invalidElementsResultError 防止不符合公共 FindElements 契约的 nil 元素泄漏。
func invalidElementsResultError() error {
	return &appium.Error{
		Code:      appium.CodeResponseInvalid,
		Operation: waitElementsOperation,
		Message:   "find elements returned a nil element",
		Delivery:  appium.DeliveryAcknowledged,
	}
}
