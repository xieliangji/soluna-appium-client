package wait

import (
	"context"
	"errors"
	"time"
)

// Until 在调用方 context 允许的总期限内轮询 condition。
//
// condition 会先立即执行一次。interval 必须为正数，并表示两次未完成
// 检查之间至少等待的时间。context 结束时返回对应的 context 错误；条件
// 返回的错误不会被包装或重试。条件函数负责在自身执行中响应传入的 context，
// Until 不启动后台 goroutine 强制中断条件。
//
// ctx 或 condition 为 nil，以及 interval 非正时，函数在开始轮询前返回
// 参数错误。Until 不修改任何 Session 超时或其他远端状态。
func Until(
	ctx context.Context,
	interval time.Duration,
	condition func(context.Context) (done bool, err error),
) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	if interval <= 0 {
		return errors.New("wait interval must be positive")
	}
	if condition == nil {
		return errors.New("condition is nil")
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		done, err := condition(ctx)
		if err != nil {
			return err
		}
		if done {
			// 条件函数可能没有使用 ctx；成功也必须不能越过调用方
			// 已经结束的总期限。
			if err := ctx.Err(); err != nil {
				return err
			}
			return nil
		}

		if err := waitInterval(ctx, interval); err != nil {
			return err
		}
	}
}

// waitInterval 在可取消的 context 下等待下一轮检查。
func waitInterval(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
