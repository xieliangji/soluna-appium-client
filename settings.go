package soluna_appium_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	getSettingsOperation    = "get_settings"
	updateSettingsOperation = "update_settings"
)

// Settings 表示 Appium Session 的开放 Settings 键值集合。
//
// Setting 名称和值由 Driver 或 Plugin 定义；客户端不会维护白名单、
// 自动规范化值或推断未知键的语义。
type Settings map[string]any

// Settings 获取当前 Session 的远端 Settings 快照。
//
// 每次调用都会读取远端结果，不缓存之前的响应。返回值是独立的深拷贝，
// 调用方修改其中的 map 或 slice 不会影响本次调用之外的任何客户端状态。
func (s *Session) Settings(ctx context.Context) (Settings, error) {
	client, err := s.commandClient(getSettingsOperation)
	if err != nil {
		return nil, err
	}

	command, err := wire.NewCommand(
		getSettingsOperation,
		http.MethodGet,
		"session",
		s.id,
		"appium",
		"settings",
	)
	if err != nil {
		return nil, commandDefinitionError(
			getSettingsOperation,
			"get settings command definition is invalid",
			err,
		)
	}

	var settings Settings
	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decodeSettings(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			settings = decoded
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return settings, nil
}

// UpdateSettings 增量更新当前 Session 的远端 Settings。
//
// settings 会作为 JSON object 原样发送；客户端不会过滤、规范化或缓存其中的
// 键值。空的非 nil Settings 会发送 `{}`。nil Settings 不是 JSON object，
// 因此在请求发送前返回 CodeInvalidArgument。
func (s *Session) UpdateSettings(
	ctx context.Context,
	settings Settings,
) error {
	client, err := s.commandClient(updateSettingsOperation)
	if err != nil {
		return err
	}

	if settings == nil {
		return &Error{
			Code:      CodeInvalidArgument,
			Operation: updateSettingsOperation,
			Message:   "settings must not be nil",
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		updateSettingsOperation,
		http.MethodPost,
		"session",
		s.id,
		"appium",
		"settings",
	)
	if err != nil {
		return commandDefinitionError(
			updateSettingsOperation,
			"update settings command definition is invalid",
			err,
		)
	}

	return client.executeCommand(
		ctx,
		command,
		cloneSettings(settings),
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		decodeNullResponse,
	)
}

// decodeSettings 严格解码 Appium Get Settings 的 JSON object value。
func decodeSettings(
	ctx context.Context,
	value json.RawMessage,
) (Settings, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := codec.ValidateUTF8(ctx, value); err != nil {
		return nil, err
	}

	var settings Settings
	if err := json.Unmarshal(value, &settings); err != nil {
		return nil, fmt.Errorf("decode settings response: %w", err)
	}
	if settings == nil {
		return nil, errors.New("settings response must be a JSON object")
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return cloneSettings(settings), nil
}

// cloneSettings 深拷贝 Settings 及其嵌套 JSON 容器。
func cloneSettings(source Settings) Settings {
	if source == nil {
		return nil
	}

	cloned := make(Settings, len(source))
	for key, value := range source {
		cloned[key] = cloneJSONValue(value)
	}

	return cloned
}
