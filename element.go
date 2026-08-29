package soluna_appium_client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	findElementOperation         = "find_element"
	getElementRectOperation      = "get_element_rect"
	getElementTextOperation      = "get_element_text"
	getElementAttributeOperation = "get_element_attribute"
	clearElementOperation        = "clear_element"
	sendKeysOperation            = "send_keys"
)

// Element 表示绑定到某个 Session 的远端 WebDriver 元素引用。
//
// Element 只保存远端元素 ID，不缓存 Locator、文本、属性或坐标。
// 元素失效后客户端不会自动重新定位。
type Element struct {
	session *Session
	id      string
}

// Find 查找当前 Session 中第一个匹配 Locator 的元素。
//
// Locator Strategy 会按照调用方提供的协议值原样发送，
// 客户端不会执行别名转换或自动规范化。
func (s *Session) Find(
	ctx context.Context,
	locator Locator,
) (*Element, error) {
	client, err := s.findElementClient()
	if err != nil {
		return nil, err
	}

	if locator.Strategy == "" {
		return nil, &Error{
			Code:      CodeInvalidArgument,
			Operation: findElementOperation,
			Message:   "locator strategy is empty",
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		findElementOperation,
		http.MethodPost,
		"session",
		s.id,
		"element",
	)
	if err != nil {
		return nil, &Error{
			Code:      CodeInvalidConfig,
			Operation: findElementOperation,
			Message:   "find element command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	request := struct {
		Using string `json:"using"`
		Value string `json:"value"`
	}{
		Using: string(locator.Strategy),
		Value: locator.Value,
	}

	var elementID string

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
			id, decodeErr := decodeElementReference(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}

			elementID = id
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return &Element{
		session: s,
		id:      elementID,
	}, nil
}

// ID 返回远端 WebDriver Element ID。
func (e *Element) ID() string {
	if e == nil {
		return ""
	}
	return e.id
}

// Rect 获取元素当前的 WebDriver Rect。
func (e *Element) Rect(ctx context.Context) (Rect, error) {
	session, client, err := e.commandContext(
		getElementRectOperation,
	)
	if err != nil {
		return Rect{}, err
	}

	command, err := wire.NewCommand(
		getElementRectOperation,
		http.MethodGet,
		"session",
		session.id,
		"element",
		e.id,
		"rect",
	)
	if err != nil {
		return Rect{}, commandDefinitionError(
			getElementRectOperation,
			"get element rect command definition is invalid",
			err,
		)
	}

	var rect Rect

	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			decoded, decodeErr := decodeRect(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}

			rect = decoded
			return nil
		},
	)
	if err != nil {
		return Rect{}, err
	}

	return rect, nil
}

// Text 获取元素当前文本。
func (e *Element) Text(ctx context.Context) (string, error) {
	session, client, err := e.commandContext(
		getElementTextOperation,
	)
	if err != nil {
		return "", err
	}

	command, err := wire.NewCommand(
		getElementTextOperation,
		http.MethodGet,
		"session",
		session.id,
		"element",
		e.id,
		"text",
	)
	if err != nil {
		return "", commandDefinitionError(
			getElementTextOperation,
			"get element text command definition is invalid",
			err,
		)
	}

	var text string

	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			decoded, decodeErr := codec.DecodeJSONString(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}

			text = decoded
			return nil
		},
	)
	if err != nil {
		return "", err
	}

	return text, nil
}

// Attribute 获取元素指定属性。
//
// exists 为 false 表示远端明确返回 null，即该属性不存在。
func (e *Element) Attribute(
	ctx context.Context,
	name string,
) (value string, exists bool, err error) {
	session, client, err := e.commandContext(
		getElementAttributeOperation,
	)
	if err != nil {
		return "", false, err
	}

	if name == "" {
		return "", false, &Error{
			Code:      CodeInvalidArgument,
			Operation: getElementAttributeOperation,
			Message:   "attribute name is empty",
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		getElementAttributeOperation,
		http.MethodGet,
		"session",
		session.id,
		"element",
		e.id,
		"attribute",
		name,
	)
	if err != nil {
		return "", false, commandDefinitionError(
			getElementAttributeOperation,
			"get element attribute command definition is invalid",
			err,
		)
	}

	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(
			ctx context.Context,
			raw json.RawMessage,
		) error {
			decoded, found, decodeErr := decodeOptionalString(
				ctx,
				raw,
			)
			if decodeErr != nil {
				return decodeErr
			}

			value = decoded
			exists = found
			return nil
		},
	)
	if err != nil {
		return "", false, err
	}

	return value, exists, nil
}

// Clear 清除元素当前输入内容。
func (e *Element) Clear(ctx context.Context) error {
	session, client, err := e.commandContext(
		clearElementOperation,
	)
	if err != nil {
		return err
	}

	command, err := wire.NewCommand(
		clearElementOperation,
		http.MethodPost,
		"session",
		session.id,
		"element",
		e.id,
		"clear",
	)
	if err != nil {
		return commandDefinitionError(
			clearElementOperation,
			"clear element command definition is invalid",
			err,
		)
	}

	return client.executeCommand(
		ctx,
		command,
		struct{}{},
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		decodeNullResponse,
	)
}

