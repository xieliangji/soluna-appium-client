package wire

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Endpoint 表示一个固定的 Appium Server 基础地址。
//
// Endpoint 只负责基础地址校验和命令路径构造，
// 不负责 HTTP 请求发送或命令语义处理。
type Endpoint struct {
	base url.URL
}

// NewEndpoint 创建并校验 Appium Server Endpoint。
//
// Endpoint 必须是绝对的 HTTP 或 HTTPS URL。
// 基础地址可以包含路径，例如 http://127.0.0.1:4723/wd/hub，
// 但不能包含用户信息、查询参数或 Fragment。
func NewEndpoint(rawURL string) (*Endpoint, error) {
	if rawURL == "" {
		return nil, errors.New("endpoint URL is empty")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint URL: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, errors.New("endpoint URL scheme must be http or https")
	}
	if parsed.Host == "" {
		return nil, errors.New("endpoint URL must contain a host")
	}
	if parsed.User != nil {
		return nil, errors.New("endpoint URL must not contain user information")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery {
		return nil, errors.New("endpoint URL must not contain a query")
	}
	if parsed.Fragment != "" {
		return nil, errors.New("endpoint URL must not contain a fragment")
	}

	parsed.Scheme = scheme

	return &Endpoint{base: *parsed}, nil
}

// URL 根据路径段构造完整的命令 URL。
//
// 每个 segment 都会作为独立路径段进行转义。
// 调用方必须传入未转义的原始值，不能预先调用 url.PathEscape。
//
// 例如：
//
//	endpoint.URL("session", sessionID, "element", elementID, "rect")
//
// sessionID 或 elementID 中即使包含 "/"，也只会作为当前路径段的内容，
// 不会被解释为新的路径层级。
func (e *Endpoint) URL(segments ...string) (*url.URL, error) {
	if e == nil {
		return nil, errors.New("endpoint is nil")
	}

	var escapedPath strings.Builder
	escapedPath.WriteString(strings.TrimRight(e.base.EscapedPath(), "/"))

	for _, segment := range segments {
		escapedPath.WriteString("/" + url.PathEscape(segment))
	}

	decodedPath, err := url.PathUnescape(escapedPath.String())
	if err != nil {
		return nil, fmt.Errorf("decode endpoint path: %w", err)
	}

	result := e.base
	result.Path = decodedPath
	result.RawPath = ""

	// 只有在转义路径与解码路径不同时才设置 RawPath。
	// 这样可以让 net/url 在普通路径下使用自身的标准编码行为，
	// 同时保留动态路径段中的转义边界。
	if escapedPath.String() != decodedPath {
		result.RawPath = escapedPath.String()
	}

	return &result, nil
}
