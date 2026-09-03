package appium

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	keyboardShownOperation   = "keyboard_shown"
	dismissKeyboardOperation = "dismiss_keyboard"
)

// KeyboardShown 读取当前 Driver 报告的软键盘显示状态快照。
//
// 每次调用只发送一次 Appium common is_keyboard_shown 命令，不缓存结果、
// 自动等待或确认后续状态。返回的 false 仅表示本次远端探测结果。
func (s *Session) KeyboardShown(ctx context.Context) (bool, error) {
	client, err := s.commandClient(keyboardShownOperation)
	if err != nil {
		return false, err
	}

	command, err := wire.NewCommand(
		keyboardShownOperation,
		http.MethodGet,
		"session",
		s.id,
		"appium",
		"device",
		"is_keyboard_shown",
	)
	if err != nil {
		return false, commandDefinitionError(
			keyboardShownOperation,
			"keyboard shown command definition is invalid",
			err,
		)
	}

	var shown bool
	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decodeKeyboardBool(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			shown = decoded
			return nil
		},
	)
	if err != nil {
		return false, err
	}

	return shown, nil
}

// DismissKeyboard 请求当前 Driver 尝试关闭软键盘。
//
// 请求体固定为空 JSON object。返回的布尔值是 Driver 对本次请求的原始
// 报告，不表示关闭后的最终状态；需要确认状态时应随后显式调用
// KeyboardShown。发生错误时返回值为 false，且不能据此推断远端状态。
func (s *Session) DismissKeyboard(ctx context.Context) (bool, error) {
	client, err := s.commandClient(dismissKeyboardOperation)
	if err != nil {
		return false, err
	}

	command, err := wire.NewCommand(
		dismissKeyboardOperation,
		http.MethodPost,
		"session",
		s.id,
		"appium",
		"device",
		"hide_keyboard",
	)
	if err != nil {
		return false, commandDefinitionError(
			dismissKeyboardOperation,
			"dismiss keyboard command definition is invalid",
			err,
		)
	}

	var dismissed bool
	err = client.executeCommand(
		ctx,
		command,
		struct{}{},
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decodeKeyboardBool(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			dismissed = decoded
			return nil
		},
	)
	if err != nil {
		return false, err
	}

	return dismissed, nil
}

// decodeKeyboardBool 严格解码键盘命令成功响应中的 JSON boolean。
//
// 使用 *bool 区分显式 null 与合法 false；数字、字符串、对象和数组同样
// 不属于该命令的成功值。
func decodeKeyboardBool(
	ctx context.Context,
	value json.RawMessage,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	var decoded *bool
	if err := json.Unmarshal(value, &decoded); err != nil {
		return false, fmt.Errorf(
			"decode keyboard response value: %w",
			err,
		)
	}
	if decoded == nil {
		return false, errors.New(
			"keyboard response value must be a JSON boolean",
		)
	}

	if err := ctx.Err(); err != nil {
		return false, err
	}

	return *decoded, nil
}
