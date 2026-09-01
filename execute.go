package appium

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/redact"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const truncatedRemoteDataJSON = `{"truncated":true}`

// responseDecoder 负责校验并解码具体命令返回的 value。
//
// decoder 返回错误时，该命令整体视为失败。
// decoder 应在处理较大响应时检查 context 状态。
type responseDecoder func(context.Context, json.RawMessage) error

const (
	executeScriptOperation             = "execute_script"
	maxExecuteScriptOperationByteCount = 64
)

// ExecuteScript 在当前 Session 中执行同步 WebDriver Script。
//
// script 和 arguments 按照 W3C Execute Script 协议原样发送。
// 返回值保留为 json.RawMessage，由调用方根据具体命令语义继续解码。
//
// Appium Driver 的 mobile: execute method 同样通过该接口执行。
func (s *Session) ExecuteScript(
	ctx context.Context,
	script string,
	arguments []any,
) (json.RawMessage, error) {
	return s.executeScriptWithOperation(
		ctx,
		executeScriptOperation,
		script,
		arguments,
	)
}

// ExecuteScriptWithOperation 通过统一命令链执行一次同步 Execute Script。
//
// operation 是调用方提供的低基数诊断 identity，只用于本地 Error 和 Observer，
// 不会发送给远端；远端请求始终使用 W3C Execute Script 的固定路由和请求格式。
// 该方法是高级 Execute Script 入口，适合平台扩展或需要稳定观测名称的调用方；
// 普通调用方不需要独立 identity 时应使用 ExecuteScript。
//
// operation 必须匹配 ASCII 格式 `[a-z][a-z0-9_]{0,63}`。客户端不会根据它构造
// HTTP Method、Route、脚本或参数，也不会因此增加重试、fallback 或其他策略；
// 调用方负责在业务上保持 identity 集合低基数且稳定。
func (s *Session) ExecuteScriptWithOperation(
	ctx context.Context,
	operation string,
	script string,
	arguments []any,
) (json.RawMessage, error) {
	if err := validateExecuteScriptOperation(operation); err != nil {
		return nil, err
	}

	return s.executeScriptWithOperation(
		ctx,
		operation,
		script,
		arguments,
	)
}

// ExecuteScriptWithOperationAndDecode 通过统一命令链执行一次同步 Execute Script，
// 并在命令完成前解码远端返回的 value。
//
// operation 是调用方提供的低基数 ASCII 诊断 identity，格式必须匹配
// `[a-z][a-z0-9_]{0,63}`。decoder 接收 W3C response envelope 中的 value，
// 在 Observer 的 OnCommandFinished 之前同步执行；decoder 返回的错误会进入
// 统一的响应解码错误映射，因此会保留 HTTP StatusCode、Delivery 和 operation。
// 该方法仍然只发送固定的 W3C Execute Script 路由，不是任意 Method/Route 的
// Raw Command API。调用方负责在业务上保持 operation 集合低基数且稳定；decoder
// 为 nil 或 operation 无效时不会发送远端请求。
func (s *Session) ExecuteScriptWithOperationAndDecode(
	ctx context.Context,
	operation string,
	script string,
	arguments []any,
	decoder func(context.Context, json.RawMessage) error,
) error {
	if err := validateExecuteScriptOperation(operation); err != nil {
		return err
	}
	if decoder == nil {
		return &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   "execute script response decoder is nil",
			Delivery:  DeliveryNotSent,
		}
	}

	return s.executeScriptWithOperationDecoder(
		ctx,
		operation,
		script,
		arguments,
		responseDecoder(decoder),
	)
}

