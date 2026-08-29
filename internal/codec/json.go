package codec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const jsonContextCheckInterval = 32 << 10

var (
	ErrInvalidJSON          = errors.New("invalid JSON data")
	ErrInvalidJSONSurrogate = errors.New("invalid JSON unicode surrogate")
)

// DecodeJSONString 严格解码一个 JSON 字符串值。
//
// 除了标准 JSON 语法校验外，该方法还会拒绝未配对的 Unicode surrogate。
// Go 的 encoding/json 默认会将未配对 surrogate 替换为 Unicode replacement character，
// 这里不接受这种隐式修正。
func DecodeJSONString(ctx context.Context, raw []byte) (string, error) {
	if ctx == nil {
		return "", errors.New("context is nil")
	}

	if err := ValidateUTF8(ctx, raw); err != nil {
		return "", err
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	if err := validateJSONSurrogates(ctx, bytes.TrimSpace(raw)); err != nil {
		return "", err
	}

	return value, nil
}

// validateJSONSurrogates 校验 JSON 字符串中的 Unicode surrogate 是否成对出现。
func validateJSONSurrogates(ctx context.Context, raw []byte) error {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return nil
	}

	nextCheck := 0

	for offset := 1; offset < len(raw)-1; {
		if offset >= nextCheck {
			if err := ctx.Err(); err != nil {
				return err
			}
			nextCheck = offset + jsonContextCheckInterval
		}

		if raw[offset] != '\\' {
			offset++
			continue
		}

		if offset+1 >= len(raw)-1 {
			return ErrInvalidJSON
		}

		if raw[offset+1] != 'u' {
			offset += 2
			continue
		}

		value, ok := decodeJSONHex4(raw, offset+2)
		if !ok {
			return ErrInvalidJSON
		}

		offset += 6

		switch {
		case value >= 0xd800 && value <= 0xdbff:
			if offset+6 > len(raw)-1 ||
				raw[offset] != '\\' ||
				raw[offset+1] != 'u' {
				return ErrInvalidJSONSurrogate
			}

			low, ok := decodeJSONHex4(raw, offset+2)
			if !ok || low < 0xdc00 || low > 0xdfff {
				return ErrInvalidJSONSurrogate
			}

			offset += 6

		case value >= 0xdc00 && value <= 0xdfff:
			return ErrInvalidJSONSurrogate
		}
	}

	return ctx.Err()
}

// decodeJSONHex4 解码 JSON \u 转义中的四位十六进制数。
func decodeJSONHex4(raw []byte, offset int) (uint16, bool) {
	if offset+4 > len(raw) {
		return 0, false
	}

	var value uint16

	for _, character := range raw[offset : offset+4] {
		value <<= 4

		switch {
		case character >= '0' && character <= '9':
			value |= uint16(character - '0')

		case character >= 'a' && character <= 'f':
			value |= uint16(character-'a') + 10

		case character >= 'A' && character <= 'F':
			value |= uint16(character-'A') + 10

		default:
			return 0, false
		}
	}

	return value, true
}
