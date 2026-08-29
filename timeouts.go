package soluna_appium_client

import (
	"context"
	"net/http"
	"time"

	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	setScriptTimeoutOperation   = "set_script_timeout"
	setPageLoadTimeoutOperation = "set_page_load_timeout"
	setImplicitWaitOperation    = "set_implicit_wait"
)

// Timeouts 表示 WebDriver Session 当前使用的超时配置。
//
// 所有超时均使用 Go 的 time.Duration 表示。
// 具体的协议传输层负责在 time.Duration 与 WebDriver 使用的毫秒值之间转换。
type Timeouts struct {
	// Script 表示脚本执行超时时间。
	Script time.Duration

	// PageLoad 表示页面加载超时时间。
	PageLoad time.Duration

	// Implicit 表示元素查找使用的隐式等待超时时间。
	Implicit time.Duration
}

// SetScriptTimeout 设置当前 Session 的脚本执行超时时间。
//
// timeout 必须是非负且能够精确表示为整数毫秒的 time.Duration。
func (s *Session) SetScriptTimeout(
	ctx context.Context,
	timeout time.Duration,
) error {
	millis, err := timeoutMilliseconds(
		setScriptTimeoutOperation,
		timeout,
	)
	if err != nil {
		return err
	}

	request := struct {
		Script int64 `json:"script"`
	}{
		Script: millis,
	}

	return s.setTimeout(
		ctx,
		setScriptTimeoutOperation,
		request,
	)
}

// SetPageLoadTimeout 设置当前 Session 的页面加载超时时间。
//
// timeout 必须是非负且能够精确表示为整数毫秒的 time.Duration。
func (s *Session) SetPageLoadTimeout(
	ctx context.Context,
	timeout time.Duration,
) error {
	millis, err := timeoutMilliseconds(
		setPageLoadTimeoutOperation,
		timeout,
	)
	if err != nil {
		return err
	}

	request := struct {
		PageLoad int64 `json:"pageLoad"`
	}{
		PageLoad: millis,
	}

	return s.setTimeout(
		ctx,
		setPageLoadTimeoutOperation,
		request,
	)
}

// SetImplicitWait 设置当前 Session 的元素隐式等待超时时间。
//
// timeout 必须是非负且能够精确表示为整数毫秒的 time.Duration。
func (s *Session) SetImplicitWait(
	ctx context.Context,
	timeout time.Duration,
) error {
	millis, err := timeoutMilliseconds(
		setImplicitWaitOperation,
		timeout,
	)
	if err != nil {
		return err
	}

	request := struct {
		Implicit int64 `json:"implicit"`
	}{
		Implicit: millis,
	}

	return s.setTimeout(
		ctx,
		setImplicitWaitOperation,
		request,
	)
}

// setTimeout 执行一次确定的 Session Timeout 更新。
//
// 客户端不会缓存设置后的 Timeout 状态。
// 命令投递结果不确定时，上层不会持有可能已经失真的本地副本。
func (s *Session) setTimeout(
	ctx context.Context,
	operation string,
	request any,
) error {
	client, err := s.timeoutCommandClient(operation)
	if err != nil {
		return err
	}

	command, err := wire.NewCommand(
		operation,
		http.MethodPost,
		"session",
		s.id,
		"timeouts",
	)
	if err != nil {
		return &Error{
			Code:      CodeInvalidConfig,
			Operation: operation,
			Message:   "set timeout command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
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

// timeoutCommandClient 校验 Session 是否允许执行 Timeout 命令。
func (s *Session) timeoutCommandClient(
	operation string,
) (*Client, error) {
	if s == nil ||
		s.client == nil ||
		s.state == nil ||
		s.id == "" {
		return nil, &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   "session is not initialized",
			Delivery:  DeliveryNotSent,
		}
	}

	if !s.usable {
		return nil, &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   "session is not usable for commands",
			Delivery:  DeliveryNotSent,
		}
	}

	if s.state.closed.Load() {
		return nil, &Error{
			Code:      CodeSessionLost,
			Operation: operation,
			Message:   "session is closed",
			Delivery:  DeliveryNotSent,
		}
	}

	return s.client, nil
}

// timeoutMilliseconds 将 Go Duration 转换为 WebDriver 使用的整数毫秒。
func timeoutMilliseconds(
	operation string,
	timeout time.Duration,
) (int64, error) {
	if timeout < 0 {
		return 0, &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   "timeout must not be negative",
			Delivery:  DeliveryNotSent,
		}
	}

	if timeout%time.Millisecond != 0 {
		return 0, &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   "timeout must be an exact number of milliseconds",
			Delivery:  DeliveryNotSent,
		}
	}

	return timeout.Milliseconds(), nil
}
