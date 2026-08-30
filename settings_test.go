package soluna_appium_client_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestSessionSettingsProtocolAndNoCache(t *testing.T) {
	getCount := 0
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.RequestURI == "/session":
			_, _ = writer.Write([]byte(`{"value":{"sessionId":"session/id","capabilities":{"automationName":"XCUITest"}}}`))
		case request.Method == http.MethodGet && request.RequestURI == "/session/session%2Fid/appium/settings":
			getCount++
			if getCount == 1 {
				_, _ = writer.Write([]byte(`{"value":{"enabled":true,"nested":{"source":"first"},"items":[1,"two"],"nullable":null}}`))
				return
			}
			_, _ = writer.Write([]byte(`{"value":{"enabled":false,"nested":{"source":"second"}}}`))
		case request.Method == http.MethodPost && request.RequestURI == "/session/session%2Fid/appium/settings":
			_, _ = writer.Write([]byte(`{"value":null}`))
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

	first, err := session.Settings(context.Background())
	if err != nil {
		t.Fatalf("read first settings: %v", err)
	}
	expectedFirst := appium.Settings{
		"enabled":  true,
		"nested":   map[string]any{"source": "first"},
		"items":    []any{float64(1), "two"},
		"nullable": nil,
	}
	if !reflect.DeepEqual(first, expectedFirst) {
		t.Fatalf("unexpected first settings: expected %#v, got %#v", expectedFirst, first)
	}

	// 返回值中的嵌套容器必须是调用方独立拥有的副本。
	firstNested, ok := first["nested"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected nested settings type: %T", first["nested"])
	}
	firstNested["source"] = "mutated"

	second, err := session.Settings(context.Background())
	if err != nil {
		t.Fatalf("read second settings: %v", err)
	}
	expectedSecond := appium.Settings{
		"enabled": false,
		"nested":  map[string]any{"source": "second"},
	}
	if !reflect.DeepEqual(second, expectedSecond) {
		t.Fatalf("unexpected second settings: expected %#v, got %#v", expectedSecond, second)
	}

	update := appium.Settings{
		"enabled":   true,
		"threshold": 3.5,
		"nested":    map[string]any{"mode": "fast"},
		"nullable":  nil,
		"items":     []any{"one", float64(2)},
	}
	if err := session.UpdateSettings(context.Background(), update); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if err := session.UpdateSettings(context.Background(), appium.Settings{}); err != nil {
		t.Fatalf("update empty settings: %v", err)
	}

	requests := recorder.Requests()
	if len(requests) != 5 {
		t.Fatalf("unexpected request count: expected 5, got %d", len(requests))
	}
	for _, request := range requests[1:3] {
		if err := contracttest.MatchMethod(request, http.MethodGet); err != nil {
			t.Fatal(err)
		}
		if err := contracttest.MatchRequestURI(request, "/session/session%2Fid/appium/settings"); err != nil {
			t.Fatal(err)
		}
		if err := contracttest.MatchBody(request, nil); err != nil {
			t.Fatal(err)
		}
		if values := request.Header.Values("Content-Type"); len(values) != 0 {
			t.Fatalf("unexpected Content-Type header for GET settings: %v", values)
		}
	}

	updateRequest := requests[3]
	if err := contracttest.MatchMethod(updateRequest, http.MethodPost); err != nil {
		t.Fatal(err)
	}
	if err := contracttest.MatchRequestURI(updateRequest, "/session/session%2Fid/appium/settings"); err != nil {
		t.Fatal(err)
	}
	if err := contracttest.MatchHeader(updateRequest, "Content-Type", "application/json"); err != nil {
		t.Fatal(err)
	}
	if err := contracttest.MatchJSONBody(updateRequest, map[string]any{
		"settings": update,
	}); err != nil {
		t.Fatal(err)
	}

	emptyUpdateRequest := requests[4]
	if err := contracttest.MatchMethod(emptyUpdateRequest, http.MethodPost); err != nil {
		t.Fatal(err)
	}
	if err := contracttest.MatchRequestURI(emptyUpdateRequest, "/session/session%2Fid/appium/settings"); err != nil {
		t.Fatal(err)
	}
	if err := contracttest.MatchHeader(emptyUpdateRequest, "Content-Type", "application/json"); err != nil {
		t.Fatal(err)
	}
	if err := contracttest.MatchJSONBody(emptyUpdateRequest, map[string]any{
		"settings": appium.Settings{},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSessionSettingsRejectInvalidResponses(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{name: "null", response: `{"value":null}`},
		{name: "array", response: `{"value":[]}`},
		{name: "string", response: `{"value":"settings"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session, _ := newSettingsTestSession(t, test.response)
			_, err := session.Settings(context.Background())
			if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
				t.Fatalf("expected response invalid error, got %v", err)
			}
			if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
				t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
			}
		})
	}
}

func TestSessionUpdateSettingsRejectsNilBeforeDelivery(t *testing.T) {
	session, recorder := newSettingsTestSession(t, `{"value":{}}`)
	recorder.Reset()

	err := session.UpdateSettings(context.Background(), nil)
	if err == nil || !appium.IsErrorCode(err, appium.CodeInvalidArgument) {
		t.Fatalf("expected invalid argument error, got %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("nil settings must not be delivered: got %d requests", len(requests))
	}
}

func TestSessionUpdateSettingsRejectsNonNullResponse(t *testing.T) {
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.RequestURI == "/session":
			_, _ = writer.Write([]byte(`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`))
		case request.Method == http.MethodPost && request.RequestURI == "/session/session/appium/settings":
			_, _ = writer.Write([]byte(`{"value":{}}`))
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

	err = session.UpdateSettings(context.Background(), appium.Settings{"enabled": true})
	if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
		t.Fatalf("expected response invalid error, got %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
}

func TestSessionSettingsCanceledBeforeDelivery(t *testing.T) {
	session, recorder := newSettingsTestSession(t, `{"value":{}}`)
	recorder.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := session.Settings(ctx)
	if err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("canceled settings read must not be delivered: got %d requests", len(requests))
	}
}

func newSettingsTestSession(t *testing.T, response string) (*appium.Session, *contracttest.Recorder) {
	t.Helper()
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost && request.RequestURI == "/session" {
			_, _ = writer.Write([]byte(`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`))
			return
		}
		_, _ = writer.Write([]byte(response))
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
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
	return session, recorder
}
