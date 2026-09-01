package appium

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	getLogTypesOperation = "get_log_types"
	getLogsOperation     = "get_logs"

	logJSONContextCheckInterval = 32 << 10
)

// LogType 表示由当前远端 Driver 提供的开放日志类型标识符。
//
// 客户端不维护日志类型枚举，也不会对合法 UTF-8 值执行大小写、空白或别名规范化。
// 非法 UTF-8 值无法无损编码为 JSON string，作为 Logs 参数时会在发送前被拒绝。
type LogType string

// LogEntry 表示一次 Pull Logs 调用返回的日志条目。
//
// Timestamp 是 Unix epoch 毫秒。Extra 保存远端返回的未知字段；没有未知字段
// 时为 nil。
type LogEntry struct {
	// Timestamp 表示日志条目的 Unix epoch 毫秒时间戳。
	Timestamp int64

	// Level 表示远端提供的原始日志级别文本。
	Level string

	// Message 表示远端提供的原始日志消息文本。
	Message string

	// Extra 保存除标准字段外的未知 JSON 字段及其递归值。
	Extra map[string]any
}

// LogTypes 读取当前 Session 在调用时刻可用的日志类型快照。
//
// 结果按远端顺序返回；空数组是合法的非 nil 空 slice。客户端不缓存结果，
// 也不会因为日志类型为空或未知而在本地拒绝后续 Logs 调用。
func (s *Session) LogTypes(
	ctx context.Context,
) ([]LogType, error) {
	client, err := s.commandClient(getLogTypesOperation)
	if err != nil {
		return nil, err
	}

	command, err := wire.NewCommand(
		getLogTypesOperation,
		http.MethodGet,
		"session",
		s.id,
		"se",
		"log",
		"types",
	)
	if err != nil {
		return nil, commandDefinitionError(
			getLogTypesOperation,
			"get log types command definition is invalid",
			err,
		)
	}

	var logTypes []LogType
	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxLogResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decodeLogTypes(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			logTypes = decoded
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return logTypes, nil
}

// Logs 按指定的开放日志类型读取当前 Session 的一次性日志快照。
//
// 合法 UTF-8 的 logType 会原样放入请求体，包括空字符串、大小写和空白；是否支持
// 该值由远端 Driver 决定。非法 UTF-8 无法无损编码为 JSON string，会在发送前返回
// CodeInvalidArgument。每次调用只发送一个请求，不缓存、轮询、合并、去重或重试。
func (s *Session) Logs(
	ctx context.Context,
	logType LogType,
) ([]LogEntry, error) {
	client, err := s.commandClient(getLogsOperation)
	if err != nil {
		return nil, err
	}

	if !utf8.ValidString(string(logType)) {
		return nil, &Error{
			Code:      CodeInvalidArgument,
			Operation: getLogsOperation,
			Message:   "log type must be valid UTF-8",
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		getLogsOperation,
		http.MethodPost,
		"session",
		s.id,
		"se",
		"log",
	)
	if err != nil {
		return nil, commandDefinitionError(
			getLogsOperation,
			"get logs command definition is invalid",
			err,
		)
	}

	request := struct {
		Type LogType `json:"type"`
	}{
		Type: logType,
	}

	var entries []LogEntry
	err = client.executeCommand(
		ctx,
		command,
		request,
		client.commandTimeout,
		client.limits.MaxLogResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decodeLogs(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			entries = decoded
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// decodeLogTypes 严格解码 LogTypes 的 JSON string array value。
func decodeLogTypes(
	ctx context.Context,
	value json.RawMessage,
) ([]LogType, error) {
	items, err := decodeLogArray(ctx, value, "log types response")
	if err != nil {
		return nil, err
	}

	logTypes := make([]LogType, len(items))
	for index, item := range items {
		if err := checkLogContext(ctx); err != nil {
			return nil, err
		}

		decoded, decodeErr := decodeRequiredLogString(ctx, item)
		if decodeErr != nil {
			return nil, fmt.Errorf(
				"decode log type at index %d: %w",
				index,
				decodeErr,
			)
		}
		logTypes[index] = LogType(decoded)
	}

	if err := checkLogContext(ctx); err != nil {
		return nil, err
	}

	return logTypes, nil
}

// decodeLogs 严格解码 Logs 的 JSON LogEntry array value。
func decodeLogs(
	ctx context.Context,
	value json.RawMessage,
) ([]LogEntry, error) {
	items, err := decodeLogArray(ctx, value, "logs response")
	if err != nil {
		return nil, err
	}

	entries := make([]LogEntry, len(items))
	for index, item := range items {
		if err := checkLogContext(ctx); err != nil {
			return nil, err
		}

		entry, decodeErr := decodeLogEntry(ctx, item)
		if decodeErr != nil {
			return nil, fmt.Errorf(
				"decode log entry at index %d: %w",
				index,
				decodeErr,
			)
		}
		entries[index] = entry
	}

	if err := checkLogContext(ctx); err != nil {
		return nil, err
	}

	return entries, nil
}

// decodeLogEntry 严格解码一个 LogEntry 对象并复制未知字段。
func decodeLogEntry(
	ctx context.Context,
	raw json.RawMessage,
) (LogEntry, error) {
	fields, err := decodeLogObject(ctx, raw, "log entry")
	if err != nil {
		return LogEntry{}, err
	}

	timestampRaw, ok := fields["timestamp"]
	if !ok || len(bytes.TrimSpace(timestampRaw)) == 0 {
		return LogEntry{}, errors.New(
			"log entry does not contain timestamp",
		)
	}
	levelRaw, ok := fields["level"]
	if !ok || len(bytes.TrimSpace(levelRaw)) == 0 {
		return LogEntry{}, errors.New(
			"log entry does not contain level",
		)
	}
	messageRaw, ok := fields["message"]
	if !ok || len(bytes.TrimSpace(messageRaw)) == 0 {
		return LogEntry{}, errors.New(
			"log entry does not contain message",
		)
	}

	timestampValue, err := decodeLogJSONValue(ctx, timestampRaw)
	if err != nil {
		return LogEntry{}, fmt.Errorf(
			"decode log entry timestamp: %w",
			err,
		)
	}
	timestampNumber, ok := timestampValue.(json.Number)
	if !ok {
		return LogEntry{}, errors.New(
			"log entry timestamp must be a JSON integer",
		)
	}
	timestamp, err := timestampNumber.Int64()
	if err != nil {
		return LogEntry{}, fmt.Errorf(
			"log entry timestamp must be an int64 JSON integer: %w",
			err,
		)
	}

	level, err := decodeRequiredLogString(ctx, levelRaw)
	if err != nil {
		return LogEntry{}, fmt.Errorf(
			"decode log entry level: %w",
			err,
		)
	}
	message, err := decodeRequiredLogString(ctx, messageRaw)
	if err != nil {
		return LogEntry{}, fmt.Errorf(
			"decode log entry message: %w",
			err,
		)
	}

	var extra map[string]any
	for key, rawValue := range fields {
		if key == "timestamp" || key == "level" || key == "message" {
			continue
		}

		if err := checkLogContext(ctx); err != nil {
			return LogEntry{}, err
		}

		decoded, decodeErr := decodeLogJSONValue(ctx, rawValue)
		if decodeErr != nil {
			return LogEntry{}, fmt.Errorf(
				"decode unknown log entry field %q: %w",
				key,
				decodeErr,
			)
		}

		if extra == nil {
			extra = make(map[string]any)
		}
		cloned, cloneErr := cloneLogJSONValue(ctx, decoded)
		if cloneErr != nil {
			return LogEntry{}, fmt.Errorf(
				"copy unknown log entry field %q: %w",
				key,
				cloneErr,
			)
		}
		extra[key] = cloned
	}

	if err := checkLogContext(ctx); err != nil {
		return LogEntry{}, err
	}

	return LogEntry{
		Timestamp: timestamp,
		Level:     level,
		Message:   message,
		Extra:     extra,
	}, nil
}

// decodeRequiredLogString 严格解码必需的 JSON string 字段。
//
// encoding/json 将显式 null 解码到 string 变量时视为零值，因此这里先单独
// 拒绝 null，再复用 codec 的 UTF-8、JSON 语法和 surrogate 校验。
func decodeRequiredLogString(
	ctx context.Context,
	raw json.RawMessage,
) (string, error) {
	if err := checkLogContext(ctx); err != nil {
		return "", err
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", errors.New("log field must be a JSON string")
	}

	return codec.DecodeJSONString(ctx, raw)
}

// decodeLogArray 校验并解码一个 JSON array，同时保留显式空数组的存在性。
func decodeLogArray(
	ctx context.Context,
	raw json.RawMessage,
	label string,
) ([]json.RawMessage, error) {
	if err := validateLogJSON(ctx, raw); err != nil {
		return nil, err
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, fmt.Errorf("%s must be a JSON array", label)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(trimmed, &items); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	if items == nil {
		items = make([]json.RawMessage, 0)
	}

	if err := checkLogContext(ctx); err != nil {
		return nil, err
	}

	return items, nil
}

// decodeLogObject 校验并解码一个 JSON object。
func decodeLogObject(
	ctx context.Context,
	raw json.RawMessage,
	label string,
) (map[string]json.RawMessage, error) {
	if err := validateLogJSON(ctx, raw); err != nil {
		return nil, err
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("%s must be a JSON object", label)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	if fields == nil {
		return nil, fmt.Errorf("%s must be a JSON object", label)
	}

	if err := checkLogContext(ctx); err != nil {
		return nil, err
	}

	return fields, nil
}

// decodeLogJSONValue 解码开放未知字段并以 json.Number 保留数字精度。
func decodeLogJSONValue(
	ctx context.Context,
	raw json.RawMessage,
) (any, error) {
	if err := validateLogJSON(ctx, raw); err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(raw)))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode JSON value: %w", err)
	}

	if err := checkLogContext(ctx); err != nil {
		return nil, err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("JSON value contains multiple values")
		}
		return nil, fmt.Errorf("decode trailing JSON value: %w", err)
	}

	if err := checkLogContext(ctx); err != nil {
		return nil, err
	}

	return value, nil
}

// cloneLogJSONValue 深拷贝未知字段，并在递归过程中检查调用方 context。
func cloneLogJSONValue(
	ctx context.Context,
	value any,
) (any, error) {
	if err := checkLogContext(ctx); err != nil {
		return nil, err
	}

	switch current := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(current))
		for key, child := range current {
			if err := checkLogContext(ctx); err != nil {
				return nil, err
			}

			copied, err := cloneLogJSONValue(ctx, child)
			if err != nil {
				return nil, err
			}
			cloned[key] = copied
		}
		return cloned, nil

	case []any:
		cloned := make([]any, len(current))
		for index, child := range current {
			if err := checkLogContext(ctx); err != nil {
				return nil, err
			}

			copied, err := cloneLogJSONValue(ctx, child)
			if err != nil {
				return nil, err
			}
			cloned[index] = copied
		}
		return cloned, nil

	default:
		return current, nil
	}
}

// validateLogJSON 校验 JSON 的 UTF-8 编码和字符串 surrogate 配对。
func validateLogJSON(
	ctx context.Context,
	raw json.RawMessage,
) error {
	if err := checkLogContext(ctx); err != nil {
		return err
	}
	if err := codec.ValidateUTF8(ctx, raw); err != nil {
		return err
	}
	if err := validateLogJSONStringEscapes(ctx, raw); err != nil {
		return err
	}
	return checkLogContext(ctx)
}

// validateLogJSONStringEscapes 拒绝 JSON 字符串中的未配对 Unicode surrogate。
func validateLogJSONStringEscapes(
	ctx context.Context,
	raw []byte,
) error {
	if ctx == nil {
		return errors.New("context is nil")
	}

	nextCheck := 0
	for offset := 0; offset < len(raw); {
		if offset >= nextCheck {
			if err := ctx.Err(); err != nil {
				return err
			}
			nextCheck = offset + logJSONContextCheckInterval
		}

		if raw[offset] != '"' {
			offset++
			continue
		}

		offset++
		for {
			if offset >= len(raw) {
				return codec.ErrInvalidJSON
			}

			if offset >= nextCheck {
				if err := ctx.Err(); err != nil {
					return err
				}
				nextCheck = offset + logJSONContextCheckInterval
			}

			switch raw[offset] {
			case '"':
				offset++
				goto nextString

			case '\\':
				if offset+1 >= len(raw) {
					return codec.ErrInvalidJSON
				}

				if raw[offset+1] != 'u' {
					offset += 2
					continue
				}

				value, ok := decodeLogHex4(raw, offset+2)
				if !ok {
					return codec.ErrInvalidJSON
				}
				offset += 6

				switch {
				case value >= 0xd800 && value <= 0xdbff:
					if offset+6 > len(raw) ||
						raw[offset] != '\\' ||
						raw[offset+1] != 'u' {
						return codec.ErrInvalidJSONSurrogate
					}

					low, ok := decodeLogHex4(raw, offset+2)
					if !ok || low < 0xdc00 || low > 0xdfff {
						return codec.ErrInvalidJSONSurrogate
					}
					offset += 6

				case value >= 0xdc00 && value <= 0xdfff:
					return codec.ErrInvalidJSONSurrogate
				}

			default:
				offset++
			}
		}

	nextString:
	}

	return ctx.Err()
}

// decodeLogHex4 解码 JSON \u 转义中的四位十六进制数。
func decodeLogHex4(raw []byte, offset int) (uint16, bool) {
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

// checkLogContext 统一检查 Pull Logs 解码过程中的调用方 context。
func checkLogContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	return ctx.Err()
}
