package soluna_appium_client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

// Appium 3 当前官方协议：POST /session 成功值是 {sessionId, capabilities}，DELETE /session/:sessionId 成功值是 null。

const (
	createSessionOperation  = "create_session"
	deleteSessionOperation  = "delete_session"
	cleanupSessionOperation = "cleanup_session"

	getWindowRectOperation = "get_window_rect"
	screenshotOperation    = "screenshot"
	pageSourceOperation    = "page_source"
)

// Session 表示一次远端 WebDriver 物理会话.
//
// Session 与创建它的 Client 绑定。
// Session 不负责逻辑会话恢复，也不会在远端 Session 丢失后自动重建。
type Session struct {
	client         *Client
	id             string
	capabilities   Capabilities
	automationName string
	usable         bool
	state          *sessionState
}

// sessionState 保存 Session 的可变生命周期状态.
//
// 单独使用指针状态可以避免 Session 值被复制后产生独立的关闭状态。
type sessionState struct {
	closeMu sync.Mutex
	closed  atomic.Bool
}

// createSessionResult 保存创建 Session 响应的解析结果.
//
// SessionID 会优先于 Capabilities 被解析，以便后续解析失败时
// 仍然能够清理已经创建的远端 Session。
type createSessionResult struct {
	SessionID      string
	Capabilities   Capabilities
	AutomationName string
}

// CreateSession 创建新的远端 WebDriver Session.
//
// capabilities 使用 W3C WebDriver capabilities 结构发送。
// 客户端不会自动修改 Capability 名称或内容.
//
// 如果远端已经返回 Session ID，但后续响应解析或调用方 context 结束，
// 客户端会使用独立的清理超时尽力删除已经创建的 Session。
//
// 如果自动清理失败，本方法会同时返回非 nil Session 和 error。
// 此时返回的 Session 仅用于识别和显式关闭该远端物理 Session，
// 不应继续用于普通 WebDriver 命令。
func (c *Client) CreateSession(
	ctx context.Context,
	capabilities W3CCapabilities,
) (*Session, error) {
	command, err := wire.NewCommand(
		createSessionOperation,
		http.MethodPost,
		"session",
	)
	if err != nil {
		return nil, &Error{
			Code:      CodeInvalidConfig,
			Operation: createSessionOperation,
			Message:   "create session command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	request := struct {
		Capabilities W3CCapabilities `json:"capabilities"`
	}{
		Capabilities: capabilities,
	}

	var result createSessionResult

	err = c.executeCommand(
		ctx,
		command,
		request,
		c.commandTimeout,
		c.limits.MaxResponseBytes,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			decoded, decodeErr := decodeCreateSessionResponse(
				ctx,
				value,
			)

			// 即使解码失败，也保留已经成功解析出的 Session ID。
			result = decoded

			return decodeErr
		},
	)
	if err == nil {
		return newSession(
			c,
			result.SessionID,
			result.Capabilities,
			result.AutomationName,
			true,
		), nil
	}

	if result.SessionID == "" {
		return nil, err
	}

	cleanupErr := c.cleanupSession(result.SessionID)
	if cleanupErr == nil {
		return nil, err
	}

	// 自动清理失败时必须把远端 Session 句柄交还调用方，
	// 避免已经创建的物理 Session 对调用方完全不可见。
	session := newSession(
		c,
		result.SessionID,
		nil,
		"",
		false,
	)

	return session, errors.Join(
		err,
		fmt.Errorf(
			"automatic cleanup of created session failed: %w",
			cleanupErr,
		),
	)
}

// ID 返回远端 WebDriver Session ID.
func (s *Session) ID() string {
	if s == nil {
		return ""
	}
	return s.id
}

// Capabilities 返回创建 Session 后远端确认的 Capability 快照.
//
// 返回值是深拷贝，调用方修改它不会改变 Session 内部保存的快照。
// 对于创建失败后因自动清理失败而返回的 Session，该方法返回 nil。
func (s *Session) Capabilities() Capabilities {
	if s == nil || s.capabilities == nil {
		return nil
	}

	return cloneCapabilities(s.capabilities)
}

// AutomationName 返回当前 Session 使用的 Appium Driver automationName。
//
// 返回值来自创建 Session 后远端确认的 Capabilities，
// 不使用创建请求中的原始值，也不会进行大小写规范化。
//
// 对于创建失败后仅用于清理的不可用 Session，返回空字符串。
func (s *Session) AutomationName() string {
	if s == nil {
		return ""
	}

	return s.automationName
}

