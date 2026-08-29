package wire

import (
	"errors"
	"strings"
)

// Command 表示一条确定的 WebDriver/Appium 远端命令。
//
// Command 只描述命令名称、HTTP Method 和路径段。
// 请求体、响应上限、超时等执行参数由调用命令时单独提供。
type Command struct {
	operation string
	method    string
	segments  []string
}

// NewCommand 创建一条远端命令定义。
//
// operation 是客户端内部使用的稳定命令名称，应保持低基数，
// 例如 new_session、find_element、screenshot。
//
// method 必须是非空的 HTTP Method。
// segments 必须传入未转义的原始路径段。
func NewCommand(operation, method string, segments ...string) (Command, error) {
	operation = strings.TrimSpace(operation)
	if operation == "" {
		return Command{}, errors.New("command operation is empty")
	}

	method = strings.TrimSpace(method)
	if method == "" {
		return Command{}, errors.New("command HTTP method is empty")
	}

	return Command{
		operation: operation,
		method:    method,
		segments:  append([]string(nil), segments...),
	}, nil
}

// Operation 返回命令的稳定操作名称。
func (c Command) Operation() string {
	return c.operation
}

// Method 返回命令使用的 HTTP Method。
func (c Command) Method() string {
	return c.method
}

// URL 使用指定 Endpoint 构造命令的完整 URL。
func (c Command) URL(endpoint *Endpoint) (string, error) {
	if endpoint == nil {
		return "", errors.New("endpoint is nil")
	}

	target, err := endpoint.URL(c.segments...)
	if err != nil {
		return "", err
	}

	return target.String(), nil
}
