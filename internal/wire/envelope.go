package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

// Envelope 表示 W3C WebDriver 响应的统一外层结构。
//
// Value 保留原始 JSON 数据，由具体命令或错误解析层继续解码。
// JSON null 会被保留为有效的 RawMessage。
type Envelope struct {
	Value json.RawMessage `json:"value"`
}

// DecodeEnvelope 解析并校验 W3C WebDriver 响应外壳。
//
// 响应必须是有效 UTF-8 编码的 JSON 对象并包含 value 字段。
// value 的具体类型和语义不在此处校验。
func DecodeEnvelope(data []byte) (Envelope, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return Envelope{}, errors.New("WebDriver response body is empty")
	}
	if !utf8.Valid(data) {
		return Envelope{}, errors.New("WebDriver response body is not valid UTF-8")
	}

	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Envelope{}, fmt.Errorf("decode WebDriver response envelope: %w", err)
	}

	if envelope.Value == nil {
		return Envelope{}, errors.New("WebDriver response envelope does not contain value")
	}

	return envelope, nil
}