// executeScriptWithOperation 在指定的本地 operation identity 下执行同步脚本。
func (s *Session) executeScriptWithOperation(
	ctx context.Context,
	operation string,
	script string,
	arguments []any,
) (json.RawMessage, error) {
	var result json.RawMessage

	err := s.executeScriptWithOperationDecoder(
		ctx,
		operation,
		script,
		arguments,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			if err := ctx.Err(); err != nil {
				return err
			}

			result = append(
				json.RawMessage(nil),
				value...,
			)

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// executeScriptWithOperationDecoder 在指定的本地 operation identity 下执行同步脚本。
func (s *Session) executeScriptWithOperationDecoder(
	ctx context.Context,
	operation string,
	script string,
	arguments []any,
	decoder responseDecoder,
) error {
	client, err := s.commandClient(
		operation,
	)
	if err != nil {
		return err
	}

	return executeScriptCommand(
		ctx,
		client,
		operation,
		s.id,
		script,
		arguments,
		decoder,
	)
}

// validateExecuteScriptOperation 校验高级 Execute Script 入口使用的诊断 identity。
func validateExecuteScriptOperation(operation string) error {
	if len(operation) == 0 ||
		len(operation) > maxExecuteScriptOperationByteCount {
		return invalidExecuteScriptOperationError()
	}

	for index := 0; index < len(operation); index++ {
		char := operation[index]

		if index == 0 {
			if char < 'a' || char > 'z' {
				return invalidExecuteScriptOperationError()
			}
			continue
		}

		if (char < 'a' || char > 'z') &&
			(char < '0' || char > '9') &&
			char != '_' {
			return invalidExecuteScriptOperationError()
		}
	}

	return nil
}

// invalidExecuteScriptOperationError 返回不暴露原始 identity 的参数错误。
func invalidExecuteScriptOperationError() error {
	return &Error{
		Code:      CodeInvalidArgument,
		Operation: executeScriptOperation,
		Message:   "execute script operation must match [a-z][a-z0-9_]{0,63}",
		Delivery:  DeliveryNotSent,
	}
}

// executeScriptCommand 通过统一命令链执行一次同步 Execute Script。
//
// operation 只用于错误和观测事件标识；发送给远端的脚本和参数仍遵循
// W3C Execute Script 请求契约。具体调用方可以提供自己的 value decoder，
// 以便在同一条执行链中完成命令专有的响应校验。
func executeScriptCommand(
	ctx context.Context,
	client *Client,
	operation string,
	sessionID string,
	script string,
	arguments []any,
	decoder responseDecoder,
) error {
	command, err := wire.NewCommand(
		operation,
		http.MethodPost,
		"session",
		sessionID,
		"execute",
		"sync",
	)
	if err != nil {
		return &Error{
			Code:      CodeInvalidConfig,
			Operation: operation,
			Message:   "execute script command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	// W3C 协议要求 args 必须是数组。
	// nil 在 Go 中需要显式转换为空 slice，避免编码成 JSON null。
	if arguments == nil {
		arguments = []any{}
	}

	request := struct {
		Script string `json:"script"`
		Args   []any  `json:"args"`
	}{
		Script: script,
		Args:   arguments,
	}

	return client.executeCommand(
		ctx,
		command,
		request,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		decoder,
	)
}

// executeCommand 执行一条 WebDriver/Appium 远端命令。
//
// payload 为 nil 时不发送请求体；否则会先编码为 JSON。
// timeout 只约束远端命令执行阶段，不包含请求参数编码和响应值解码时间。
//
// decoder 必须负责校验具体命令返回的 value。
// 只有 Transport 和 decoder 均成功时，该命令才视为成功。
func (c *Client) executeCommand(
	ctx context.Context,
	command wire.Command,
	payload any,
	timeout time.Duration,
	responseLimit int64,
	decoder responseDecoder,
) (resultErr error) {
	operation := command.Operation()

	if c == nil || c.transport == nil {
		return &Error{
			Code:      CodeInvalidConfig,
			Operation: operation,
			Message:   "client is not initialized",
			Delivery:  DeliveryNotSent,
		}
	}
	if operation == "" || command.Method() == "" {
		return &Error{
			Code:      CodeInvalidConfig,
			Operation: operation,
			Message:   "command definition is invalid",
			Delivery:  DeliveryNotSent,
		}
	}
	if ctx == nil {
		return &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   "context is nil",
			Delivery:  DeliveryNotSent,
		}
	}
	if timeout <= 0 {
		return &Error{
			Code:      CodeInvalidConfig,
			Operation: operation,
			Message:   "command timeout must be positive",
			Delivery:  DeliveryNotSent,
		}
	}
	if responseLimit <= 0 {
		return &Error{
			Code:      CodeInvalidConfig,
			Operation: operation,
			Message:   "response limit must be positive",
			Delivery:  DeliveryNotSent,
		}
	}
	if decoder == nil {
		return &Error{
			Code:      CodeInvalidConfig,
			Operation: operation,
			Message:   "response decoder is nil",
			Delivery:  DeliveryNotSent,
		}
	}

	if err := ctx.Err(); err != nil {
		return contextExecutionError(
			operation,
			0,
			DeliveryNotSent,
			err,
			err,
		)
	}

	var body []byte

	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return &Error{
				Code:      CodeInvalidArgument,
				Operation: operation,
				Message:   "request payload cannot be encoded as JSON",
				Delivery:  DeliveryNotSent,
				Cause:     err,
			}
		}
		body = encoded
	}

	if err := ctx.Err(); err != nil {
		return contextExecutionError(
			operation,
			0,
			DeliveryNotSent,
			err,
			err,
		)
	}

	startedAt := time.Now()

	if c.observer != nil {
		c.observer.OnCommandStarted(CommandStartedEvent{
			Operation:    operation,
			StartedAt:    startedAt,
			RequestBytes: int64(len(body)),
		})
	}

	var response wire.Response

	defer func() {
		c.observeCommandFinished(
			operation,
			startedAt,
			response,
			resultErr,
		)
	}()

	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	response, executeErr := c.transport.Execute(
		commandCtx,
		command,
		body,
		responseLimit,
	)

	if executeErr != nil {
		resultErr = c.mapWireFailure(
			commandCtx,
			operation,
			response,
			executeErr,
		)
		cancel()
		return resultErr
	}

	cancel()

	if err := decoder(ctx, response.Value); err != nil {
		resultErr = mapResponseDecodeFailure(
			operation,
			response,
			err,
		)
		return resultErr
	}

	return nil
}

// mapWireFailure 将 wire 层错误映射为客户端公开错误。
func (c *Client) mapWireFailure(
	ctx context.Context,
	operation string,
	response wire.Response,
	err error,
) error {
	delivery := deliveryFromWireResponse(response)

	var failure *wire.Failure
	if !errors.As(err, &failure) {
		return &Error{
			Code:       CodeTransportFailed,
			Operation:  operation,
			Message:    "WebDriver command execution failed",
			StatusCode: response.StatusCode,
			Delivery:   delivery,
			Cause:      err,
		}
	}

	switch failure.Kind {
	case wire.FailureRequest:
		return &Error{
			Code:       CodeInvalidConfig,
			Operation:  operation,
			Message:    "WebDriver request could not be constructed",
			StatusCode: response.StatusCode,
			Delivery:   delivery,
			Cause:      err,
		}

	case wire.FailureTransport, wire.FailureResponseRead:
		if ctxErr := ctx.Err(); ctxErr != nil {
			return contextExecutionError(
				operation,
				response.StatusCode,
				delivery,
				ctxErr,
				errors.Join(ctxErr, err),
			)
		}

		return &Error{
			Code:       CodeTransportFailed,
			Operation:  operation,
			Message:    "WebDriver transport failed",
			StatusCode: response.StatusCode,
			Delivery:   delivery,
			Cause:      err,
		}

	case wire.FailureResponseTooLarge:
		return &Error{
			Code:       CodeResponseTooLarge,
			Operation:  operation,
			Message:    "WebDriver response exceeds configured limit",
			StatusCode: response.StatusCode,
			Delivery:   delivery,
			Cause:      err,
		}

	case wire.FailureResponseInvalid:
		return &Error{
			Code:       CodeResponseInvalid,
			Operation:  operation,
			Message:    "WebDriver response is invalid",
			StatusCode: response.StatusCode,
			Delivery:   delivery,
			Cause:      err,
		}

	case wire.FailureRemote:
		return c.mapRemoteError(
			operation,
			response.StatusCode,
			delivery,
			err,
		)

	default:
		return &Error{
			Code:       CodeTransportFailed,
			Operation:  operation,
			Message:    "unknown wire failure",
			StatusCode: response.StatusCode,
			Delivery:   delivery,
			Cause:      err,
		}
	}
}

// mapResponseDecodeFailure 将命令级响应解码错误映射为公开错误。
func mapResponseDecodeFailure(
	operation string,
	response wire.Response,
	err error,
) error {
	switch {
	case errors.Is(err, codec.ErrOutputWrite):
		var outputErr *codec.OutputError
		cause := err
		if errors.As(err, &outputErr) && outputErr.Cause != nil {
			cause = outputErr.Cause
		}

		return &Error{
			Code:       CodeOutputFailed,
			Operation:  operation,
			Message:    "WebDriver response value could not be written to output",
			StatusCode: response.StatusCode,
			Delivery:   DeliveryAcknowledged,
			Cause:      cause,
		}

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return contextExecutionError(
			operation,
			response.StatusCode,
			DeliveryAcknowledged,
			err,
			err,
		)

	case errors.Is(err, codec.ErrBase64TooLarge):
		return &Error{
			Code:       CodeResponseTooLarge,
			Operation:  operation,
			Message:    "WebDriver response value exceeds configured limit",
			StatusCode: response.StatusCode,
			Delivery:   DeliveryAcknowledged,
			Cause:      err,
		}

	default:
		return &Error{
			Code:       CodeResponseInvalid,
			Operation:  operation,
			Message:    "WebDriver response value is invalid",
			StatusCode: response.StatusCode,
			Delivery:   DeliveryAcknowledged,
			Cause:      err,
		}
	}
}

// mapRemoteError 将远端 W3C WebDriver 错误映射为公开错误。
func (c *Client) mapRemoteError(
	operation string,
	statusCode int,
	delivery DeliveryState,
	err error,
) error {
	var remote *wire.RemoteError
	if !errors.As(err, &remote) {
		return &Error{
			Code:       CodeResponseInvalid,
			Operation:  operation,
			Message:    "remote WebDriver error is invalid",
			StatusCode: statusCode,
			Delivery:   delivery,
			Cause:      err,
		}
	}

	message := sanitizeRemoteMessage(
		remote.Message,
		c.limits.MaxRemoteErrorBytes,
	)
	if message == "" {
		message = "remote WebDriver command failed"
	}

	return &Error{
		Code:       mapRemoteErrorCode(remote.Code),
		Operation:  operation,
		Message:    message,
		StatusCode: statusCode,
		RemoteCode: remote.Code,
		Delivery:   delivery,
		RemoteData: sanitizeRemoteData(
			remote.Value,
			c.limits.MaxRemoteErrorBytes,
		),
		Cause: err,
	}
}

// mapRemoteErrorCode 将 W3C WebDriver 错误码映射为客户端错误类别。
func mapRemoteErrorCode(remoteCode string) ErrorCode {
	switch remoteCode {
	case "invalid argument", "invalid selector":
		return CodeInvalidArgument

	case "invalid session id":
		return CodeSessionLost

	case "no such element":
		return CodeElementNotFound

	case "no such alert":
		return CodeAlertNotFound

	case "stale element reference":
		return CodeElementStale

	case "unknown command", "unknown method", "unsupported operation":
		return CodeUnsupported

	default:
		return CodeCommandFailed
	}
}

// deliveryFromWireResponse 根据 wire 层事实推导命令投递状态。
func deliveryFromWireResponse(response wire.Response) DeliveryState {
	if response.ResponseReceived {
		return DeliveryAcknowledged
	}
	if response.RequestAttempted {
		return DeliveryUnknown
	}
	return DeliveryNotSent
}

// contextExecutionError 将 context 结束状态映射为客户端错误。
func contextExecutionError(
	operation string,
	statusCode int,
	delivery DeliveryState,
	ctxErr error,
	cause error,
) error {
	code := CodeCanceled
	message := "operation canceled"

	if errors.Is(ctxErr, context.DeadlineExceeded) {
		code = CodeDeadlineExceeded
		message = "operation deadline exceeded"
	}

	return &Error{
		Code:       code,
		Operation:  operation,
		Message:    message,
		StatusCode: statusCode,
		Delivery:   delivery,
		Cause:      cause,
	}
}

// observeCommandFinished 发送命令完成观测事件。
func (c *Client) observeCommandFinished(
	operation string,
	startedAt time.Time,
	response wire.Response,
	err error,
) {
	if c.observer == nil {
		return
	}

	event := CommandFinishedEvent{
		Operation:     operation,
		Duration:      time.Since(startedAt),
		StatusCode:    response.StatusCode,
		RequestBytes:  response.RequestBytes,
		ResponseBytes: response.ResponseBytes,
		Delivery:      deliveryFromWireResponse(response),
	}

	var clientErr *Error
	if errors.As(err, &clientErr) {
		event.ErrorCode = clientErr.Code
		event.Delivery = clientErr.Delivery
	}

	c.observer.OnCommandFinished(event)
}

// sanitizeRemoteMessage 对远端错误文本进行脱敏并限制保留大小。
func sanitizeRemoteMessage(message string, maxBytes int64) string {
	message = redact.Text(message)

	if maxBytes <= 0 || message == "" {
		return ""
	}
	if int64(len(message)) <= maxBytes {
		return message
	}

	const suffix = "...[truncated]"

	if maxBytes <= int64(len(suffix)) {
		return truncateUTF8(message, int(maxBytes))
	}

	prefix := truncateUTF8(
		message,
		int(maxBytes)-len(suffix),
	)

	return prefix + suffix
}

// sanitizeRemoteData 对远端错误 JSON 进行脱敏并限制保留大小。
func sanitizeRemoteData(
	value json.RawMessage,
	maxBytes int64,
) json.RawMessage {
	if maxBytes <= 0 || len(value) == 0 {
		return nil
	}

	sanitized, err := redact.JSON(value)
	if err != nil {
		return nil
	}

	if int64(len(sanitized)) <= maxBytes {
		return append(json.RawMessage(nil), sanitized...)
	}

	if int64(len(truncatedRemoteDataJSON)) <= maxBytes {
		return json.RawMessage(truncatedRemoteDataJSON)
	}

	return nil
}

// truncateUTF8 在不破坏 UTF-8 字符边界的情况下截断字符串。
func truncateUTF8(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}

	end := maxBytes

	for end > 0 &&
		end < len(value) &&
		!utf8.RuneStart(value[end]) {
		end--
	}

	return value[:end]
}
