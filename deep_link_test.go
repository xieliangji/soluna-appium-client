package appium_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

const dp130ExecutePath = "/session/session%2Fid/execute/sync"

func TestDP130DeepLinkMapsRemoteDriverAndPreservesArguments(t *testing.T) {
	for _, driver := range []struct {
		name string
		key  string
	}{
		{name: "XCUITest", key: "bundleId"},
		{name: "UiAutomator2", key: "package"},
	} {
		t.Run(driver.name, func(t *testing.T) {
			session, recorder := newDP130DeepLinkSession(t, driver.name, appium.ClientOptions{},
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(`{"value":null}`))
				}),
			)
			// 外部 Capability 副本不能改变已确认的 Driver 或默认补全目标 App。
			capabilities := session.Capabilities()
			capabilities["automationName"] = "OtherDriver"
			capabilities["bundleId"] = "com.example.modified"

			for _, input := range []struct {
				url   string
				appID string
			}{
				{url: "example://content/42", appID: "com.example.target"},
				{url: "HTTPS://example.test/a%2fb?x=1+2&y=%20#Frag"},
				{url: " custom+测试://路径/%2f?x=1+2#片段 ", appID: " com.Example.目标 "},
			} {
				recorder.Reset()
				if err := session.DeepLink(context.Background(), input.url, input.appID); err != nil {
					t.Fatalf("deep link: %v", err)
				}
				requests := recorder.Requests()
				if len(requests) != 1 {
					t.Fatalf("request count = %d, want 1", len(requests))
				}
				request := requests[0]
				if err := contracttest.MatchMethod(request, http.MethodPost); err != nil {
					t.Fatal(err)
				}
				if err := contracttest.MatchRequestURI(request, dp130ExecutePath); err != nil {
					t.Fatal(err)
				}
				if err := contracttest.MatchHeader(request, "Content-Type", "application/json"); err != nil {
					t.Fatal(err)
				}
				arguments := map[string]any{"url": input.url}
				if input.appID != "" {
					arguments[driver.key] = input.appID
				}
				if err := contracttest.MatchJSONBody(request, map[string]any{
					"script": "mobile: deepLink",
					"args":   []any{arguments},
				}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestDP130DeepLinkRejectsUnknownDriverWithoutRequests(t *testing.T) {
	for _, driver := range []string{
		"Espresso", "OtherDriver", "xcuitest", "XCUITest ", "uiautomator2", " UIAutomator2",
	} {
		t.Run(driver, func(t *testing.T) {
			session, recorder := newDP130DeepLinkSession(t, driver, appium.ClientOptions{}, http.NotFoundHandler())
			err := session.DeepLink(context.Background(), "example://open", "com.example.target")
			assertDP130Error(t, err, appium.CodeUnsupported, appium.DeliveryNotSent, 0)
			if requests := recorder.Requests(); len(requests) != 0 {
				t.Fatalf("unsupported driver sent %d requests, want 0", len(requests))
			}
		})
	}
}

func TestDP130DeepLinkLocalFailuresSendNoRequests(t *testing.T) {
	for _, driver := range []string{"XCUITest", "UiAutomator2"} {
		t.Run(driver, func(t *testing.T) {
			session, recorder := newDP130DeepLinkSession(t, driver, appium.ClientOptions{}, http.NotFoundHandler())
			canceled, cancel := context.WithCancel(context.Background())
			cancel()
			expired, cancelDeadline := context.WithDeadline(context.Background(), time.Unix(1, 0))
			defer cancelDeadline()
			for _, test := range []struct {
				name  string
				ctx   context.Context
				url   string
				appID string
				code  appium.ErrorCode
				cause error
			}{
				{name: "empty URL", ctx: context.Background(), code: appium.CodeInvalidArgument},
				{name: "invalid URL UTF-8", ctx: context.Background(), url: "example://\xff", code: appium.CodeInvalidArgument},
				{name: "invalid app ID UTF-8", ctx: context.Background(), url: "example://open", appID: "com.\xff", code: appium.CodeInvalidArgument},
				{name: "nil context", url: "example://open", code: appium.CodeInvalidArgument},
				{name: "canceled", ctx: canceled, url: "example://open", code: appium.CodeCanceled, cause: context.Canceled},
				{name: "expired", ctx: expired, url: "example://open", code: appium.CodeDeadlineExceeded, cause: context.DeadlineExceeded},
			} {
				t.Run(test.name, func(t *testing.T) {
					err := session.DeepLink(test.ctx, test.url, test.appID)
					assertDP130Error(t, err, test.code, appium.DeliveryNotSent, 0)
					if test.cause != nil && !errors.Is(err, test.cause) {
						t.Fatalf("error = %v, want cause %v", err, test.cause)
					}
					if requests := recorder.Requests(); len(requests) != 0 {
						t.Fatalf("local failure sent %d requests, want 0", len(requests))
					}
				})
			}
		})
	}
}

func TestDP130DeepLinkRejectsUninitializedAndClosedSessions(t *testing.T) {
	for _, session := range []*appium.Session{nil, {}} {
		err := session.DeepLink(context.Background(), "example://open", "")
		assertDP130Error(t, err, appium.CodeInvalidArgument, appium.DeliveryNotSent, 0)
	}
	session, recorder := newDP130DeepLinkSession(t, "XCUITest", appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodDelete {
				http.NotFound(writer, request)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"value":null}`))
		}),
	)
	if err := session.Close(context.Background()); err != nil {
		t.Fatalf("close session: %v", err)
	}
	recorder.Reset()
	err := session.DeepLink(context.Background(), "example://open", "")
	assertDP130Error(t, err, appium.CodeSessionLost, appium.DeliveryNotSent, 0)
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("closed session sent %d requests, want 0", len(requests))
	}
}

func TestDP130DeepLinkResponseAndObserverUseCommandContract(t *testing.T) {
	for _, driver := range []string{"XCUITest", "UiAutomator2"} {
		t.Run(driver, func(t *testing.T) {
			for _, test := range []struct {
				name   string
				body   string
				status int
				code   appium.ErrorCode
				remote string
			}{
				{name: "null", body: `{"value":null}`, status: 200},
				{name: "true", body: `{"value":true}`, status: 200, code: appium.CodeResponseInvalid},
				{name: "false", body: `{"value":false}`, status: 200, code: appium.CodeResponseInvalid},
				{name: "number", body: `{"value":0}`, status: 200, code: appium.CodeResponseInvalid},
				{name: "string", body: `{"value":""}`, status: 200, code: appium.CodeResponseInvalid},
				{name: "object", body: `{"value":{}}`, status: 200, code: appium.CodeResponseInvalid},
				{name: "array", body: `{"value":[]}`, status: 200, code: appium.CodeResponseInvalid},
				{name: "missing value", body: `{}`, status: 200, code: appium.CodeResponseInvalid},
				{name: "invalid JSON", body: `{"value":`, status: 200, code: appium.CodeResponseInvalid},
				{name: "unknown command", status: 404, code: appium.CodeUnsupported, remote: "unknown command"},
				{name: "unsupported operation", status: 500, code: appium.CodeUnsupported, remote: "unsupported operation"},
				{name: "invalid argument", status: 400, code: appium.CodeInvalidArgument, remote: "invalid argument"},
				{name: "session lost", status: 404, code: appium.CodeSessionLost, remote: "invalid session id"},
				{name: "command failed", status: 500, code: appium.CodeCommandFailed, remote: "unknown error"},
			} {
				t.Run(test.name, func(t *testing.T) {
					observer := &commandObserverRecorder{}
					session, recorder := newDP130DeepLinkSession(t, driver, appium.ClientOptions{Observer: observer},
						http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
							writer.Header().Set("Content-Type", "application/json")
							writer.WriteHeader(test.status)
							if test.remote != "" {
								_, _ = writer.Write([]byte(`{"value":{"error":"` + test.remote + `","message":"synthetic failure"}}`))
								return
							}
							_, _ = writer.Write([]byte(test.body))
						}),
					)
					observer.reset()
					err := session.DeepLink(context.Background(), "example://open", "com.example.target")
					if test.code == "" {
						if err != nil {
							t.Fatalf("deep link: %v", err)
						}
					} else {
						clientErr := assertDP130Error(t, err, test.code, appium.DeliveryAcknowledged, test.status)
						if clientErr.RemoteCode != test.remote {
							t.Fatalf("remote code = %q, want %q", clientErr.RemoteCode, test.remote)
						}
					}
					if requests := recorder.Requests(); len(requests) != 1 {
						t.Fatalf("request count = %d, want 1 without fallback or confirmation", len(requests))
					}
					started, finished := observer.snapshot()
					if len(started) != 1 || len(finished) != 1 {
						t.Fatalf("observer events = %d/%d, want 1/1", len(started), len(finished))
					}
					if started[0].Operation != "deep_link" || finished[0].Operation != "deep_link" ||
						finished[0].ErrorCode != test.code || finished[0].StatusCode != test.status ||
						finished[0].Delivery != appium.DeliveryAcknowledged {
						t.Fatalf("unexpected observer events: started=%+v finished=%+v", started[0], finished[0])
					}
				})
			}
		})
	}
}

func TestDP130DeepLinkUnknownDeliveryIsNotReplayed(t *testing.T) {
	for _, driver := range []string{"XCUITest", "UiAutomator2"} {
		t.Run(driver, func(t *testing.T) {
			observer := &commandObserverRecorder{}
			hijacked := make(chan error, 8)
			session, recorder := newDP130DeepLinkSession(t, driver, appium.ClientOptions{Observer: observer},
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					connection, _, err := http.NewResponseController(writer).Hijack()
					hijacked <- err
					if err == nil {
						_ = connection.Close()
					}
				}),
			)
			observer.reset()
			err := session.DeepLink(context.Background(), "example://open", "com.example.target")
			if hijackErr := <-hijacked; hijackErr != nil {
				t.Fatalf("hijack connection: %v", hijackErr)
			}
			assertDP130Error(t, err, appium.CodeTransportFailed, appium.DeliveryUnknown, 0)
			if requests := recorder.Requests(); len(requests) != 1 {
				t.Fatalf("request count = %d, want 1 without replay", len(requests))
			}
			started, finished := observer.snapshot()
			if len(started) != 1 || len(finished) != 1 ||
				started[0].Operation != "deep_link" || finished[0].Operation != "deep_link" ||
				finished[0].ErrorCode != appium.CodeTransportFailed || finished[0].Delivery != appium.DeliveryUnknown {
				t.Fatalf("unexpected observer events: started=%+v finished=%+v", started, finished)
			}
		})
	}
}

func assertDP130Error(
	t *testing.T,
	err error,
	code appium.ErrorCode,
	delivery appium.DeliveryState,
	status int,
) *appium.Error {
	t.Helper()
	var clientErr *appium.Error
	if !errors.As(err, &clientErr) {
		t.Fatalf("error = %v, want *appium.Error", err)
	}
	if clientErr.Code != code || clientErr.Operation != "deep_link" ||
		clientErr.Delivery != delivery || clientErr.StatusCode != status {
		t.Fatalf("error = %+v, want code=%s operation=deep_link delivery=%s status=%d", clientErr, code, delivery, status)
	}
	return clientErr
}

func newDP130DeepLinkSession(
	t *testing.T,
	automationName string,
	options appium.ClientOptions,
	commandHandler http.Handler,
) (*appium.Session, *contracttest.Recorder) {
	t.Helper()
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost && request.RequestURI == "/session" {
			writer.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(writer).Encode(map[string]any{"value": map[string]any{
				"sessionId": "session/id",
				"capabilities": map[string]any{
					"automationName":        automationName,
					"appium:automationName": "OtherDriver",
					"bundleId":              "com.example.capability.ios",
					"appPackage":            "com.example.capability.android",
				},
			}})
			return
		}
		commandHandler.ServeHTTP(writer, request)
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
	client, err := server.NewClient(options)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	requestedDriver := "XCUITest"
	if automationName == "XCUITest" {
		requestedDriver = "UiAutomator2"
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"appium:automationName": requestedDriver,
		"appium:bundleId":       "com.example.request.ios",
		"appium:appPackage":     "com.example.request.android",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	recorder.Reset()
	return session, recorder
}
