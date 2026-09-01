package appium_test

import (
	"context"
	"math"
	"net/http"
	"sync"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestObserverLifecycleIncludesResponseDecodeFailure(t *testing.T) {
	const responseBody = `{"value":{"ready":true,"message":"ready","build":{}}}`

	observer := &commandObserverRecorder{}

	server := contracttest.NewServer(
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.Header().Set(
					"Content-Type",
					"application/json",
				)

				_, _ = writer.Write(
					[]byte(responseBody),
				)
			},
		),
	)
	defer server.Close()

	client, err := server.NewClient(
		appium.ClientOptions{
			Observer: observer,
		},
	)
	if err != nil {
		t.Fatalf(
			"create client: %v",
			err,
		)
	}

	_, err = client.Status(
		context.Background(),
	)
	if err == nil {
		t.Fatal(
			"expected invalid status response error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeResponseInvalid,
	) {
		t.Fatalf(
			"unexpected error code: %v",
			err,
		)
	}

	started, finished := observer.snapshot()

	if len(started) != 1 {
		t.Fatalf(
			"unexpected started event count: expected 1, got %d",
			len(started),
		)
	}

	if len(finished) != 1 {
		t.Fatalf(
			"unexpected finished event count: expected 1, got %d",
			len(finished),
		)
	}

	startEvent := started[0]

	if startEvent.Operation != "get_status" {
		t.Fatalf(
			"unexpected started operation: %q",
			startEvent.Operation,
		)
	}

	if startEvent.StartedAt.IsZero() {
		t.Fatal(
			"started event must contain start time",
		)
	}

	if startEvent.RequestBytes != 0 {
		t.Fatalf(
			"unexpected started request bytes: %d",
			startEvent.RequestBytes,
		)
	}

	finishEvent := finished[0]

	if finishEvent.Operation != startEvent.Operation {
		t.Fatalf(
			"operation mismatch: started %q, finished %q",
			startEvent.Operation,
			finishEvent.Operation,
		)
	}

	if finishEvent.StatusCode != http.StatusOK {
		t.Fatalf(
			"unexpected finished HTTP status: expected %d, got %d",
			http.StatusOK,
			finishEvent.StatusCode,
		)
	}

	if finishEvent.RequestBytes != 0 {
		t.Fatalf(
			"unexpected finished request bytes: %d",
			finishEvent.RequestBytes,
		)
	}

	if finishEvent.ResponseBytes != int64(len(responseBody)) {
		t.Fatalf(
			"unexpected finished response bytes: expected %d, got %d",
			len(responseBody),
			finishEvent.ResponseBytes,
		)
	}

	if finishEvent.ErrorCode != appium.CodeResponseInvalid {
		t.Fatalf(
			"unexpected finished error code: %q",
			finishEvent.ErrorCode,
		)
	}

	if finishEvent.Delivery != appium.DeliveryAcknowledged {
		t.Fatalf(
			"unexpected finished delivery state: expected %q, got %q",
			appium.DeliveryAcknowledged,
			finishEvent.Delivery,
		)
	}

	if finishEvent.Duration < 0 {
		t.Fatalf(
			"finished duration must not be negative: %v",
			finishEvent.Duration,
		)
	}
}

func TestObserverUsesExecuteScriptOperationIdentity(t *testing.T) {
	observer := &commandObserverRecorder{}
	recorder := contracttest.NewRecorder(
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.Header().Set(
					"Content-Type",
					"application/json",
				)

				switch request.RequestURI {
				case "/session":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
						),
					)

				case "/session/session/execute/sync":
					_, _ = writer.Write([]byte(`{"value":null}`))

				default:
					http.NotFound(writer, request)
				}
			},
		),
	)
	server := contracttest.NewServer(recorder)
	defer server.Close()

	client, err := server.NewClient(
		appium.ClientOptions{
			Observer: observer,
		},
	)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	session, err := client.CreateSession(
		context.Background(),
		appium.MatchCapabilities(
			appium.Capabilities{
				"platformName":          "iOS",
				"appium:automationName": "XCUITest",
			},
		),
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	observer.reset()
	recorder.Reset()

	if _, err := session.ExecuteScriptWithOperation(
		context.Background(),
		"ios_press_button",
		"mobile: pressButton",
		nil,
	); err != nil {
		t.Fatalf("execute script: %v", err)
	}

	started, finished := observer.snapshot()
	if len(started) != 1 || len(finished) != 1 {
		t.Fatalf(
			"unexpected observer event counts: started=%d finished=%d",
			len(started),
			len(finished),
		)
	}

	if started[0].Operation != "ios_press_button" {
		t.Fatalf("unexpected started operation: %q", started[0].Operation)
	}
	if finished[0].Operation != "ios_press_button" {
		t.Fatalf("unexpected finished operation: %q", finished[0].Operation)
	}
}

