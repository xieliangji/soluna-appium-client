package appium

import (
	"time"

	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	newClientOperation = "new_client"

	defaultCommandTimeout        = 120 * time.Second
	defaultReadyProbeTimeout     = 5 * time.Second
	defaultSessionCleanupTimeout = 10 * time.Second
)

// Client 表示一个固定 Appium Server Endpoint 的客户端。
//
// Client 保存客户端级配置和底层传输能力，可以被多个 goroutine 安全地共享。
// Client 不负责 Appium Server 的启动、停止或进程生命周期管理。
type Client struct {
	transport             *wire.Transport
	commandTimeout        time.Duration
	readyProbeTimeout     time.Duration
	sessionCleanupTimeout time.Duration
	limits                Limits
	observer              Observer
}

// NewClient 创建 Appium Client。
//
// serverURL 表示固定的 Appium Server 地址。
// options 使用零值时采用客户端默认配置。
func NewClient(serverURL string, options ClientOptions) (*Client, error) {
	normalized, err := normalizeClientOptions(options)
	if err != nil {
		return nil, err
	}

	endpoint, err := wire.NewEndpoint(serverURL)
	if err != nil {
		return nil, &Error{
			Code:      CodeInvalidConfig,
			Operation: newClientOperation,
			Message:   "Appium Server URL 无效",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	transport, err := wire.NewTransport(endpoint, normalized.HTTPClient)
	if err != nil {
		return nil, &Error{
			Code:      CodeInvalidConfig,
			Operation: newClientOperation,
			Message:   "HTTP Client 配置无效",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	return &Client{
		transport:             transport,
		commandTimeout:        normalized.CommandTimeout,
		readyProbeTimeout:     normalized.ReadyProbeTimeout,
		sessionCleanupTimeout: normalized.SessionCleanupTimeout,
		limits:                normalized.Limits,
		observer:              normalized.Observer,
	}, nil
}

// normalizeClientOptions 校验并补全 ClientOptions 的默认值。
func normalizeClientOptions(options ClientOptions) (ClientOptions, error) {
	if options.CommandTimeout < 0 {
		return ClientOptions{}, invalidClientConfig("命令超时时间不能为负数")
	}
	if options.ReadyProbeTimeout < 0 {
		return ClientOptions{}, invalidClientConfig("就绪探测超时时间不能为负数")
	}
	if options.SessionCleanupTimeout < 0 {
		return ClientOptions{}, invalidClientConfig("Session 清理超时时间不能为负数")
	}

	if options.CommandTimeout == 0 {
		options.CommandTimeout = defaultCommandTimeout
	}
	if options.ReadyProbeTimeout == 0 {
		options.ReadyProbeTimeout = defaultReadyProbeTimeout
	}
	if options.SessionCleanupTimeout == 0 {
		options.SessionCleanupTimeout = defaultSessionCleanupTimeout
	}

	limits, err := normalizeLimits(options.Limits)
	if err != nil {
		return ClientOptions{}, err
	}
	options.Limits = limits

	return options, nil
}

// normalizeLimits 校验并补全资源限制配置。
func normalizeLimits(limits Limits) (Limits, error) {
	if limits.MaxResponseBytes < 0 {
		return Limits{}, invalidClientConfig("普通响应大小上限不能为负数")
	}
	if limits.MaxPageSourceResponseBytes < 0 {
		return Limits{}, invalidClientConfig("Page Source 响应大小上限不能为负数")
	}
	if limits.MaxScreenshotResponseBytes < 0 {
		return Limits{}, invalidClientConfig("Screenshot 响应大小上限不能为负数")
	}
	if limits.MaxRecordingResponseBytes < 0 {
		return Limits{}, invalidClientConfig("录屏响应大小上限不能为负数")
	}
	if limits.MaxRemoteErrorBytes < 0 {
		return Limits{}, invalidClientConfig("远端错误数据大小上限不能为负数")
	}

	defaults := DefaultLimits()

	if limits.MaxResponseBytes == 0 {
		limits.MaxResponseBytes = defaults.MaxResponseBytes
	}
	if limits.MaxPageSourceResponseBytes == 0 {
		limits.MaxPageSourceResponseBytes = defaults.MaxPageSourceResponseBytes
	}
	if limits.MaxScreenshotResponseBytes == 0 {
		limits.MaxScreenshotResponseBytes = defaults.MaxScreenshotResponseBytes
	}
	if limits.MaxRecordingResponseBytes == 0 {
		limits.MaxRecordingResponseBytes = defaults.MaxRecordingResponseBytes
	}
	if limits.MaxRemoteErrorBytes == 0 {
		limits.MaxRemoteErrorBytes = defaults.MaxRemoteErrorBytes
	}

	return limits, nil
}

// invalidClientConfig 创建 Client 配置错误。
func invalidClientConfig(message string) error {
	return &Error{
		Code:      CodeInvalidConfig,
		Operation: newClientOperation,
		Message:   message,
		Delivery:  DeliveryNotSent,
	}
}
