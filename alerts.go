package soluna_appium_client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	getAlertTextOperation = "get_alert_text"
	acceptAlertOperation  = "accept_alert"
	dismissAlertOperation = "dismiss_alert"
	setAlertTextOperation = "set_alert_text"
)

// AlertText 获取当前 Alert 的文本。
//
// 远端响应值可以是 JSON 字符串或 null。返回值中的 hasText 为 true
// 表示远端返回了字符串（包括空字符串），false 表示远端返回了 null。
// 如果当前没有 Alert，远端通常会返回 no such alert 错误。
// 客户端不会自动等待 Alert 出现。
func (s *Session) AlertText(ctx context.Context) (text string, hasText bool, err error) {
	client, err := s.commandClient(getAlertTextOperation)
	if err != nil {
		return "", false, err
	}

	command, err := wire.NewCommand(
		getAlertTextOperation,
		http.MethodGet,
		"session",
		s.id,
		"alert",
		"text",
	)
	if err != nil {
		return "", false, commandDefinitionError(
			getAlertTextOperation,
			"get alert text command definition is invalid",
			err,
		)
	}

	var hasTextResult bool
	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, present, decodeErr := decodeAlertText(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			text = decoded
			hasTextResult = present
			return nil
		},
	)
	if err != nil {
		return "", false, err
	}

	return text, hasTextResult, nil
}

// AcceptAlert 接受当前 Alert。
//
// 该方法只执行一次标准 W3C Alert 命令，不会自动等待或重试。
func (s *Session) AcceptAlert(ctx context.Context) error {
	return s.executeAlertCommand(
		ctx,
		acceptAlertOperation,
		http.MethodPost,
		struct{}{},
	)
}

// DismissAlert 关闭当前 Alert。
//
// 该方法只执行一次标准 W3C Alert 命令，不会自动等待或重试。
func (s *Session) DismissAlert(ctx context.Context) error {
	return s.executeAlertCommand(
		ctx,
		dismissAlertOperation,
		http.MethodPost,
		struct{}{},
	)
}

// SetAlertText 设置当前 Alert 的文本。
//
// 只有带输入框的 Alert 才支持该操作；具体支持情况由远端 Driver 决定。
// text 会作为标准 W3C Alert Text 请求体原样发送。
func (s *Session) SetAlertText(ctx context.Context, text string) error {
	request := struct {
		Text string `json:"text"`
	}{
		Text: text,
	}

	return s.executeAlertCommand(
		ctx,
		setAlertTextOperation,
		http.MethodPost,
		request,
	)
}

// executeAlertCommand 执行一个返回 JSON null 的标准 Alert 命令。
func (s *Session) executeAlertCommand(
	ctx context.Context,
	operation string,
	method string,
	payload any,
) error {
	client, err := s.commandClient(operation)
	if err != nil {
		return err
	}

	command, err := wire.NewCommand(
		operation,
		method,
		"session",
		s.id,
		"alert",
		alertCommandPath(operation),
	)
	if err != nil {
		return commandDefinitionError(
			operation,
			"alert command definition is invalid",
			err,
		)
	}

	return client.executeCommand(
		ctx,
		command,
		payload,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		decodeNullResponse,
	)
}

// alertCommandPath 将 Alert 操作映射为标准 W3C 路径末段。
func alertCommandPath(operation string) string {
	switch operation {
	case acceptAlertOperation:
		return "accept"
	case dismissAlertOperation:
		return "dismiss"
	case setAlertTextOperation:
		return "text"
	default:
		return ""
	}
}

// decodeAlertText 严格解码 Alert 文本响应。
//
// W3C Alert Text 的成功值必须是 JSON 字符串或显式 null。
func decodeAlertText(
	ctx context.Context,
	value json.RawMessage,
) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}

	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return "", false, nil
	}

	text, err := codec.DecodeJSONString(ctx, value)
	if err != nil {
		return "", false, err
	}

	return text, true, nil
}
