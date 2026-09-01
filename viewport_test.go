package appium_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestSessionViewportRectProtocolForSupportedDrivers(t *testing.T) {
	tests := []struct {
		name           string
		automationName string
		response       string
		expected       appium.PixelRect
	}{
		{
			name:           "XCUITest",
			automationName: "XCUITest",
			response:       `{"left":0,"top":88,"width":1170,"height":2452}`,
			expected: appium.PixelRect{
				X:      0,
				Y:      88,
				Width:  1170,
				Height: 2452,
			},
		},
		{
			name:           "UiAutomator2",
			automationName: "UiAutomator2",
			response:       `{"left":0,"top":61,"width":720,"height":1543}`,
			expected: appium.PixelRect{
				X:      0,
				Y:      61,
				Width:  720,
				Height: 1543,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := contracttest.NewRecorder(
				http.HandlerFunc(func(
					writer http.ResponseWriter,
					request *http.Request,
				) {
					writer.Header().Set("Content-Type", "application/json")

					switch {
					case request.Method == http.MethodPost &&
						request.RequestURI == "/session":
						_, _ = writer.Write([]byte(fmt.Sprintf(
							`{"value":{"sessionId":"session/id","capabilities":{"automationName":%q}}}`,
							test.automationName,
						)))

					case request.Method == http.MethodPost &&
						request.RequestURI == "/session/session%2Fid/execute/sync":
						_, _ = writer.Write([]byte(
							`{"value":` + test.response + `}`,
						))

					default:
						http.NotFound(writer, request)
					}
				}),
			)
			server := contracttest.NewServer(recorder)
			t.Cleanup(server.Close)

			client, err := server.NewClient(appium.ClientOptions{})
			if err != nil {
				t.Fatalf("create client: %v", err)
			}
			session, err := client.CreateSession(
				context.Background(),
				appium.MatchCapabilities(appium.Capabilities{
					"platformName":          "synthetic",
					"appium:automationName": test.automationName,
				}),
			)
			if err != nil {
				t.Fatalf("create session: %v", err)
			}

			rect, err := session.ViewportRect(context.Background())
			if err != nil {
				t.Fatalf("get viewport rect: %v", err)
			}
			if rect != test.expected {
				t.Fatalf("unexpected viewport rect: expected %+v, got %+v", test.expected, rect)
			}

			requests := recorder.Requests()
			if len(requests) != 2 {
				t.Fatalf("unexpected request count: expected 2, got %d", len(requests))
			}
			executeRequest := requests[1]
			if err := contracttest.MatchMethod(executeRequest, http.MethodPost); err != nil {
				t.Fatal(err)
			}
			if err := contracttest.MatchRequestURI(
				executeRequest,
				"/session/session%2Fid/execute/sync",
			); err != nil {
				t.Fatal(err)
			}
			if err := contracttest.MatchHeader(
				executeRequest,
				"Content-Type",
				"application/json",
			); err != nil {
				t.Fatal(err)
			}
			if err := contracttest.MatchJSONBody(
				executeRequest,
				map[string]any{
					"script": "mobile: viewportRect",
					"args":   []any{},
				},
			); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSessionViewportRectReadsFreshSnapshot(t *testing.T) {
	var executeCount atomic.Int32
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.RequestURI == "/session":
			_, _ = writer.Write([]byte(
				`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
			))
		case request.Method == http.MethodPost && request.RequestURI == "/session/session/execute/sync":
			if executeCount.Add(1) == 1 {
				_, _ = writer.Write([]byte(`{"value":{"left":0,"top":1,"width":10,"height":20}}`))
				return
			}
			_, _ = writer.Write([]byte(`{"value":{"left":2,"top":3,"width":30,"height":40}}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"platformName": "synthetic", "appium:automationName": "XCUITest",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	first, err := session.ViewportRect(context.Background())
	if err != nil {
		t.Fatalf("read first viewport rect: %v", err)
	}
	second, err := session.ViewportRect(context.Background())
	if err != nil {
		t.Fatalf("read second viewport rect: %v", err)
	}
	if first != (appium.PixelRect{X: 0, Y: 1, Width: 10, Height: 20}) {
		t.Fatalf("unexpected first rect: %+v", first)
	}
	if second != (appium.PixelRect{X: 2, Y: 3, Width: 30, Height: 40}) {
		t.Fatalf("unexpected second rect: %+v", second)
	}
	if executeCount.Load() != 2 {
		t.Fatalf("expected two viewport reads, got %d", executeCount.Load())
	}
}

func TestSessionViewportRectRejectsUnknownDriverBeforeDelivery(t *testing.T) {
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost && request.RequestURI == "/session" {
			_, _ = writer.Write([]byte(
				`{"value":{"sessionId":"session","capabilities":{"automationName":"OtherDriver"}}}`,
			))
			return
		}
		http.Error(writer, "unexpected request", http.StatusInternalServerError)
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"platformName": "synthetic", "appium:automationName": "OtherDriver",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	recorder.Reset()

	_, err = session.ViewportRect(context.Background())
	if err == nil {
		t.Fatal("expected unsupported driver error")
	}
	if !appium.IsErrorCode(err, appium.CodeUnsupported) {
		t.Fatalf("unexpected error code: %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("unknown driver must not send execute request: got %d", len(requests))
	}
}

func TestSessionViewportRectRejectsInvalidResponses(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	overflow := strconv.FormatUint(uint64(maxInt)+1, 10)
	maxIntText := strconv.Itoa(maxInt)

	tests := []struct {
		name     string
		response string
	}{
		{name: "null", response: `null`},
		{name: "array", response: `[]`},
		{name: "missing field", response: `{"left":0,"top":0,"width":10}`},
		{name: "null field", response: `{"left":null,"top":0,"width":10,"height":10}`},
		{name: "wrong type", response: `{"left":"0","top":0,"width":10,"height":10}`},
		{name: "alias fields", response: `{"x":0,"y":0,"width":10,"height":10}`},
		{name: "uppercase left", response: `{"Left":0,"top":0,"width":10,"height":10}`},
		{name: "uppercase top", response: `{"left":0,"TOP":0,"width":10,"height":10}`},
		{name: "fractional", response: `{"left":0,"top":0,"width":1.5,"height":10}`},
		{name: "negative origin", response: `{"left":-1,"top":0,"width":10,"height":10}`},
		{name: "zero width", response: `{"left":0,"top":0,"width":0,"height":10}`},
		{name: "negative height", response: `{"left":0,"top":0,"width":10,"height":-1}`},
		{name: "int overflow", response: `{"left":` + overflow + `,"top":0,"width":10,"height":10}`},
		{name: "endpoint overflow", response: `{"left":` + maxIntText + `,"top":0,"width":1,"height":10}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session, recorder := newViewportTestSession(t, test.response)
			_, err := session.ViewportRect(context.Background())
			if err == nil {
				t.Fatal("expected invalid response error")
			}
			if !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
				t.Fatalf("unexpected error code: %v", err)
			}
			if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
				t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
			}
			if requests := recorder.Requests(); len(requests) != 2 {
				t.Fatalf("expected one execute request, got %d total requests", len(requests))
			}
		})
	}
}

func TestSessionViewportRectCanceledBeforeDelivery(t *testing.T) {
	session, recorder := newViewportTestSession(t, `{"left":0,"top":0,"width":10,"height":10}`)
	recorder.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := session.ViewportRect(ctx)
	if err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("canceled viewport read must not be delivered: got %d requests", len(requests))
	}
}

func TestSessionViewportRectPropagatesRemoteError(t *testing.T) {
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost && request.RequestURI == "/session" {
			_, _ = writer.Write([]byte(
				`{"value":{"sessionId":"session","capabilities":{"automationName":"UiAutomator2"}}}`,
			))
			return
		}
		writer.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"value": map[string]any{
				"error":   "unknown command",
				"message": "viewport rect is unsupported",
			},
		})
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"platformName": "synthetic", "appium:automationName": "UiAutomator2",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err = session.ViewportRect(context.Background())
	if err == nil || !appium.IsErrorCode(err, appium.CodeUnsupported) {
		t.Fatalf("expected unsupported remote error, got %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
	var clientErr *appium.Error
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected structured error, got %T", err)
	}
	if clientErr.Operation != "get_viewport_rect" {
		t.Fatalf("unexpected operation: %q", clientErr.Operation)
	}
}

func newViewportTestSession(
	t *testing.T,
	response string,
) (*appium.Session, *contracttest.Recorder) {
	t.Helper()
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.RequestURI == "/session":
			_, _ = writer.Write([]byte(
				`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
			))
		case request.Method == http.MethodPost && request.RequestURI == "/session/session/execute/sync":
			_, _ = writer.Write([]byte(`{"value":` + response + `}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"platformName": "synthetic", "appium:automationName": "XCUITest",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return session, recorder
}
