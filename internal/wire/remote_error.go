package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// RemoteError 表示 Appium/WebDriver 返回的远端错误信息。
//
// RemoteError 只保留协议层事实，不负责映射客户端公共错误码。
// Value 保存完整的远端错误 value，供上层在限制大小并脱敏后用于诊断。
type RemoteError struct {
	Code    string
	Message string
	Value   json.RawMessage
}

// Error 返回远端错误的简要描述。
//
// 为避免错误文本意外携带远端返回的敏感信息，
// 这里只包含远端错误码，不直接包含 Message 或 Value。
func (e *RemoteError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Code == "" {
		return "remote WebDriver error"
	}
	return "remote WebDriver error: " + e.Code
}

// DecodeRemoteError 解析 W3C WebDriver 错误响应中的 value。
//
// value 必须是 JSON 对象，并明确包含 error 和 message 字段。
// 具体错误码的语义映射由上层客户端负责。
func DecodeRemoteError(value json.RawMessage) (*RemoteError, error) {
	if len(bytes.TrimSpace(value)) == 0 {
		return nil, errors.New("remote WebDriver error value is empty")
	}

	var payload struct {
		Error   *string `json:"error"`
		Message *string `json:"message"`
	}

	if err := json.Unmarshal(value, &payload); err != nil {
		return nil, fmt.Errorf("decode remote WebDriver error: %w", err)
	}

	if payload.Error == nil {
		return nil, errors.New("remote WebDriver error does not contain error")
	}
	if *payload.Error == "" {
		return nil, errors.New("remote WebDriver error code is empty")
	}
	if payload.Message == nil {
		return nil, errors.New("remote WebDriver error does not contain message")
	}

	return &RemoteError{
		Code:    *payload.Error,
		Message: *payload.Message,
		Value:   append(json.RawMessage(nil), value...),
	}, nil
}
