package soluna_appium_client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	getTimeoutsOperation        = "get_timeouts"
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

// Timeouts 获取当前 Session 的 Script、PageLoad 和 Implicit 超时配置。
//
// 远端返回值必须包含三个整数毫秒字段；字段值可以为零，但不能为负数，
// 且必须能够安全转换为 time.Duration。方法每次调用都会读取远端结果，
// 不缓存之前的响应。
func (s *Session) Timeouts(ctx context.Context) (Timeouts, error) {
	client, err := s.commandClient(getTimeoutsOperation)
	if err != nil {
		return Timeouts{}, err
	}

	command, err := wire.NewCommand(
		getTimeoutsOperation,
		http.MethodGet,
		"session",
		s.id,
		"timeouts",
	)
	if err != nil {
		return Timeouts{}, commandDefinitionError(
			getTimeoutsOperation,
			"get timeouts command definition is invalid",
			err,
		)
	}

	var timeouts Timeouts
	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decodeTimeouts(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			timeouts = decoded
			return nil
		},
	)
	if err != nil {
		return Timeouts{}, err
	}

	return timeouts, nil
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
	client, err := s.commandClient(operation)
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

// decodeTimeouts 严格解码 W3C Get Timeouts 的响应值。
func decodeTimeouts(
	ctx context.Context,
	value json.RawMessage,
) (Timeouts, error) {
	if err := ctx.Err(); err != nil {
		return Timeouts{}, err
	}

	var payload struct {
		Script   json.RawMessage `json:"script"`
		PageLoad json.RawMessage `json:"pageLoad"`
		Implicit json.RawMessage `json:"implicit"`
	}
	if err := json.Unmarshal(value, &payload); err != nil {
		return Timeouts{}, fmt.Errorf("decode timeouts response: %w", err)
	}

	script, err := decodeTimeoutDuration(
		ctx,
		"script",
		payload.Script,
	)
	if err != nil {
		return Timeouts{}, err
	}

	pageLoad, err := decodeTimeoutDuration(
		ctx,
		"pageLoad",
		payload.PageLoad,
	)
	if err != nil {
		return Timeouts{}, err
	}

	implicit, err := decodeTimeoutDuration(
		ctx,
		"implicit",
		payload.Implicit,
	)
	if err != nil {
		return Timeouts{}, err
	}

	return Timeouts{
		Script:   script,
		PageLoad: pageLoad,
		Implicit: implicit,
	}, nil
}

// decodeTimeoutDuration 将一个整数毫秒 JSON 字段转换为 time.Duration。
func decodeTimeoutDuration(
	ctx context.Context,
	field string,
	value json.RawMessage,
) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if len(value) == 0 {
		return 0, fmt.Errorf("timeouts response does not contain %s", field)
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return 0, fmt.Errorf("timeouts response field %s is null", field)
	}

	var millis int64
	if err := json.Unmarshal(value, &millis); err != nil {
		return 0, fmt.Errorf("decode timeouts %s milliseconds: %w", field, err)
	}
	if millis < 0 {
		return 0, fmt.Errorf("timeouts response field %s is negative", field)
	}
	if millis > math.MaxInt64/int64(time.Millisecond) {
		return 0, errors.New("timeouts response value overflows time.Duration")
	}

	if err := ctx.Err(); err != nil {
		return 0, err
	}

	return time.Duration(millis) * time.Millisecond, nil
}
