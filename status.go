package appium

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const getStatusOperation = "get_status"

// ServerStatus 表示 Appium Server 当前的运行状态。
type ServerStatus struct {
	// Ready 表示 Appium Server 当前是否能够接受新的 Session 创建请求。
	//
	// Ready 为 true 只表示 Server 当前处于可创建 Session 的状态，
	// 不保证后续 Session 创建一定成功。
	Ready bool

	// Message 表示 Appium Server 对当前状态的说明。
	Message string

	// Version 表示 Appium Server 的版本号。
	Version string
}

// Status 获取 Appium Server 当前状态。
//
// Status 使用 ClientOptions.ReadyProbeTimeout 作为客户端内部超时。
// 如果调用方 context 具有更早的截止时间，则以调用方 context 为准。
func (c *Client) Status(ctx context.Context) (ServerStatus, error) {
	command, err := wire.NewCommand(
		getStatusOperation,
		http.MethodGet,
		"status",
	)
	if err != nil {
		return ServerStatus{}, &Error{
			Code:      CodeInvalidConfig,
			Operation: getStatusOperation,
			Message:   "status command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	var status ServerStatus

	err = c.executeCommand(
		ctx,
		command,
		nil,
		c.readyProbeTimeout,
		c.limits.MaxResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, err := decodeServerStatus(ctx, value)
			if err != nil {
				return err
			}

			status = decoded
			return nil
		},
	)
	if err != nil {
		return ServerStatus{}, err
	}

	return status, nil
}

// decodeServerStatus 校验并解码 Appium Server Status 响应。
func decodeServerStatus(
	ctx context.Context,
	value json.RawMessage,
) (ServerStatus, error) {
	if err := ctx.Err(); err != nil {
		return ServerStatus{}, err
	}

	var payload struct {
		Ready   *bool   `json:"ready"`
		Message *string `json:"message"`
		Build   *struct {
			Version *string `json:"version"`
		} `json:"build"`
	}

	if err := json.Unmarshal(value, &payload); err != nil {
		return ServerStatus{}, fmt.Errorf(
			"decode Appium status response: %w",
			err,
		)
	}

	if payload.Ready == nil {
		return ServerStatus{}, errors.New(
			"appium status response does not contain ready",
		)
	}
	if payload.Message == nil {
		return ServerStatus{}, errors.New(
			"appium status response does not contain message",
		)
	}
	if payload.Build == nil {
		return ServerStatus{}, errors.New(
			"appium status response does not contain build",
		)
	}
	if payload.Build.Version == nil {
		return ServerStatus{}, errors.New(
			"appium status response does not contain build.version",
		)
	}

	if err := ctx.Err(); err != nil {
		return ServerStatus{}, err
	}

	return ServerStatus{
		Ready:   *payload.Ready,
		Message: *payload.Message,
		Version: *payload.Build.Version,
	}, nil
}
