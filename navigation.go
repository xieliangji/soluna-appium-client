package appium

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const backgroundAppOperation = "background_app"

// BackgroundApp 请求将当前 App 放入后台且不自动恢复。
//
// 每次调用只发送一次 Appium background 命令，并固定使用负数 seconds。
// nil error 仅表示 Driver 返回了已接受的成功响应；客户端不会读取、缓存或
// 确认后续 App 状态。需要恢复时应显式调用 ActivateApp。
func (s *Session) BackgroundApp(ctx context.Context) error {
	client, err := s.commandClient(backgroundAppOperation)
	if err != nil {
		return err
	}

	command, err := wire.NewCommand(
		backgroundAppOperation,
		http.MethodPost,
		"session",
		s.id,
		"appium",
		"app",
		"background",
	)
	if err != nil {
		return commandDefinitionError(
			backgroundAppOperation,
			"background app command definition is invalid",
			err,
		)
	}

	request := struct {
		Seconds int64 `json:"seconds"`
	}{
		Seconds: -1,
	}

	return client.executeCommand(
		ctx,
		command,
		request,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		decodeBackgroundAppResponse,
	)
}

// decodeBackgroundAppResponse 校验两个目标 Driver 的 background 成功值。
//
// XCUITest 的无恢复路径返回 null；UiAutomator2 当前返回 true。该差异不形成
// 公共结果，但其他成功值不属于本命令的已接受协议形态。
func decodeBackgroundAppResponse(
	ctx context.Context,
	value json.RawMessage,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	value = bytes.TrimSpace(value)
	if bytes.Equal(value, []byte("null")) ||
		bytes.Equal(value, []byte("true")) {
		return nil
	}

	return errors.New(
		"background app response must be null or true",
	)
}
