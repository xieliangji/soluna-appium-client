package codec

import (
	"context"
	"errors"
	"unicode/utf8"
)

const utf8ContextCheckInterval = 32 << 10

var ErrInvalidUTF8 = errors.New("invalid UTF-8 data")

// ValidateUTF8 校验字节数据是否为有效的 UTF-8 编码。
//
// 对较大的数据会周期性检查 context 状态，
// 以便 Page Source 等大响应的处理能够及时响应取消。
func ValidateUTF8(ctx context.Context, value []byte) error {
	if ctx == nil {
		return errors.New("context is nil")
	}

	nextCheck := 0

	for offset := 0; offset < len(value); {
		if offset >= nextCheck {
			if err := ctx.Err(); err != nil {
				return err
			}
			nextCheck = offset + utf8ContextCheckInterval
		}

		r, size := utf8.DecodeRune(value[offset:])
		if r == utf8.RuneError && size == 1 {
			return ErrInvalidUTF8
		}

		offset += size
	}

	return ctx.Err()
}

// ValidateUTF8String 校验字符串是否包含有效的 UTF-8 编码。
//
// 该方法直接遍历字符串，不会为了校验而额外转换为 []byte。
func ValidateUTF8String(ctx context.Context, value string) error {
	if ctx == nil {
		return errors.New("context is nil")
	}

	nextCheck := 0

	for offset := 0; offset < len(value); {
		if offset >= nextCheck {
			if err := ctx.Err(); err != nil {
				return err
			}
			nextCheck = offset + utf8ContextCheckInterval
		}

		r, size := utf8.DecodeRuneInString(value[offset:])
		if r == utf8.RuneError && size == 1 {
			return ErrInvalidUTF8
		}

		offset += size
	}

	return ctx.Err()
}
