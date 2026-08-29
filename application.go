package soluna_appium_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	activateAppOperation  = "activate_app"
	terminateAppOperation = "terminate_app"
	getAppStateOperation  = "get_app_state"
)

// AppState 表示应用在设备上的当前运行状态。
//
// 数值与 Appium queryAppState 返回的协议值保持一致。
type AppState int

const (
	AppStateNotInstalled        AppState = 0 // 应用未安装
	AppStateNotRunning          AppState = 1 // 应用已安装但未运行
	AppStateBackgroundSuspended AppState = 2 // 应用在后台运行但处于挂起状态
	AppStateBackground          AppState = 3 // 应用正在后台运行
	AppStateForeground          AppState = 4 // 应用正在前台运行
)

// ActivateApp 激活指定应用。
//
// appID 表示 Android package name 或 iOS bundle ID。
func (s *Session) ActivateApp(
	ctx context.Context,
	appID string,
) error {
	client, err := s.commandClient(
		activateAppOperation,
	)
	if err != nil {
		return err
	}

	if appID == "" {
		return &Error{
			Code:      CodeInvalidArgument,
			Operation: activateAppOperation,
			Message:   "app ID is empty",
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		activateAppOperation,
		http.MethodPost,
		"session",
		s.id,
		"appium",
		"device",
		"activate_app",
	)
	if err != nil {
		return &Error{
			Code:      CodeInvalidConfig,
			Operation: activateAppOperation,
			Message:   "activate app command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	request := struct {
		AppID string `json:"appId"`
	}{
		AppID: appID,
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

// TerminateApp 终止指定应用。
//
// appID 表示 Android package name 或 iOS bundle ID。
func (s *Session) TerminateApp(
	ctx context.Context,
	appID string,
) error {
	client, err := s.commandClient(
		terminateAppOperation,
	)
	if err != nil {
		return err
	}

	if appID == "" {
		return &Error{
			Code:      CodeInvalidArgument,
			Operation: terminateAppOperation,
			Message:   "app ID is empty",
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		terminateAppOperation,
		http.MethodPost,
		"session",
		s.id,
		"appium",
		"device",
		"terminate_app",
	)
	if err != nil {
		return &Error{
			Code:      CodeInvalidConfig,
			Operation: terminateAppOperation,
			Message:   "terminate app command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	request := struct {
		AppID string `json:"appId"`
	}{
		AppID: appID,
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

// AppState 获取指定应用当前的运行状态。
//
// appID 表示 Android package name 或 iOS bundle ID。
// 如果远端返回 Appium 协议未定义的状态值，则返回响应格式错误。
func (s *Session) AppState(
	ctx context.Context,
	appID string,
) (AppState, error) {
	client, err := s.commandClient(
		getAppStateOperation,
	)
	if err != nil {
		return 0, err
	}

	if appID == "" {
		return 0, &Error{
			Code:      CodeInvalidArgument,
			Operation: getAppStateOperation,
			Message:   "app ID is empty",
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		getAppStateOperation,
		http.MethodPost,
		"session",
		s.id,
		"appium",
		"device",
		"app_state",
	)
	if err != nil {
		return 0, &Error{
			Code:      CodeInvalidConfig,
			Operation: getAppStateOperation,
			Message:   "get app state command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	request := struct {
		AppID string `json:"appId"`
	}{
		AppID: appID,
	}

	var state AppState

	err = client.executeCommand(
		ctx,
		command,
		request,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			decoded, decodeErr := decodeAppState(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}

			state = decoded
			return nil
		},
	)
	if err != nil {
		return 0, err
	}

	return state, nil
}

// decodeAppState 严格解码 Appium queryAppState 返回值。
func decodeAppState(
	ctx context.Context,
	value json.RawMessage,
) (AppState, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	var state *int

	if err := json.Unmarshal(value, &state); err != nil {
		return 0, fmt.Errorf(
			"decode app state: %w",
			err,
		)
	}

	if state == nil {
		return 0, errors.New(
			"app state response must be an integer",
		)
	}

	switch AppState(*state) {
	case AppStateNotInstalled,
		AppStateNotRunning,
		AppStateBackgroundSuspended,
		AppStateBackground,
		AppStateForeground:
		return AppState(*state), nil

	default:
		return 0, fmt.Errorf(
			"app state response contains invalid value: %d",
			*state,
		)
	}
}
