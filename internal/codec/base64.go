package codec

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrInvalidBase64  = errors.New("invalid base64 data")
	ErrBase64TooLarge = errors.New("decoded base64 data exceeds limit")
	ErrOutputWrite    = errors.New("base64 output write failed")
)

// DecodeBase64 将标准 Base64 字符串解码为字节数据。
//
// maxBytes 表示允许的最大解码后大小。
// maxBytes 为负数属于无效参数。
func DecodeBase64(ctx context.Context, encoded string, maxBytes int64) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}

	decodedLength, err := decodedBase64Length(encoded)
	if err != nil {
		return nil, err
	}
	if maxBytes < 0 {
		return nil, errors.New("base64 decoded size limit is negative")
	}
	if decodedLength > maxBytes {
		return nil, fmt.Errorf(
			"%w: decoded=%d limit=%d",
			ErrBase64TooLarge,
			decodedLength,
			maxBytes,
		)
	}

	var buffer bytes.Buffer
	buffer.Grow(int(decodedLength))

	if _, err := decodeBase64To(ctx, &buffer, encoded, decodedLength); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// DecodeBase64To 将标准 Base64 字符串直接解码并写入 dst。
//
// 该方法适用于录屏等较大的响应，避免额外创建完整的解码结果副本。
// maxBytes 表示允许的最大解码后大小。
// maxBytes 为负数属于无效参数。
//
// 如果解码过程或 dst 写入失败，dst 中可能已经包含部分数据。
func DecodeBase64To(
	ctx context.Context,
	dst io.Writer,
	encoded string,
	maxBytes int64,
) (int64, error) {
	if ctx == nil {
		return 0, errors.New("context is nil")
	}
	if dst == nil {
		return 0, errors.New("base64 destination writer is nil")
	}

	decodedLength, err := decodedBase64Length(encoded)
	if err != nil {
		return 0, err
	}
	if maxBytes < 0 {
		return 0, errors.New("base64 decoded size limit is negative")
	}
	if decodedLength > maxBytes {
		return 0, fmt.Errorf(
			"%w: decoded=%d limit=%d",
			ErrBase64TooLarge,
			decodedLength,
			maxBytes,
		)
	}

	return decodeBase64To(ctx, dst, encoded, decodedLength)
}

// decodeBase64To 执行实际的 Base64 流式解码。
func decodeBase64To(
	ctx context.Context,
	dst io.Writer,
	encoded string,
	expectedLength int64,
) (int64, error) {
	decoder := base64.NewDecoder(
		base64.StdEncoding.Strict(),
		strings.NewReader(encoded),
	)
	destination := base64TrackingWriter{
		writer: dst,
	}

	written, err := io.Copy(&destination, contextReader{
		ctx:    ctx,
		reader: decoder,
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return written, ctxErr
		}
		if destination.err != nil {
			return written, &OutputError{
				Cause: destination.err,
			}
		}
		if errors.Is(err, io.ErrShortWrite) {
			return written, &OutputError{
				Cause: err,
			}
		}
		return written, fmt.Errorf("%w: %w", ErrInvalidBase64, err)
	}
	if err := ctx.Err(); err != nil {
		return written, err
	}

	if written != expectedLength {
		return written, fmt.Errorf(
			"%w: decoded length mismatch: expected=%d actual=%d",
			ErrInvalidBase64,
			expectedLength,
			written,
		)
	}

	return written, nil
}

// OutputError 表示 Base64 解码结果写入目标时发生的错误。
//
// Cause 保留底层 Writer 错误，调用方可以通过 errors.Is/As 继续检查。
type OutputError struct {
	Cause error
}

// Error 返回输出交付错误文本。
func (e *OutputError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause == nil {
		return ErrOutputWrite.Error()
	}
	return ErrOutputWrite.Error() + ": " + e.Cause.Error()
}

// Unwrap 返回底层 Writer 错误。
func (e *OutputError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is 让 OutputError 能够匹配 ErrOutputWrite。
func (e *OutputError) Is(target error) bool {
	return target == ErrOutputWrite
}

// base64TrackingWriter 保留目标 Writer 的原始错误，以免和 Base64 解码错误混淆。
type base64TrackingWriter struct {
	writer io.Writer
	err    error
}

// Write 实现 io.Writer。
func (w *base64TrackingWriter) Write(p []byte) (int, error) {
	written, err := w.writer.Write(p)
	if err != nil {
		w.err = err
	}
	return written, err
}

// decodedBase64Length 返回标准 Base64 数据解码后的准确字节数。
//
// 本客户端只接受紧凑的标准 Base64 表达形式，不接受换行或缺失 Padding 的变体。
func decodedBase64Length(encoded string) (int64, error) {
	if encoded == "" {
		return 0, nil
	}
	if strings.ContainsAny(encoded, "\r\n") {
		return 0, fmt.Errorf("%w: line breaks are not allowed", ErrInvalidBase64)
	}
	if len(encoded)%4 != 0 {
		return 0, fmt.Errorf("%w: encoded length is not a multiple of four", ErrInvalidBase64)
	}

	padding := 0
	switch {
	case strings.HasSuffix(encoded, "=="):
		padding = 2
	case strings.HasSuffix(encoded, "="):
		padding = 1
	}

	dataEnd := len(encoded) - padding
	if strings.Contains(encoded[:dataEnd], "=") {
		return 0, fmt.Errorf("%w: invalid padding position", ErrInvalidBase64)
	}

	return int64(len(encoded)/4*3 - padding), nil
}

// contextReader 在每次读取前检查 context 状态。
//
// Base64 数据来自内存，读取本身不会长时间阻塞；
// 按读取块检查即可让大数据解码及时响应取消。
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

// Read 实现 io.Reader。
func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
