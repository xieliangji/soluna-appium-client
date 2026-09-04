package appium_test

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

const dp120ExecutePath = "/session/session%2Fid/execute/sync"

func TestDP120ActiveAppIDMapsDriverResultsToFreshSnapshots(t *testing.T) {
	for _, test := range []struct {
		name           string
		automationName string
		script         string
		responses      [2]string
		expected       [2]string
	}{
		{
			name:           "XCUITest bundleId",
			automationName: "XCUITest",
			script:         "mobile: activeAppInfo",
			responses: [2]string{
				`{"bundleId":"com.example.ios.first","pid":41,"name":"First","processArguments":{"args":[],"env":{}}}`,
				`{"name":"Second","bundleId":" com.Example.ios.second ","unexpected":{"ignored":true}}`,
			},
			expected: [2]string{
				"com.example.ios.first",
				" com.Example.ios.second ",
			},
		},
		{
			name:           "UiAutomator2 package",
			automationName: "UiAutomator2",
			script:         "mobile: getCurrentPackage",
			responses: [2]string{
				`"com.example.android.first"`,
				`" com.Example.android.second "`,
			},
			expected: [2]string{
				"com.example.android.first",
				" com.Example.android.second ",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var reads atomic.Int32
			session, recorder := newDP120ActiveAppSession(
				t,
				test.automationName,
				appium.ClientOptions{},
				http.HandlerFunc(func(
					writer http.ResponseWriter,
					request *http.Request,
				) {
					if request.Method != http.MethodPost ||
						request.RequestURI != dp120ExecutePath {
						http.NotFound(writer, request)
						return
					}

					index := int(reads.Add(1)) - 1
					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(
						`{"value":` + test.responses[index] + `}`,
					))
				}),
			)

			for index, expected := range test.expected {
				appID, err := session.ActiveAppID(context.Background())
				if err != nil {
					t.Fatalf("active app ID snapshot %d: %v", index, err)
				}
				if appID != expected {
					t.Fatalf(
						"active app ID snapshot %d = %q, want %q",
						index,
						appID,
						expected,
					)
				}
			}

			requests := recorder.Requests()
			if len(requests) != len(test.expected) {
				t.Fatalf(
					"request count = %d, want %d",
					len(requests),
					len(test.expected),
				)
			}

			for index, request := range requests {
				if err := contracttest.MatchMethod(request, http.MethodPost); err != nil {
					t.Fatalf("request %d method: %v", index, err)
				}
				if err := contracttest.MatchRequestURI(
					request,
					dp120ExecutePath,
				); err != nil {
					t.Fatalf("request %d URI: %v", index, err)
				}
				if err := contracttest.MatchJSONBody(
					request,
					map[string]any{
						"script": test.script,
						"args":   []any{},
					},
				); err != nil {
					t.Fatalf("request %d body: %v", index, err)
				}
				if err := contracttest.MatchHeader(
					request,
					"Content-Type",
					"application/json",
				); err != nil {
					t.Fatalf("request %d header: %v", index, err)
				}
			}
		})
	}
}

func TestDP120ActiveAppIDRejectsUnknownDriverWithoutCapabilityInference(
	t *testing.T,
) {
	for _, automationName := range []string{
		"OtherDriver",
		"xcuitest",
		"uiautomator2",
	} {
		t.Run(automationName, func(t *testing.T) {
			session, recorder := newDP120ActiveAppSession(
				t,
				automationName,
				appium.ClientOptions{},
				http.NotFoundHandler(),
			)

			appID, err := session.ActiveAppID(context.Background())
			if appID != "" {
				t.Fatalf("active app ID = %q on failure, want empty", appID)
			}
			if err == nil || !appium.IsErrorCode(err, appium.CodeUnsupported) {
				t.Fatalf("error = %v, want unsupported", err)
			}
			if appium.DeliveryOf(err) != appium.DeliveryNotSent {
				t.Fatalf(
					"delivery = %q, want not_sent",
					appium.DeliveryOf(err),
				)
			}

			var clientErr *appium.Error
			if !errors.As(err, &clientErr) {
				t.Fatalf("error type = %T, want *appium.Error", err)
			}
			if clientErr.Operation != "get_active_app_id" {
				t.Fatalf(
					"operation = %q, want get_active_app_id",
					clientErr.Operation,
				)
			}
			if requests := recorder.Requests(); len(requests) != 0 {
				t.Fatalf(
					"unknown driver sent %d requests, want 0",
					len(requests),
				)
			}
		})
	}
}