func TestObserverNotCalledForLocalFailures(t *testing.T) {
	observer := &commandObserverRecorder{}

	recorder := contracttest.NewRecorder(
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.Header().Set(
					"Content-Type",
					"application/json",
				)

				if request.Method == http.MethodPost &&
					request.RequestURI == "/session" {
					_, _ = writer.Write(
						[]byte(
							`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
						),
					)
					return
				}

				http.NotFound(
					writer,
					request,
				)
			},
		),
	)

	server := contracttest.NewServer(recorder)
	defer server.Close()

	client, err := server.NewClient(
		appium.ClientOptions{
			Observer: observer,
		},
	)
	if err != nil {
		t.Fatalf(
			"create client: %v",
			err,
		)
	}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err = client.Status(ctx)
	if err == nil {
		t.Fatal(
			"expected canceled status error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeCanceled,
	) {
		t.Fatalf(
			"unexpected canceled status error: %v",
			err,
		)
	}

	assertNoObserverEvents(
		t,
		observer,
		"pre-canceled context",
	)

	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf(
			"pre-canceled command must not be delivered: got %d requests",
			len(requests),
		)
	}

	session, err := client.CreateSession(
		context.Background(),
		appium.MatchCapabilities(
			appium.Capabilities{
				"platformName":          "iOS",
				"appium:automationName": "XCUITest",
			},
		),
	)
	if err != nil {
		t.Fatalf(
			"create session: %v",
			err,
		)
	}

	observer.reset()
	recorder.Reset()

	_, err = session.Find(
		context.Background(),
		appium.Locator{
			Value: "login",
		},
	)
	if err == nil {
		t.Fatal(
			"expected empty locator strategy error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeInvalidArgument,
	) {
		t.Fatalf(
			"unexpected locator error: %v",
			err,
		)
	}

	assertNoObserverEvents(
		t,
		observer,
		"local argument validation",
	)

	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf(
			"invalid locator must not be delivered: got %d requests",
			len(requests),
		)
	}

	_, err = session.ExecuteScript(
		context.Background(),
		"return arguments[0]",
		[]any{
			math.Inf(1),
		},
	)
	if err == nil {
		t.Fatal(
			"expected JSON encoding error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeInvalidArgument,
	) {
		t.Fatalf(
			"unexpected script encoding error: %v",
			err,
		)
	}

	assertNoObserverEvents(
		t,
		observer,
		"request JSON encoding",
	)

	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf(
			"unencodable request must not be delivered: got %d requests",
			len(requests),
		)
	}
}

// commandObserverRecorder 保存 Observer 回调产生的事件。
type commandObserverRecorder struct {
	mu sync.Mutex

	started  []appium.CommandStartedEvent
	finished []appium.CommandFinishedEvent
}

func (o *commandObserverRecorder) OnCommandStarted(
	event appium.CommandStartedEvent,
) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.started = append(
		o.started,
		event,
	)
}

func (o *commandObserverRecorder) OnCommandFinished(
	event appium.CommandFinishedEvent,
) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.finished = append(
		o.finished,
		event,
	)
}

// reset 清除已经记录的 Observer 事件。
func (o *commandObserverRecorder) reset() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.started = nil
	o.finished = nil
}

// snapshot 返回当前 Observer 事件快照。
func (o *commandObserverRecorder) snapshot() (
	[]appium.CommandStartedEvent,
	[]appium.CommandFinishedEvent,
) {
	o.mu.Lock()
	defer o.mu.Unlock()

	started := append(
		[]appium.CommandStartedEvent(nil),
		o.started...,
	)

	finished := append(
		[]appium.CommandFinishedEvent(nil),
		o.finished...,
	)

	return started, finished
}

// assertNoObserverEvents 断言指定阶段没有产生远端命令事件。
func assertNoObserverEvents(
	t *testing.T,
	observer *commandObserverRecorder,
	stage string,
) {
	t.Helper()

	started, finished := observer.snapshot()

	if len(started) != 0 ||
		len(finished) != 0 {
		t.Fatalf(
			"%s must not produce observer events: started=%d finished=%d",
			stage,
			len(started),
			len(finished),
		)
	}
}
