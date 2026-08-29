package contracttest

import (
	"bytes"
	"io"
	"net/http"
	"sync"
)

// RecordedRequest 表示 Contract Test Server 实际收到的一次 HTTP 请求快照。
//
// 记录的是请求到达 Server 时的事实，不对 Header、Path 或 Body 做规范化。
type RecordedRequest struct {
	Method     string
	RequestURI string
	Path       string
	RawQuery   string
	Header     http.Header
	Body       []byte
}

// Recorder 记录经过它的 HTTP 请求，并将请求继续交给下游 Handler。
//
// Recorder 可以被并发请求安全使用。
// Requests 返回的是独立快照，调用方修改返回值不会影响 Recorder 内部状态。
type Recorder struct {
	next http.Handler

	mu       sync.Mutex
	requests []RecordedRequest
}

// NewRecorder 创建请求记录器。
//
// next 为 nil 时使用固定的 404 Handler，避免隐式使用全局 DefaultServeMux。
func NewRecorder(next http.Handler) *Recorder {
	if next == nil {
		next = http.NotFoundHandler()
	}

	return &Recorder{
		next: next,
	}
}

// ServeHTTP 记录请求后将其继续交给下游 Handler。
//
// 请求 Body 会完整读取并重新构造，
// 因此下游 Handler 仍然能够读取与原请求相同的 Body 内容。
func (r *Recorder) ServeHTTP(
	writer http.ResponseWriter,
	request *http.Request,
) {
	body, err := readAndRestoreRequestBody(request)
	if err != nil {
		http.Error(
			writer,
			"failed to read request body",
			http.StatusInternalServerError,
		)
		return
	}

	recorded := RecordedRequest{
		Method:     request.Method,
		RequestURI: request.RequestURI,
		Path:       request.URL.Path,
		RawQuery:   request.URL.RawQuery,
		Header:     request.Header.Clone(),
		Body:       append([]byte(nil), body...),
	}

	r.mu.Lock()
	r.requests = append(
		r.requests,
		recorded,
	)
	r.mu.Unlock()

	r.next.ServeHTTP(
		writer,
		request,
	)
}

// Requests 返回当前已经记录的全部请求快照。
//
// 请求顺序与 Recorder 完成请求捕获的顺序一致。
// 返回值及其中的 Header、Body 都与内部状态相互独立。
func (r *Recorder) Requests() []RecordedRequest {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	requests := make(
		[]RecordedRequest,
		len(r.requests),
	)

	for index, request := range r.requests {
		requests[index] = cloneRecordedRequest(
			request,
		)
	}

	return requests
}

// Reset 清除当前已经记录的全部请求。
//
// Reset 不影响正在执行或后续到达的请求。
func (r *Recorder) Reset() {
	if r == nil {
		return
	}

	r.mu.Lock()
	r.requests = nil
	r.mu.Unlock()
}

// readAndRestoreRequestBody 读取请求 Body，并恢复供下游 Handler 再次读取。
func readAndRestoreRequestBody(
	request *http.Request,
) ([]byte, error) {
	if request == nil ||
		request.Body == nil ||
		request.Body == http.NoBody {
		return nil, nil
	}

	original := request.Body

	body, err := io.ReadAll(original)
	closeErr := original.Close()

	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}

	request.Body = io.NopCloser(
		bytes.NewReader(body),
	)

	return body, nil
}

// cloneRecordedRequest 深拷贝一个请求快照。
func cloneRecordedRequest(
	request RecordedRequest,
) RecordedRequest {
	cloned := request

	if request.Header != nil {
		cloned.Header = request.Header.Clone()
	}

	if request.Body != nil {
		cloned.Body = append(
			[]byte(nil),
			request.Body...,
		)
	}

	return cloned
}
