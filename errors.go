package appium

import (
	"encoding/json"
	"errors"
)

// ErrorCode 表示客户端错误或远端命令失败的错误类别。
type ErrorCode string

const (
	CodeInvalidConfig    ErrorCode = "invalid_config"     // 客户端配置无效
	CodeInvalidArgument  ErrorCode = "invalid_argument"   // 调用参数无效
	CodeCanceled         ErrorCode = "canceled"           // 操作被调用方取消
	CodeDeadlineExceeded ErrorCode = "deadline_exceeded"  // 操作超过截止时间
	CodeTransportFailed  ErrorCode = "transport_failed"   // HTTP 或网络传输失败
	CodeResponseInvalid  ErrorCode = "response_invalid"   // 远端响应格式无效
	CodeResponseTooLarge ErrorCode = "response_too_large" // 远端响应超过允许上限
	CodeOutputFailed     ErrorCode = "output_failed"      // 本地输出交付失败
	CodeCommandFailed    ErrorCode = "command_failed"     // 远端命令执行失败
	CodeSessionLost      ErrorCode = "session_lost"       // WebDriver Session 已丢失或失效
	CodeElementNotFound  ErrorCode = "element_not_found"  // 未找到目标元素
	CodeAlertNotFound    ErrorCode = "alert_not_found"    // 当前没有可操作的 Alert
	CodeElementStale     ErrorCode = "element_stale"      // 元素引用已经失效
	CodeUnsupported      ErrorCode = "unsupported"        // 当前命令或能力不受支持
)

// DeliveryState 表示客户端能够确认的命令投递状态。
//
// 投递状态不表示命令是否适合重试。
type DeliveryState string

const (
	// DeliveryUnknown 表示客户端无法确认远端是否已经收到或执行命令。
	DeliveryUnknown DeliveryState = "unknown"

	// DeliveryNotSent 表示客户端可以确认命令尚未发送到远端。
	DeliveryNotSent DeliveryState = "not_sent"

	// DeliveryAcknowledged 表示客户端已经收到远端响应。
	DeliveryAcknowledged DeliveryState = "acknowledged"
)

// Error 表示客户端公开返回的结构化错误。
type Error struct {
	Code       ErrorCode
	Operation  string
	Message    string
	StatusCode int
	RemoteCode string
	Delivery   DeliveryState
	RemoteData json.RawMessage
	Cause      error
}

// Error 返回错误文本。
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Operation != "" && e.Message != "" {
		return e.Operation + ": " + e.Message
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Operation != "" {
		return e.Operation + ": " + string(e.Code)
	}
	return string(e.Code)
}

// Unwrap 返回底层原始错误。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// IsErrorCode 判断错误链或 errors.Join 错误树中是否包含指定错误码。
func IsErrorCode(err error, code ErrorCode) bool {
	if err == nil {
		return false
	}

	var target *Error
	if errors.As(err, &target) && target != nil && target.Code == code {
		return true
	}

	// errors.As 返回错误树中第一个匹配的 *Error；当它的 Code 不匹配时，
	// 仍需显式遍历其他分支，才能让 errors.Join 同时暴露主错误和诊断错误。
	switch unwrapped := err.(type) {
	case interface{ Unwrap() []error }:
		for _, child := range unwrapped.Unwrap() {
			if IsErrorCode(child, code) {
				return true
			}
		}
	case interface{ Unwrap() error }:
		return IsErrorCode(unwrapped.Unwrap(), code)
	}

	return false
}

// DeliveryOf 返回错误中记录的命令投递状态。
//
// 对包含多个客户端 Error 的错误树，返回 errors.As 遍历到的第一个
// Error 的状态；调用方可使用 IsErrorCode 判断其他分支中的错误码。
// 如果 err 不是客户端定义的 Error，则返回 DeliveryUnknown。
func DeliveryOf(err error) DeliveryState {
	var target *Error
	if !errors.As(err, &target) || target == nil {
		return DeliveryUnknown
	}
	return target.Delivery
}
