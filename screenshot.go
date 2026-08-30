package soluna_appium_client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const screenshotOperation = "screenshot"

// Screenshot 获取当前 Session 的屏幕截图。
//
// 返回值为解码后的 PNG 字节数据。该方法使用与 ScreenshotTo 相同的
// Base64 解码、资源上限和错误语义；客户端不会对截图尺寸或像素坐标
// 语义进行额外转换。
func (s *Session) Screenshot(
	ctx context.Context,
) ([]byte, error) {
	var buffer bytes.Buffer

	_, err := s.ScreenshotTo(
		ctx,
		&buffer,
	)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// ScreenshotTo 获取当前 Session 的屏幕截图，并将解码后的 PNG 数据直接写入 dst。
//
// 返回值表示已经成功写入 dst 的字节数。该方法使用
// MaxScreenshotResponseBytes 限制远端响应和解码后的截图数据。
// 如果 Base64 解码、context 或 dst 写入过程中发生错误，dst 中可能已经
// 包含部分截图数据。
func (s *Session) ScreenshotTo(
	ctx context.Context,
	dst io.Writer,
) (int64, error) {
	client, err := s.commandClient(
		screenshotOperation,
	)
	if err != nil {
		return 0, err
	}

	if dst == nil {
		return 0, &Error{
			Code:      CodeInvalidArgument,
			Operation: screenshotOperation,
			Message:   "screenshot destination writer is nil",
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		screenshotOperation,
		http.MethodGet,
		"session",
		s.id,
		"screenshot",
	)
	if err != nil {
		return 0, &Error{
			Code:      CodeInvalidConfig,
			Operation: screenshotOperation,
			Message:   "screenshot command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	var written int64

	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxScreenshotResponseBytes,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			encoded, decodeErr := codec.DecodeJSONString(
				ctx,
				value,
			)
			if decodeErr != nil {
				return decodeErr
			}

			count, decodeErr := codec.DecodeBase64To(
				ctx,
				dst,
				encoded,
				client.limits.MaxScreenshotResponseBytes,
			)
			written = count

			return decodeErr
		},
	)
	if err != nil {
		return written, err
	}

	return written, nil
}
