package soluna_appium_client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	startRecordingOperation = "start_recording"
	stopRecordingOperation  = "stop_recording"
)

// RecordingOptions 定义屏幕录制使用的跨平台通用参数。
type RecordingOptions struct {
	// TimeLimit 表示本次录屏允许持续的最长时间。
	//
	// 为零时不显式指定录屏时长，由远端 Driver 使用其默认行为。
	// 负数属于无效参数。
	TimeLimit time.Duration
}

// StartRecording 开始当前 Session 的屏幕录制。
//
// TimeLimit 会按照 Appium Driver 协议转换为秒。
// 非零值必须能够精确表示为整数秒。
//
// 如果远端已有录屏正在运行，具体处理方式由当前 Driver 决定。
// 部分 Driver 可能在响应中返回上一段录屏数据；该数据不会由本方法保留。
func (s *Session) StartRecording(
	ctx context.Context,
	options RecordingOptions,
) error {
	client, err := s.recordingCommandClient(
		startRecordingOperation,
	)
	if err != nil {
		return err
	}

	requestOptions, err := encodeRecordingOptions(
		options,
	)
	if err != nil {
		return err
	}

	command, err := wire.NewCommand(
		startRecordingOperation,
		http.MethodPost,
		"session",
		s.id,
		"appium",
		"start_recording_screen",
	)
	if err != nil {
		return &Error{
			Code:      CodeInvalidConfig,
			Operation: startRecordingOperation,
			Message:   "start recording command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	request := struct {
		Options recordingRequestOptions `json:"options"`
	}{
		Options: requestOptions,
	}

	return client.executeCommand(
		ctx,
		command,
		request,
		client.commandTimeout,
		client.limits.MaxRecordingResponseBytes,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			// Driver 可能返回空字符串，也可能在重新开始录屏时
			// 返回上一段录屏的 Base64 数据。
			// StartRecording 不消费该媒体数据，但仍严格要求响应为字符串。
			_, err := codec.DecodeJSONString(ctx, value)
			return err
		},
	)
}

// StopRecording 停止当前屏幕录制并返回解码后的媒体数据。
//
// 如果远端没有可用录屏，返回空字节切片。
func (s *Session) StopRecording(
	ctx context.Context,
) ([]byte, error) {
	var buffer bytes.Buffer

	_, err := s.StopRecordingTo(
		ctx,
		&buffer,
	)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// StopRecordingTo 停止当前屏幕录制，并将解码后的媒体数据直接写入 dst。
//
// 该方法用于降低长录屏场景下解码后媒体数据的峰值内存占用。
// 返回值表示已经成功写入 dst 的字节数。
//
// 如果 Base64 解码、context 或 dst 写入过程中发生错误，
// dst 中可能已经包含部分媒体数据。
func (s *Session) StopRecordingTo(
	ctx context.Context,
	dst io.Writer,
) (int64, error) {
	client, err := s.recordingCommandClient(
		stopRecordingOperation,
	)
	if err != nil {
		return 0, err
	}

	if dst == nil {
		return 0, &Error{
			Code:      CodeInvalidArgument,
			Operation: stopRecordingOperation,
			Message:   "recording destination writer is nil",
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		stopRecordingOperation,
		http.MethodPost,
		"session",
		s.id,
		"appium",
		"stop_recording_screen",
	)
	if err != nil {
		return 0, &Error{
			Code:      CodeInvalidConfig,
			Operation: stopRecordingOperation,
			Message:   "stop recording command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	var written int64

	err = client.executeCommand(
		ctx,
		command,
		struct{}{},
		client.commandTimeout,
		client.limits.MaxRecordingResponseBytes,
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
				client.limits.MaxRecordingResponseBytes,
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

// recordingRequestOptions 表示两个移动 Driver 共同支持的录屏参数。
type recordingRequestOptions struct {
	TimeLimit *int64 `json:"timeLimit,omitempty"`
}

// encodeRecordingOptions 将公共录屏参数转换为 Appium Driver 参数。
func encodeRecordingOptions(
	options RecordingOptions,
) (recordingRequestOptions, error) {
	if options.TimeLimit < 0 {
		return recordingRequestOptions{}, &Error{
			Code:      CodeInvalidArgument,
			Operation: startRecordingOperation,
			Message:   "recording time limit must not be negative",
			Delivery:  DeliveryNotSent,
		}
	}

	if options.TimeLimit == 0 {
		return recordingRequestOptions{}, nil
	}

	if options.TimeLimit%time.Second != 0 {
		return recordingRequestOptions{}, &Error{
			Code:      CodeInvalidArgument,
			Operation: startRecordingOperation,
			Message:   "recording time limit must be an exact number of seconds",
			Delivery:  DeliveryNotSent,
		}
	}

	seconds := int64(options.TimeLimit / time.Second)

	return recordingRequestOptions{
		TimeLimit: &seconds,
	}, nil
}

// recordingCommandClient 校验 Session 是否允许执行录屏命令。
func (s *Session) recordingCommandClient(
	operation string,
) (*Client, error) {
	if s == nil ||
		s.client == nil ||
		s.state == nil ||
		s.id == "" {
		return nil, &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   "session is not initialized",
			Delivery:  DeliveryNotSent,
		}
	}

	if !s.usable {
		return nil, &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   "session is not usable for commands",
			Delivery:  DeliveryNotSent,
		}
	}

	if s.state.closed.Load() {
		return nil, &Error{
			Code:      CodeSessionLost,
			Operation: operation,
			Message:   "session is closed",
			Delivery:  DeliveryNotSent,
		}
	}

	return s.client, nil
}
