package appium_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestLocalDP081PullLogsProtocolAndSnapshotSemantics(t *testing.T) {
	logReads := 0
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.RequestURI == "/session":
			_, _ = writer.Write([]byte(`{"value":{"sessionId":"session/id","capabilities":{"automationName":"XCUITest"}}}`))
		case request.Method == http.MethodGet && request.RequestURI == "/session/session%2Fid/se/log/types":
			_, _ = writer.Write([]byte(`{"value":["browser","","browser"]}`))
		case request.Method == http.MethodPost && request.RequestURI == "/session/session%2Fid/se/log":
			logReads++
			if logReads == 1 {
				_, _ = writer.Write([]byte(`{"value":[{"timestamp":-9223372036854775808,"level":"INFO","message":"first","nested":{"count":9007199254740993},"items":[1,{"ok":true}],"nullable":null},{"timestamp":0,"level":"","message":""}]}`))
				return
			}
			_, _ = writer.Write([]byte(`{"value":[]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	server := contracttest.NewServer(recorder)
	defer server.Close()

	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"platformName": "iOS", "appium:automationName": "XCUITest",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	logTypes, err := session.LogTypes(context.Background())
	if err != nil {
		t.Fatalf("read log types: %v", err)
	}
	if expected := []appium.LogType{"browser", "", "browser"}; !reflect.DeepEqual(logTypes, expected) {
		t.Fatalf("unexpected log types: expected %#v, got %#v", expected, logTypes)
	}

	entries, err := session.Logs(context.Background(), appium.LogType("  custom/type  "))
	if err != nil {
		t.Fatalf("read logs: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("unexpected log entry count: %d", len(entries))
	}
	if entries[0].Timestamp != -9223372036854775808 ||
		entries[0].Level != "INFO" ||
		entries[0].Message != "first" {
		t.Fatalf("unexpected first log entry: %#v", entries[0])
	}
	nested, ok := entries[0].Extra["nested"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected nested extra type: %T", entries[0].Extra["nested"])
	}
	if number, ok := nested["count"].(json.Number); !ok || number.String() != "9007199254740993" {
		t.Fatalf("unknown number was not preserved: %#v (%T)", nested["count"], nested["count"])
	}
	if entries[1].Extra != nil {
		t.Fatalf("entry without unknown fields must have nil Extra: %#v", entries[1].Extra)
	}

	// Extra 及嵌套容器属于本次快照的独立副本；后续读取也不使用缓存。
	nested["count"] = json.Number("1")
	second, err := session.Logs(context.Background(), "")
	if err != nil {
		t.Fatalf("read logs second time: %v", err)
	}
	if second == nil || len(second) != 0 {
		t.Fatalf("expected a non-nil empty second snapshot, got %#v", second)
	}

	requests := recorder.Requests()
	if len(requests) != 4 {
		t.Fatalf("unexpected request count: %d", len(requests))
	}
	getRequest := requests[1]
	if err := contracttest.MatchMethod(getRequest, http.MethodGet); err != nil {
		t.Fatal(err)
	}
	if err := contracttest.MatchRequestURI(getRequest, "/session/session%2Fid/se/log/types"); err != nil {
		t.Fatal(err)
	}
	if err := contracttest.MatchBody(getRequest, nil); err != nil {
		t.Fatal(err)
	}
	if values := getRequest.Header.Values("Content-Type"); len(values) != 0 {
		t.Fatalf("GET LogTypes must not send Content-Type: %v", values)
	}

	for _, request := range requests[2:] {
		if err := contracttest.MatchMethod(request, http.MethodPost); err != nil {
			t.Fatal(err)
		}
		if err := contracttest.MatchRequestURI(request, "/session/session%2Fid/se/log"); err != nil {
			t.Fatal(err)
		}
		if err := contracttest.MatchHeader(request, "Content-Type", "application/json"); err != nil {
			t.Fatal(err)
		}
	}
	if err := contracttest.MatchJSONBody(requests[2], map[string]any{"type": "  custom/type  "}); err != nil {
		t.Fatal(err)
	}
	if err := contracttest.MatchJSONBody(requests[3], map[string]any{"type": ""}); err != nil {
		t.Fatal(err)
	}
}

func TestLocalDP081PullLogsRejectsInvalidResponsesWithoutPartialResults(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{name: "top-level null", response: `{"value":null}`},
		{name: "top-level object", response: `{"value":{}}`},
		{name: "top-level string", response: `{"value":"logs"}`},
		{name: "log type is not string", response: `{"value":["ok",null]}`},
		{name: "entry is null", response: `{"value":[null]}`},
		{name: "missing timestamp", response: `{"value":[{"level":"INFO","message":"m"}]}`},
		{name: "timestamp null", response: `{"value":[{"timestamp":null,"level":"INFO","message":"m"}]}`},
		{name: "timestamp fractional", response: `{"value":[{"timestamp":1.5,"level":"INFO","message":"m"}]}`},
		{name: "timestamp exponent", response: `{"value":[{"timestamp":1e3,"level":"INFO","message":"m"}]}`},
		{name: "timestamp overflow", response: `{"value":[{"timestamp":9223372036854775808,"level":"INFO","message":"m"}]}`},
		{name: "level wrong type", response: `{"value":[{"timestamp":1,"level":1,"message":"m"}]}`},
		{name: "message missing", response: `{"value":[{"timestamp":1,"level":"INFO"}]}`},
		{name: "unknown field malformed", response: `{"value":[{"timestamp":1,"level":"INFO","message":"m","extra":}]}`},
		{name: "unpaired surrogate", response: `{"value":[{"timestamp":1,"level":"INFO","message":"m","extra":"\ud800"}]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session, _ := newLocalDP081LogsTestSession(t, test.response, appium.ClientOptions{})
			if test.name == "log type is not string" {
				_, err := session.LogTypes(context.Background())
				assertLocalDP081ResponseInvalid(t, err)
				return
			}

			_, err := session.Logs(context.Background(), "type")
			assertLocalDP081ResponseInvalid(t, err)
		})
	}

	// 第二个条目非法时，第一个已解码条目也不能作为部分结果返回。
	session, _ := newLocalDP081LogsTestSession(t, `{"value":[{"timestamp":1,"level":"INFO","message":"ok"},{"timestamp":2,"level":"INFO"}]}`, appium.ClientOptions{})
	entries, err := session.Logs(context.Background(), "type")
	if entries != nil {
		t.Fatalf("invalid batch must not return partial entries: %#v", entries)
	}
	assertLocalDP081ResponseInvalid(t, err)
}

