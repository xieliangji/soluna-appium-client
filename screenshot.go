package appium

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	screenshotOperation        = "screenshot"
	elementScreenshotOperation = "element_screenshot"
)

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

	return executeScreenshotTo(
		ctx,
		client,
		dst,
		screenshotOperation,
		"screenshot destination writer is nil",
		"session",
		s.id,
		"screenshot",
	)
}

// Screenshot 获取当前 Element 的远端截图。
//
// 截图由 Driver 按 W3C Element Screenshot 语义生成；客户端不会自动滚动、
// 恢复可见性、处理 stale 引用，也不会将完整截图按 Element Rect 在本地裁剪。
// 返回值为解码后的截图字节数据，并使用与 Session.Screenshot 相同的资源上限
// 和错误语义。
func (e *Element) Screenshot(
	ctx context.Context,
) ([]byte, error) {
	var buffer bytes.Buffer

	_, err := e.ScreenshotTo(
		ctx,
		&buffer,
	)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// ScreenshotTo 获取当前 Element 的远端截图，并将解码后的 PNG 数据直接写入 dst。
//
// 返回值表示已经成功写入 dst 的字节数。该方法使用
// MaxScreenshotResponseBytes 限制远端响应和解码后的截图数据；如果 Base64
// 解码、context 或 dst 写入过程中发生错误，dst 中可能已经包含部分数据。
// Driver 对元素可见性和 stale 引用的处理结果会原样通过统一错误模型返回。
func (e *Element) ScreenshotTo(
	ctx context.Context,
	dst io.Writer,
) (int64, error) {
	session, client, err := e.commandContext(
		elementScreenshotOperation,
	)
	if err != nil {
		return 0, err
	}

	return executeScreenshotTo(
		ctx,
		client,
		dst,
		elementScreenshotOperation,
		"element screenshot destination writer is nil",
		"session",
		session.id,
		"element",
		e.id,
		"screenshot",
	)
}

// executeScreenshotTo 执行一条截图命令并将其 Base64 value 流式解码到 dst。
//
// Session 和 Element 截图共用该路径，确保响应上限、解码校验、部分写入和
// 输出错误的可观察语义一致。
func executeScreenshotTo(
	ctx context.Context,
	client *Client,
	dst io.Writer,
	operation string,
	nilWriterMessage string,
	segments ...string,
) (int64, error) {
	if dst == nil {
		return 0, &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   nilWriterMessage,
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		operation,
		http.MethodGet,
		segments...,
	)
	if err != nil {
		return 0, commandDefinitionError(
			operation,
			"screenshot command definition is invalid",
			err,
		)
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
			count, decodeErr := decodeScreenshotTo(
				ctx,
				dst,
				value,
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

// decodeScreenshotTo 校验截图 value 并将其 Base64 内容写入 dst。
func decodeScreenshotTo(
	ctx context.Context,
	dst io.Writer,
	value json.RawMessage,
	maxBytes int64,
) (int64, error) {
	encoded, err := codec.DecodeJSONString(
		ctx,
		value,
	)
	if err != nil {
		return 0, err
	}

	return codec.DecodeBase64To(
		ctx,
		dst,
		encoded,
		maxBytes,
	)
}
