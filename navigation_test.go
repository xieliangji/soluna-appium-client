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

func TestDP110BackgroundAppSendsNoRestoreBackgroundCommand(t *testing.T) {
	for _, test := range []struct {
		name           string
		automationName string
		response       string
	}{
		{
			name:           "XCUITest null response",
			automationName: "XCUITest",
			response:       "null",
		},
		{
			name:           "UiAutomator2 true response",
			automationName: "UiAutomator2",
			response:       "true",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, recorder := newDP110BackgroundSession(
				t,
				test.automationName,
				appium.ClientOptions{},
				http.HandlerFunc(func(
					writer http.ResponseWriter,
					request *http.Request,
				) {
					if request.Method != http.MethodPost ||
						request.RequestURI != "/session/session%2Fid/appium/app/background" {
						http.NotFound(writer, request)
						return
					}

					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(`{"value":` + test.response + `}`))
				}),
			)

			if err := session.BackgroundApp(context.Background()); err != nil {
				t.Fatalf("background app: %v", err)
			}

			requests := recorder.Requests()
			if len(requests) != 1 {
				t.Fatalf("request count = %d, want 1", len(requests))
			}
			request := requests[0]
			if err := contracttest.MatchMethod(request, http.MethodPost); err != nil {
				t.Fatal(err)
			}
			if err := contracttest.MatchRequestURI(
				request,
				"/session/session%2Fid/appium/app/background",
			); err != nil {
				t.Fatal(err)
			}
			if err := contracttest.MatchJSONBody(
				request,
				map[string]any{"seconds": int64(-1)},
			); err != nil {
				t.Fatal(err)
			}
			if err := contracttest.MatchHeader(
				request,
				"Content-Type",
				"application/json",
			); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDP110BackgroundAppRejectsUnexpectedSuccessValues(t *testing.T) {
	for _, test := range []struct {
		name     string
		response string
	}{
		{name: "false", response: "false"},
		{name: "number", response: "-1"},
		{name: "string", response: `"true"`},
		{name: "object", response: `{}`},
		{name: "array", response: `[]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, recorder := newDP110BackgroundSession(
				t,
				"XCUITest",
				appium.ClientOptions{},
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(`{"value":` + test.response + `}`))
				}),
			)

			err := session.BackgroundApp(context.Background())
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
			if clientErr.Operation != "background_app" {
				t.Fatalf("operation = %q, want background_app", clientErr.Operation)
			}
			if requests := recorder.Requests(); len(requests) != 1 {
				t.Fatalf("request count = %d, want 1", len(requests))
			}
		})
	}
}

func TestDP110BackgroundAppObserverUsesCanonicalIdentity(t *testing.T) {
	observer := &commandObserverRecorder{}
	session, recorder := newDP110BackgroundSession(
		t,
		"XCUITest",
		appium.ClientOptions{Observer: observer},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"value":false}`))
		}),
	)
	observer.reset()
	recorder.Reset()

	err := session.BackgroundApp(context.Background())
	if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
		t.Fatalf("error = %v, want response invalid", err)
	}

	started, finished := observer.snapshot()
	if len(started) != 1 || len(finished) != 1 {
		t.Fatalf(
			"observer events = started %d, finished %d; want 1, 1",
			len(started),
			len(finished),
		)
	}
	if started[0].Operation != "background_app" ||
		finished[0].Operation != "background_app" {
		t.Fatalf(
			"observer operations = %q, %q; want background_app",
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

func TestDP110BackgroundAppRemoteFailureDoesNotFallback(t *testing.T) {
	session, recorder := newDP110BackgroundSession(
		t,
		"UiAutomator2",
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(
				`{"value":{"error":"unknown command","message":"background route unavailable"}}`,
			))
		}),
	)

	err := session.BackgroundApp(context.Background())
	if err == nil || !appium.IsErrorCode(err, appium.CodeUnsupported) {
		t.Fatalf("error = %v, want unsupported", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf("delivery = %q, want acknowledged", appium.DeliveryOf(err))
	}
	var clientErr *appium.Error
	if !errors.As(err, &clientErr) {
		t.Fatalf("error type = %T, want *appium.Error", err)
	}
	if clientErr.Operation != "background_app" {
		t.Fatalf("operation = %q, want background_app", clientErr.Operation)
	}
	if requests := recorder.Requests(); len(requests) != 1 {
		t.Fatalf("remote failure must not trigger fallback: got %d requests", len(requests))
	}
}

func TestDP110BackgroundAppUnknownDeliveryIsNotReplayed(t *testing.T) {
	hijacked := make(chan error, 1)
	var calls atomic.Int32
	session, recorder := newDP110BackgroundSession(
		t,
		"XCUITest",
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			calls.Add(1)
			if request.Method != http.MethodPost ||
				request.RequestURI != "/session/session%2Fid/appium/app/background" {
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

	err := session.BackgroundApp(context.Background())
	if hijackErr := <-hijacked; hijackErr != nil {
		t.Fatalf("hijack connection: %v", hijackErr)
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
		t.Fatalf("unknown delivery must not replay request: got %d", len(requests))
	}
}

func TestDP110BackgroundAppCanceledBeforeDelivery(t *testing.T) {
	session, recorder := newDP110BackgroundSession(
		t,
		"XCUITest",
		appium.ClientOptions{},
		http.NotFoundHandler(),
	)
	recorder.Reset()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := session.BackgroundApp(ctx)
	if err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("error = %v, want canceled", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("delivery = %q, want not_sent", appium.DeliveryOf(err))
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("canceled command must not be delivered: got %d", len(requests))
	}
}

func newDP110BackgroundSession(
	t *testing.T,
	automationName string,
	options appium.ClientOptions,
	backgroundHandler http.Handler,
) (*appium.Session, *contracttest.Recorder) {
	t.Helper()

	recorder := contracttest.NewRecorder(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.Method == http.MethodPost && request.RequestURI == "/session" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(
				`{"value":{"sessionId":"session/id","capabilities":{"automationName":"` +
					automationName + `"}}}`,
			))
			return
		}
		backgroundHandler.ServeHTTP(writer, request)
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
		}),
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	recorder.Reset()
	return session, recorder
}