func TestDP120ActiveAppIDPreservesUiAutomator2NullAsNoFocusedPackage(
	t *testing.T,
) {
	session, recorder := newDP120ActiveAppSession(
		t,
		"UiAutomator2",
		appium.ClientOptions{},
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"value":null}`))
		}),
	)

	appID, err := session.ActiveAppID(context.Background())
	if err != nil {
		t.Fatalf("active app ID without focused package: %v", err)
	}
	if appID != "" {
		t.Fatalf("active app ID = %q, want empty", appID)
	}
	if requests := recorder.Requests(); len(requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(requests))
	}
}

func TestDP120ActiveAppIDRejectsInvalidDriverResults(t *testing.T) {
	for _, test := range []struct {
		name           string
		automationName string
		response       string
	}{
		{name: "XCUITest null", automationName: "XCUITest", response: `null`},
		{name: "XCUITest string", automationName: "XCUITest", response: `"com.example"`},
		{name: "XCUITest array", automationName: "XCUITest", response: `[]`},
		{name: "XCUITest missing bundleId", automationName: "XCUITest", response: `{}`},
		{name: "XCUITest wrong key case", automationName: "XCUITest", response: `{"BundleId":"com.example"}`},
		{name: "XCUITest null bundleId", automationName: "XCUITest", response: `{"bundleId":null}`},
		{name: "XCUITest numeric bundleId", automationName: "XCUITest", response: `{"bundleId":7}`},
		{name: "XCUITest empty bundleId", automationName: "XCUITest", response: `{"bundleId":""}`},
		{name: "XCUITest unpaired surrogate", automationName: "XCUITest", response: `{"bundleId":"\ud800"}`},
		{name: "XCUITest invalid UTF-8", automationName: "XCUITest", response: "{\"bundleId\":\"\xff\"}"},
		{name: "UiAutomator2 boolean", automationName: "UiAutomator2", response: `true`},
		{name: "UiAutomator2 number", automationName: "UiAutomator2", response: `1`},
		{name: "UiAutomator2 object", automationName: "UiAutomator2", response: `{}`},
		{name: "UiAutomator2 array", automationName: "UiAutomator2", response: `[]`},
		{name: "UiAutomator2 empty package", automationName: "UiAutomator2", response: `""`},
		{name: "UiAutomator2 unpaired surrogate", automationName: "UiAutomator2", response: `"\ud800"`},
		{name: "UiAutomator2 invalid UTF-8", automationName: "UiAutomator2", response: "\"\xff\""},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, recorder := newDP120ActiveAppSession(
				t,
				test.automationName,
				appium.ClientOptions{},
				http.HandlerFunc(func(
					writer http.ResponseWriter,
					request *http.Request,
				) {
					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(
						`{"value":` + test.response + `}`,
					))
				}),
			)

			appID, err := session.ActiveAppID(context.Background())
			if appID != "" {
				t.Fatalf("active app ID = %q on failure, want empty", appID)
			}
			assertDP120ResponseInvalid(t, err)
			if requests := recorder.Requests(); len(requests) != 1 {
				t.Fatalf("request count = %d, want 1", len(requests))
			}
		})
	}
}

func TestDP120ActiveAppIDDecoderFinishesInsideObservedCommand(t *testing.T) {
	observer := &commandObserverRecorder{}
	session, recorder := newDP120ActiveAppSession(
		t,
		"XCUITest",
		appium.ClientOptions{Observer: observer},
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"value":{"name":"missing bundle"}}`))
		}),
	)
	observer.reset()
	recorder.Reset()

	_, err := session.ActiveAppID(context.Background())
	assertDP120ResponseInvalid(t, err)

	started, finished := observer.snapshot()
	if len(started) != 1 || len(finished) != 1 {
		t.Fatalf(
			"observer events = started %d, finished %d; want 1, 1",
			len(started),
			len(finished),
		)
	}
	if started[0].Operation != "get_active_app_id" ||
		finished[0].Operation != "get_active_app_id" {
		t.Fatalf(
			"observer operations = %q, %q; want get_active_app_id",
			started[0].Operation,
			finished[0].Operation,
		)
	}
	if finished[0].ErrorCode != appium.CodeResponseInvalid {
		t.Fatalf(
			"finished error code = %q, want response_invalid",
			finished[0].ErrorCode,
		)
	}
	if finished[0].Delivery != appium.DeliveryAcknowledged {
		t.Fatalf(
			"finished delivery = %q, want acknowledged",
			finished[0].Delivery,
		)
	}
}

func TestDP120ActiveAppIDRemoteFailureDoesNotFallback(t *testing.T) {
	for _, automationName := range []string{"XCUITest", "UiAutomator2"} {
		t.Run(automationName, func(t *testing.T) {
			session, recorder := newDP120ActiveAppSession(
				t,
				automationName,
				appium.ClientOptions{},
				http.HandlerFunc(func(
					writer http.ResponseWriter,
					request *http.Request,
				) {
					writer.Header().Set("Content-Type", "application/json")
					if request.RequestURI == dp120ExecutePath {
						writer.WriteHeader(http.StatusNotFound)
						_, _ = writer.Write([]byte(
							`{"value":{"error":"unknown command","message":"active app execute method unavailable"}}`,
						))
						return
					}

					// These compatibility and internal routes would succeed if used.
					_, _ = writer.Write([]byte(
						`{"value":"com.example.fallback"}`,
					))
				}),
			)

			appID, err := session.ActiveAppID(context.Background())
			if appID != "" {
				t.Fatalf("active app ID = %q on failure, want empty", appID)
			}
			if err == nil || !appium.IsErrorCode(err, appium.CodeUnsupported) {
				t.Fatalf("error = %v, want unsupported", err)
			}
			if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
				t.Fatalf(
					"delivery = %q, want acknowledged",
					appium.DeliveryOf(err),
				)
			}

			var clientErr *appium.Error
			if !errors.As(err, &clientErr) {
				t.Fatalf("error type = %T, want *appium.Error", err)
			}
			if clientErr.Operation != "get_active_app_id" {
				t.Fatalf(
					"operation = %q, want get_active_app_id",
					clientErr.Operation,
				)
			}

			requests := recorder.Requests()
			if len(requests) != 1 {
				t.Fatalf(
					"remote failure triggered fallback: got %d requests, want 1",
					len(requests),
				)
			}
			if requests[0].RequestURI != dp120ExecutePath {
				t.Fatalf(
					"request URI = %q, want %q",
					requests[0].RequestURI,
					dp120ExecutePath,
				)
			}
		})
	}
}