func TestLocalDP081PullLogsResponseLimitAndConfiguration(t *testing.T) {
	if defaults := appium.DefaultLimits(); defaults.MaxLogResponseBytes != 32<<20 {
		t.Fatalf("unexpected default log response limit: %d", defaults.MaxLogResponseBytes)
	}

	client, err := appium.NewClient("http://127.0.0.1:4723", appium.ClientOptions{
		Limits: appium.Limits{MaxLogResponseBytes: -1},
	})
	if err == nil || !appium.IsErrorCode(err, appium.CodeInvalidConfig) {
		t.Fatalf("expected negative log limit configuration error, got %v", err)
	}
	if client != nil || appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("invalid log limit must not create a client: client=%v error=%v", client, err)
	}

	response := `{"value":[{"timestamp":1,"level":"INFO","message":"m"}]}`
	exactSession, _ := newLocalDP081LogsTestSession(t, response, appium.ClientOptions{
		Limits: appium.Limits{MaxLogResponseBytes: int64(len(response))},
	})
	exactEntries, err := exactSession.Logs(context.Background(), "type")
	if err != nil || len(exactEntries) != 1 {
		t.Fatalf("response exactly at limit must be accepted: entries=%#v error=%v", exactEntries, err)
	}

	tooSmall := int64(len(response) - 1)
	session, _ := newLocalDP081LogsTestSession(t, response, appium.ClientOptions{
		Limits: appium.Limits{MaxLogResponseBytes: tooSmall},
	})
	entries, err := session.Logs(context.Background(), "type")
	if entries != nil {
		t.Fatalf("oversized response must not return entries: %#v", entries)
	}
	if err == nil || !appium.IsErrorCode(err, appium.CodeResponseTooLarge) {
		t.Fatalf("expected response too large error, got %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf("unexpected delivery for oversized response: %q", appium.DeliveryOf(err))
	}
}

func TestLocalDP081PullLogsCanceledBeforeDelivery(t *testing.T) {
	session, recorder := newLocalDP081LogsTestSession(t, `{"value":[]}`, appium.ClientOptions{})
	recorder.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	entries, err := session.Logs(ctx, "type")
	if entries != nil {
		t.Fatalf("canceled call must not return entries: %#v", entries)
	}
	if err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("canceled call must not be delivered: %d requests", len(requests))
	}
}

func TestLocalDP081PullLogsRejectsInvalidUTF8LogTypeBeforeDelivery(t *testing.T) {
	session, recorder := newLocalDP081LogsTestSession(t, `{"value":[]}`, appium.ClientOptions{})
	recorder.Reset()

	invalidLogType := appium.LogType(string([]byte{0xff}))
	entries, err := session.Logs(context.Background(), invalidLogType)
	if entries != nil {
		t.Fatalf("invalid UTF-8 log type must not return entries: %#v", entries)
	}
	if err == nil || !appium.IsErrorCode(err, appium.CodeInvalidArgument) {
		t.Fatalf("expected invalid argument error, got %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("invalid UTF-8 log type must not be delivered: %d requests", len(requests))
	}
}

func TestLocalDP081PullLogsRejectsInvalidUTF8(t *testing.T) {
	response := `{"value":[{"timestamp":1,"level":"INFO","message":"m","extra":"` +
		string([]byte{0xff}) +
		`"}]}`
	session, _ := newLocalDP081LogsTestSession(t, response, appium.ClientOptions{})
	entries, err := session.Logs(context.Background(), "type")
	if entries != nil {
		t.Fatalf("invalid UTF-8 response must not return entries: %#v", entries)
	}
	assertLocalDP081ResponseInvalid(t, err)
}

func newLocalDP081LogsTestSession(
	t *testing.T,
	response string,
	options appium.ClientOptions,
) (*appium.Session, *contracttest.Recorder) {
	t.Helper()
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.RequestURI == "/session":
			_, _ = writer.Write([]byte(`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`))
		case request.Method == http.MethodGet && request.RequestURI == "/session/session/se/log/types":
			_, _ = writer.Write([]byte(response))
		case request.Method == http.MethodPost && request.RequestURI == "/session/session/se/log":
			_, _ = writer.Write([]byte(response))
		default:
			http.NotFound(writer, request)
		}
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)

	client, err := server.NewClient(options)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"platformName": "iOS", "appium:automationName": "XCUITest",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return session, recorder
}

func assertLocalDP081ResponseInvalid(t *testing.T, err error) {
	t.Helper()
	if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
		t.Fatalf("expected response invalid error, got %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
	var clientErr *appium.Error
	if !errors.As(err, &clientErr) || clientErr.StatusCode != http.StatusOK {
		t.Fatalf("unexpected structured error: %#v", clientErr)
	}
}