// WindowRect 获取当前 Session 的 WebDriver Window Rect。
//
// 返回的是 WebDriver viewport/window 坐标语义，
// 不保证与截图像素坐标一一对应。
func (s *Session) WindowRect(
	ctx context.Context,
) (Rect, error) {
	client, err := s.commandClient(
		getWindowRectOperation,
	)
	if err != nil {
		return Rect{}, err
	}

	command, err := wire.NewCommand(
		getWindowRectOperation,
		http.MethodGet,
		"session",
		s.id,
		"window",
		"rect",
	)
	if err != nil {
		return Rect{}, &Error{
			Code:      CodeInvalidConfig,
			Operation: getWindowRectOperation,
			Message:   "get window rect command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	var rect Rect

	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			decoded, decodeErr := decodeRect(
				ctx,
				value,
			)
			if decodeErr != nil {
				return decodeErr
			}

			rect = decoded
			return nil
		},
	)
	if err != nil {
		return Rect{}, err
	}

	return rect, nil
}

// Screenshot 获取当前 Session 的屏幕截图。
//
// 返回值为解码后的 PNG 字节数据。
// 客户端不会对截图尺寸或像素坐标语义进行额外转换。
func (s *Session) Screenshot(
	ctx context.Context,
) ([]byte, error) {
	client, err := s.commandClient(
		screenshotOperation,
	)
	if err != nil {
		return nil, err
	}

	command, err := wire.NewCommand(
		screenshotOperation,
		http.MethodGet,
		"session",
		s.id,
		"screenshot",
	)
	if err != nil {
		return nil, &Error{
			Code:      CodeInvalidConfig,
			Operation: screenshotOperation,
			Message:   "screenshot command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	var screenshot []byte

	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
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

			decoded, decodeErr := codec.DecodeBase64(
				ctx,
				encoded,
				client.limits.MaxResponseBytes,
			)
			if decodeErr != nil {
				return decodeErr
			}

			screenshot = decoded
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return screenshot, nil
}

// PageSource 获取当前 Session 的页面源。
//
// Page Source 可能是较大的 XML 或 HTML 字符串，
// 因此使用独立的 MaxPageSourceResponseBytes 响应上限。
func (s *Session) PageSource(
	ctx context.Context,
) (string, error) {
	client, err := s.commandClient(
		pageSourceOperation,
	)
	if err != nil {
		return "", err
	}

	command, err := wire.NewCommand(
		pageSourceOperation,
		http.MethodGet,
		"session",
		s.id,
		"source",
	)
	if err != nil {
		return "", &Error{
			Code:      CodeInvalidConfig,
			Operation: pageSourceOperation,
			Message:   "page source command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	var source string

	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxPageSourceResponseBytes,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			decoded, decodeErr := codec.DecodeJSONString(
				ctx,
				value,
			)
			if decodeErr != nil {
				return decodeErr
			}

			source = decoded
			return nil
		},
	)
	if err != nil {
		return "", err
	}

	return source, nil
}

// Close 删除远端 WebDriver Session.
//
// Close 可以重复调用。
// 本地已经确认关闭后，后续调用直接返回 nil.
//
// 如果远端明确返回 invalid session id，Session 会被标记为已经关闭，
// 同时本次调用仍返回 CodeSessionLost，以保留“远端 Session 已不存在”这一事实。
//
// 传输失败且无法确认删除结果时，Session 不会被标记为关闭。
func (s *Session) Close(ctx context.Context) error {
	if s == nil ||
		s.client == nil ||
		s.state == nil ||
		s.id == "" {
		return &Error{
			Code:      CodeInvalidArgument,
			Operation: deleteSessionOperation,
			Message:   "session is not initialized",
			Delivery:  DeliveryNotSent,
		}
	}

	s.state.closeMu.Lock()
	defer s.state.closeMu.Unlock()

	if s.state.closed.Load() {
		return nil
	}

	err := s.client.deleteSession(
		ctx,
		s.id,
		s.client.commandTimeout,
		deleteSessionOperation,
	)
	if err == nil {
		s.state.closed.Store(true)
		return nil
	}

	if IsErrorCode(err, CodeSessionLost) &&
		DeliveryOf(err) == DeliveryAcknowledged {
		s.state.closed.Store(true)
	}

	return err
}

// commandClient 校验 Session 是否允许执行普通远端命令。
//
// 该方法只校验本地能够确认的 Session 状态。
// 它不会主动探测远端 Session 是否仍然存在。
func (s *Session) commandClient(
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

// newSession 创建本地 Session 句柄.
func newSession(
	client *Client,
	sessionID string,
	capabilities Capabilities,
	automationName string,
	usable bool,
) *Session {
	return &Session{
		client:         client,
		id:             sessionID,
		capabilities:   cloneCapabilities(capabilities),
		automationName: automationName,
		usable:         usable,
		state:          &sessionState{},
	}
}

// cleanupSession 对创建过程中未能正常交付给调用方的 Session 执行尽力清理.
func (c *Client) cleanupSession(sessionID string) error {
	err := c.deleteSession(
		context.Background(),
		sessionID,
		c.sessionCleanupTimeout,
		cleanupSessionOperation,
	)
	if err == nil {
		return nil
	}

	// invalid session id 表示远端已经不存在该 Session，
	// 从清理目标来看已经达到预期状态。
	if IsErrorCode(err, CodeSessionLost) &&
		DeliveryOf(err) == DeliveryAcknowledged {
		return nil
	}

	return err
}

// deleteSession 删除指定远端 Session.
func (c *Client) deleteSession(
	ctx context.Context,
	sessionID string,
	timeout time.Duration,
	operation string,
) error {
	command, err := wire.NewCommand(
		operation,
		http.MethodDelete,
		"session",
		sessionID,
	)
	if err != nil {
		return &Error{
			Code:      CodeInvalidConfig,
			Operation: operation,
			Message:   "delete session command definition is invalid",
			Delivery:  DeliveryNotSent,
			Cause:     err,
		}
	}

	return c.executeCommand(
		ctx,
		command,
		nil,
		timeout,
		c.limits.MaxResponseBytes,
		decodeNullResponse,
	)
}

// decodeCreateSessionResponse 校验并解码创建 Session 的响应.
//
// Session ID 优先解析并写入结果。
// 一旦成功获得 Session ID，即使后续 context 结束或 Capabilities 无效，
// 调用方仍然能够据此执行 Session 清理。
func decodeCreateSessionResponse(
	ctx context.Context,
	value json.RawMessage,
) (createSessionResult, error) {
	var result createSessionResult

	var payload struct {
		SessionID    json.RawMessage `json:"sessionId"`
		Capabilities json.RawMessage `json:"capabilities"`
	}

	if err := json.Unmarshal(value, &payload); err != nil {
		return result, fmt.Errorf(
			"decode create session response: %w",
			err,
		)
	}

	if len(payload.SessionID) == 0 {
		return result, errors.New(
			"create session response does not contain sessionId",
		)
	}

	// Session ID 是清理远端 Session 所需的物理句柄。
	// 即使调用方 context 此刻已经结束，也优先完成这一小段严格解码。
	sessionID, err := codec.DecodeJSONString(
		context.Background(),
		payload.SessionID,
	)
	if err != nil {
		return result, fmt.Errorf(
			"decode create session sessionId: %w",
			err,
		)
	}
	if sessionID == "" {
		return result, errors.New(
			"create session response contains empty sessionId",
		)
	}

	result.SessionID = sessionID

	if err := ctx.Err(); err != nil {
		return result, err
	}

	if len(payload.Capabilities) == 0 {
		return result, errors.New(
			"create session response does not contain capabilities",
		)
	}

	if err := codec.ValidateUTF8(
		ctx,
		payload.Capabilities,
	); err != nil {
		return result, err
	}

	var capabilities Capabilities
	if err := json.Unmarshal(
		payload.Capabilities,
		&capabilities,
	); err != nil {
		return result, fmt.Errorf(
			"decode create session capabilities: %w",
			err,
		)
	}
	if capabilities == nil {
		return result, errors.New(
			"create session capabilities must be a JSON object",
		)
	}

	automationName, err := decodeAutomationName(capabilities)
	if err != nil {
		return result, err
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}

	result.Capabilities = capabilities
	result.AutomationName = automationName

	return result, nil
}

// decodeNullResponse 校验命令响应 value 是否为 JSON null.
func decodeNullResponse(
	ctx context.Context,
	value json.RawMessage,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if !bytes.Equal(
		bytes.TrimSpace(value),
		[]byte("null"),
	) {
		return errors.New(
			"WebDriver response value must be null",
		)
	}

	return nil
}

// cloneCapabilities 深拷贝 Capability 快照.
func cloneCapabilities(
	source Capabilities,
) Capabilities {
	if source == nil {
		return nil
	}

	cloned := make(Capabilities, len(source))

	for key, value := range source {
		cloned[key] = cloneCapabilityValue(value)
	}

	return cloned
}

// cloneCapabilityValue 深拷贝 JSON Capability 值.
func cloneCapabilityValue(value any) any {
	switch current := value.(type) {
	case map[string]any:
		cloned := make(
			map[string]any,
			len(current),
		)

		for key, child := range current {
			cloned[key] = cloneCapabilityValue(child)
		}

		return cloned

	case []any:
		cloned := make([]any, len(current))

		for index, child := range current {
			cloned[index] = cloneCapabilityValue(child)
		}

		return cloned

	default:
		return current
	}
}

// decodeAutomationName 从 Appium 处理后的 Session Capabilities 中
// 严格读取当前 Session 使用的 Driver automationName。
func decodeAutomationName(
	capabilities Capabilities,
) (string, error) {
	value, exists := capabilities["automationName"]
	if !exists {
		return "", errors.New(
			"create session capabilities do not contain automationName",
		)
	}

	automationName, ok := value.(string)
	if !ok {
		return "", errors.New(
			"create session automationName must be a string",
		)
	}

	if automationName == "" {
		return "", errors.New(
			"create session automationName must not be empty",
		)
	}

	return automationName, nil
}
