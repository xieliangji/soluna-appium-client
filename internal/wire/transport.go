package wire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptrace"
	"sync/atomic"
)

// FailureKind 表示命令在 wire 层失败的阶段。
type FailureKind string

const (
	FailureRequest          FailureKind = "request"            // 请求地址或 HTTP Request 构造失败
	FailureTransport        FailureKind = "transport"          // HTTP 传输失败
	FailureResponseRead     FailureKind = "response_read"      // 响应体读取失败
	FailureResponseTooLarge FailureKind = "response_too_large" // 响应体超过允许上限
	FailureResponseInvalid  FailureKind = "response_invalid"   // 响应不符合 W3C 协议
	FailureRemote           FailureKind = "remote"             // 远端明确返回 WebDriver 错误
)

// Failure 表示 wire 层的一次命令失败。
//
// Failure 只描述失败发生在哪个阶段，具体公共错误码由上层映射。
type Failure struct {
	Kind  FailureKind
	Cause error
}

// Error 返回 wire 层错误描述。
func (e *Failure) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause == nil {
		return "wire failure: " + string(e.Kind)
	}
	return "wire failure: " + string(e.Kind) + ": " + e.Cause.Error()
}

// Unwrap 返回底层错误。
func (e *Failure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Response 表示一次命令执行后 wire 层能够确认的结果。
type Response struct {
	// StatusCode 表示远端 HTTP 状态码。
	//
	// 尚未收到 HTTP 响应时为零。
	StatusCode int

	// RequestBytes 表示发送的请求体大小。
	RequestBytes int64

	// ResponseBytes 表示实际读取到的响应体大小。
	ResponseBytes int64

	// RequestAttempted 表示请求可能已经开始写入网络连接。
	//
	// 该字段为 true 不代表远端一定已经收到完整请求。
	RequestAttempted bool

	// ResponseReceived 表示客户端已经收到远端 HTTP 响应头。
	ResponseReceived bool

	// Value 保存 W3C Envelope 中的 value。
	//
	// 对成功响应和远端错误响应都会保留该值。
	Value json.RawMessage
}

// Transport 负责执行 WebDriver/Appium HTTP 命令。
type Transport struct {
	endpoint   *Endpoint
	httpClient *http.Client
}

// NewTransport 创建 wire Transport。
//
// httpClient 为 nil 时使用默认 HTTP Client。
// Transport 不跟随 HTTP Redirect。
func NewTransport(endpoint *Endpoint, httpClient *http.Client) (*Transport, error) {
	if endpoint == nil {
		return nil, errors.New("endpoint is nil")
	}

	if httpClient == nil {
		httpClient = &http.Client{}
	}
	if httpClient.Timeout != 0 {
		return nil, errors.New("HTTP client timeout must be zero")
	}

	client := *httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &Transport{
		endpoint:   endpoint,
		httpClient: &client,
	}, nil
}

// Execute 执行一条远端命令。
//
// body 必须是已经编码完成的 JSON 请求体。
// body 为 nil 时不会发送 Content-Type。
//
// responseLimit 必须为正数，表示允许读取的最大响应体大小。
func (t *Transport) Execute(
	ctx context.Context,
	command Command,
	body []byte,
	responseLimit int64,
) (Response, error) {
	result := Response{
		RequestBytes: int64(len(body)),
	}

	if ctx == nil {
		return result, &Failure{
			Kind:  FailureRequest,
			Cause: errors.New("context is nil"),
		}
	}
	if t == nil || t.endpoint == nil || t.httpClient == nil {
		return result, &Failure{
			Kind:  FailureRequest,
			Cause: errors.New("transport is not initialized"),
		}
	}
	if responseLimit <= 0 {
		return result, &Failure{
			Kind:  FailureRequest,
			Cause: errors.New("response limit must be positive"),
		}
	}

	target, err := command.URL(t.endpoint)
	if err != nil {
		return result, &Failure{
			Kind:  FailureRequest,
			Cause: err,
		}
	}

	var requestBody io.Reader
	if body != nil {
		requestBody = bytes.NewReader(body)
	}

	var requestAttempted atomic.Bool

	trace := &httptrace.ClientTrace{
		WroteRequest: func(httptrace.WroteRequestInfo) {
			requestAttempted.Store(true)
		},
	}
	requestCtx := httptrace.WithClientTrace(ctx, trace)

	request, err := http.NewRequestWithContext(
		requestCtx,
		command.Method(),
		target,
		requestBody,
	)
	if err != nil {
		return result, &Failure{
			Kind:  FailureRequest,
			Cause: err,
		}
	}

	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := t.httpClient.Do(request)
	result.RequestAttempted = requestAttempted.Load()

	if err != nil {
		return result, &Failure{
			Kind:  FailureTransport,
			Cause: err,
		}
	}
	defer response.Body.Close()

	result.ResponseReceived = true
	result.StatusCode = response.StatusCode

	// 比配置上限多读一个字节即可识别超限响应，无需保留完整的超限 Body。
	// 调用方配置 int64 最大正值时不能让加一操作溢出。
	readLimit := responseLimit
	if responseLimit < math.MaxInt64 {
		readLimit++
	}

	data, err := io.ReadAll(
		io.LimitReader(response.Body, readLimit),
	)
	result.ResponseBytes = int64(len(data))

	if err != nil {
		return result, &Failure{
			Kind:  FailureResponseRead,
			Cause: err,
		}
	}

	if int64(len(data)) > responseLimit {
		return result, &Failure{
			Kind: FailureResponseTooLarge,
			Cause: fmt.Errorf(
				"response body exceeds limit: limit=%d",
				responseLimit,
			),
		}
	}

	envelope, err := DecodeEnvelope(data)
	if err != nil {
		return result, &Failure{
			Kind:  FailureResponseInvalid,
			Cause: err,
		}
	}

	result.Value = append(json.RawMessage(nil), envelope.Value...)

	if successfulStatus(response.StatusCode) {
		return result, nil
	}

	remoteError, err := DecodeRemoteError(envelope.Value)
	if err != nil {
		return result, &Failure{
			Kind:  FailureResponseInvalid,
			Cause: err,
		}
	}

	return result, &Failure{
		Kind:  FailureRemote,
		Cause: remoteError,
	}
}

// successfulStatus 判断 HTTP 状态码是否表示成功响应。
func successfulStatus(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}