func TestDP120ActiveAppIDUnknownDeliveryIsNotReplayed(t *testing.T) {
	hijacked := make(chan error, 1)
	var calls atomic.Int32
	session, recorder := newDP120ActiveAppSession(
		t,
		"UiAutomator2",
		appium.ClientOptions{},
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			calls.Add(1)
			if request.Method != http.MethodPost ||
				request.RequestURI != dp120ExecutePath {
				http.NotFound(writer, request)
				return
			}

			connection, _, err := http.NewResponseController(writer).Hijack()
			if err != nil {
				hijacked <- err
				return
			}
			hijacked <- nil
			_ = connection.Close()
		}),
	)

	appID, err := session.ActiveAppID(context.Background())
	if hijackErr := <-hijacked; hijackErr != nil {
		t.Fatalf("hijack connection: %v", hijackErr)
	}
	if appID != "" {
		t.Fatalf("active app ID = %q on failure, want empty", appID)
	}
	if err == nil || !appium.IsErrorCode(err, appium.CodeTransportFailed) {
		t.Fatalf("error = %v, want transport failure", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryUnknown {
		t.Fatalf("delivery = %q, want unknown", appium.DeliveryOf(err))
	}
	if calls.Load() != 1 {
		t.Fatalf("handler call count = %d, want 1", calls.Load())
	}
	if requests := recorder.Requests(); len(requests) != 1 {
		t.Fatalf(
			"unknown delivery replayed request: got %d requests, want 1",
			len(requests),
		)
	}
}

func TestDP120ActiveAppIDCanceledBeforeDelivery(t *testing.T) {
	for _, automationName := range []string{"XCUITest", "UiAutomator2"} {
		t.Run(automationName, func(t *testing.T) {
			session, recorder := newDP120ActiveAppSession(
				t,
				automationName,
				appium.ClientOptions{},
				http.NotFoundHandler(),
			)

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			appID, err := session.ActiveAppID(ctx)
			if appID != "" {
				t.Fatalf("active app ID = %q on cancellation, want empty", appID)
			}
			if err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
				t.Fatalf("error = %v, want canceled", err)
			}
			if appium.DeliveryOf(err) != appium.DeliveryNotSent {
				t.Fatalf(
					"delivery = %q, want not_sent",
					appium.DeliveryOf(err),
				)
			}
			if requests := recorder.Requests(); len(requests) != 0 {
				t.Fatalf(
					"canceled read sent %d requests, want 0",
					len(requests),
				)
			}
		})
	}
}

func assertDP120ResponseInvalid(t *testing.T, err error) {
	t.Helper()

	if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
		t.Fatalf("error = %v, want response invalid", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf(
			"delivery = %q, want acknowledged",
			appium.DeliveryOf(err),
		)
	}

	var clientErr *appium.Error
	if !errors.As(err, &clientErr) {
		t.Fatalf("error type = %T, want *appium.Error", err)
	}
	if clientErr.Operation != "get_active_app_id" {
		t.Fatalf(
			"operation = %q, want get_active_app_id",
			clientErr.Operation,
		)
	}
}

func newDP120ActiveAppSession(
	t *testing.T,
	automationName string,
	options appium.ClientOptions,
	activeAppHandler http.Handler,
) (*appium.Session, *contracttest.Recorder) {
	t.Helper()

	recorder := contracttest.NewRecorder(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.Method == http.MethodPost && request.RequestURI == "/session" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(
				`{"value":{"sessionId":"session/id","capabilities":{` +
					`"automationName":"` + automationName + `",` +
					`"bundleId":"com.example.capability.ios",` +
					`"appPackage":"com.example.capability.android"}}}`,
			))
			return
		}

		activeAppHandler.ServeHTTP(writer, request)
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)

	client, err := server.NewClient(options)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(
		context.Background(),
		appium.MatchCapabilities(appium.Capabilities{
			"platformName":          "synthetic",
			"appium:automationName": automationName,
			"appium:bundleId":       "com.example.request.ios",
			"appium:appPackage":     "com.example.request.android",
		}),
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	recorder.Reset()
	return session, recorder
}