// SendKeys 向元素发送文本输入。
//
// value 会原样作为 W3C Element Send Keys 的 text 参数发送。
func (e *Element) SendKeys(
	ctx context.Context,
	value string,
) error {
	session, client, err := e.commandContext(
		sendKeysOperation,
	)
	if err != nil {
		return err
	}

	command, err := wire.NewCommand(
		sendKeysOperation,
		http.MethodPost,
		"session",
		session.id,
		"element",
		e.id,
		"value",
	)
	if err != nil {
		return commandDefinitionError(
			sendKeysOperation,
			"send keys command definition is invalid",
			err,
		)
	}

	request := struct {
		Text string `json:"text"`
	}{
		Text: value,
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

// findElementClient 校验 Session 是否允许执行元素查找命令。
func (s *Session) findElementClient() (*Client, error) {
	if s == nil ||
		s.client == nil ||
		s.state == nil ||
		s.id == "" {
		return nil, &Error{
			Code:      CodeInvalidArgument,
			Operation: findElementOperation,
			Message:   "session is not initialized",
			Delivery:  DeliveryNotSent,
		}
	}

	if !s.usable {
		return nil, &Error{
			Code:      CodeInvalidArgument,
			Operation: findElementOperation,
			Message:   "session is not usable for commands",
			Delivery:  DeliveryNotSent,
		}
	}

	if s.state.closed.Load() {
		return nil, &Error{
			Code:      CodeSessionLost,
			Operation: findElementOperation,
			Message:   "session is closed",
			Delivery:  DeliveryNotSent,
		}
	}

	return s.client, nil
}

// commandContext 校验 Element 及其所属 Session 是否可用于远端命令。
func (e *Element) commandContext(
	operation string,
) (*Session, *Client, error) {
	if e == nil ||
		e.session == nil ||
		e.id == "" {
		return nil, nil, &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   "element is not initialized",
			Delivery:  DeliveryNotSent,
		}
	}

	session := e.session

	if session.client == nil ||
		session.state == nil ||
		session.id == "" {
		return nil, nil, &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   "element session is not initialized",
			Delivery:  DeliveryNotSent,
		}
	}

	if !session.usable {
		return nil, nil, &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   "element session is not usable for commands",
			Delivery:  DeliveryNotSent,
		}
	}

	if session.state.closed.Load() {
		return nil, nil, &Error{
			Code:      CodeSessionLost,
			Operation: operation,
			Message:   "element session is closed",
			Delivery:  DeliveryNotSent,
		}
	}

	return session, session.client, nil
}

// decodeElementReference 严格解码 W3C Element Reference。
//
// 客户端只接受 W3C element reference key，
// 不使用旧 JSON Wire Protocol 的 ELEMENT 别名。
func decodeElementReference(
	ctx context.Context,
	value json.RawMessage,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var payload struct {
		ElementID json.RawMessage `json:"element-6066-11e4-a52e-4f735466cecf"`
	}

	if err := json.Unmarshal(value, &payload); err != nil {
		return "", fmt.Errorf(
			"decode element reference: %w",
			err,
		)
	}

	if len(payload.ElementID) == 0 {
		return "", errors.New(
			"element response does not contain W3C element reference",
		)
	}

	elementID, err := codec.DecodeJSONString(
		ctx,
		payload.ElementID,
	)
	if err != nil {
		return "", fmt.Errorf(
			"decode element ID: %w",
			err,
		)
	}

	if elementID == "" {
		return "", errors.New(
			"element response contains empty element ID",
		)
	}

	return elementID, nil
}

// decodeRect 严格解码元素 Rect。
func decodeRect(
	ctx context.Context,
	value json.RawMessage,
) (Rect, error) {
	if err := ctx.Err(); err != nil {
		return Rect{}, err
	}

	var payload struct {
		X      *float64 `json:"x"`
		Y      *float64 `json:"y"`
		Width  *float64 `json:"width"`
		Height *float64 `json:"height"`
	}

	if err := json.Unmarshal(value, &payload); err != nil {
		return Rect{}, fmt.Errorf(
			"decode element rect: %w",
			err,
		)
	}

	if payload.X == nil ||
		payload.Y == nil ||
		payload.Width == nil ||
		payload.Height == nil {
		return Rect{}, errors.New(
			"element rect response is incomplete",
		)
	}

	if *payload.Width < 0 || *payload.Height < 0 {
		return Rect{}, errors.New(
			"element rect contains negative size",
		)
	}

	if err := ctx.Err(); err != nil {
		return Rect{}, err
	}

	return Rect{
		X:      *payload.X,
		Y:      *payload.Y,
		Width:  *payload.Width,
		Height: *payload.Height,
	}, nil
}

// decodeOptionalString 解码 string 或 null 类型的响应值。
func decodeOptionalString(
	ctx context.Context,
	value json.RawMessage,
) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}

	if bytes.Equal(
		bytes.TrimSpace(value),
		[]byte("null"),
	) {
		return "", false, nil
	}

	decoded, err := codec.DecodeJSONString(ctx, value)
	if err != nil {
		return "", false, err
	}

	return decoded, true, nil
}

// commandDefinitionError 创建内部命令定义错误。
func commandDefinitionError(
	operation string,
	message string,
	cause error,
) error {
	return &Error{
		Code:      CodeInvalidConfig,
		Operation: operation,
		Message:   message,
		Delivery:  DeliveryNotSent,
		Cause:     cause,
	}
}
