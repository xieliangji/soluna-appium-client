package contracttest

import (
	"net/http"
	"net/http/httptest"
	"sync"

	appium "github.com/xieliangji/soluna-appium-client"
)

// Server 表示用于 Appium 协议测试的本地 HTTP Server。
//
// Server 只负责测试 Server 的启动、地址访问和生命周期管理。
// 请求记录、协议匹配和响应行为由独立组件负责。
type Server struct {
	server    *httptest.Server
	closeOnce sync.Once
}

// NewServer 启动一个新的本地协议测试 Server。
//
// handler 为 nil 时使用固定的 404 Handler，
// 不会回退到全局 http.DefaultServeMux，避免测试之间产生隐式耦合。
func NewServer(handler http.Handler) *Server {
	if handler == nil {
		handler = http.NotFoundHandler()
	}

	return &Server{
		server: httptest.NewServer(handler),
	}
}

// URL 返回测试 Server 的根地址。
//
// 未初始化的 Server 返回空字符串。
func (s *Server) URL() string {
	if s == nil || s.server == nil {
		return ""
	}

	return s.server.URL
}

// NewClient 创建一个指向当前测试 Server 的 Appium Client。
//
// options 原样传递给根包 NewClient，
// contracttest 不修改调用方提供的超时、资源限制或 HTTP Client。
func (s *Server) NewClient(
	options appium.ClientOptions,
) (*appium.Client, error) {
	return appium.NewClient(
		s.URL(),
		options,
	)
}

// Close 关闭测试 Server。
//
// Close 可以重复调用。
func (s *Server) Close() {
	if s == nil || s.server == nil {
		return
	}

	s.closeOnce.Do(
		s.server.Close,
	)
}
