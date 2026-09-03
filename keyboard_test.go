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

func TestDP101KeyboardRoutesPreserveDriverBooleans(t *testing.T) {
	for _, test := range []struct {
		name           string
		automationName string
	}{
		{name: "XCUITest", automationName: "XCUITest"},
		{name: "UiAutomator2", automationName: "UiAutomator2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var shownCalls atomic.Int32
			var dismissCalls atomic.Int32
			session, recorder := newDP101KeyboardSession(
				t,
				test.automationName,
				appium.ClientOptions{},
				http.HandlerFunc(func(
					writer http.ResponseWriter,
					request *http.Request,
				) {
					switch {
					case request.Method == http.MethodGet &&
						request.RequestURI == "/session/session%2Fid/appium/device/is_keyboard_shown":
						// Deliberately omit Content-Type: GET has no request body and
						// the client must not require or synthesize this header.
						if shownCalls.Add(1) == 1 {
							_, _ = writer.Write([]byte(`{"value":true}`))
							return
						}
						_, _ = writer.Write([]byte(`{"value":false}`))

					case request.Method == http.MethodPost &&
						request.RequestURI == "/session/session%2Fid/appium/device/hide_keyboard":
						writer.Header().Set("Content-Type", "application/json")
						if dismissCalls.Add(1) == 1 {
							_, _ = writer.Write([]byte(`{"value":false}`))
							return
						}
						_, _ = writer.Write([]byte(`{"value":true}`))

					default:
						http.NotFound(writer, request)
					}
				}),
			)

			first, err := session.KeyboardShown(context.Background())
			if err != nil {
				t.Fatalf("first keyboard snapshot: %v", err)
			}
			if !first {
				t.Fatal("first keyboard snapshot = false, want true")
			}

			second, err := session.KeyboardShown(context.Background())
			if err != nil {
				t.Fatalf("second keyboard snapshot: %v", err)
			}
			if second {
				t.Fatal("second keyboard snapshot = true, want false")
			}

			driverReported, err := session.DismissKeyboard(context.Background())
			if err != nil {
				t.Fatalf("dismiss keyboard: %v", err)
			}
			if driverReported {
				t.Fatal("driver-reported dismissal = true, want false")
			}
			driverReported, err = session.DismissKeyboard(context.Background())
			if err != nil {
				t.Fatalf("second dismiss keyboard: %v", err)
			}
			if !driverReported {
				t.Fatal("second driver-reported dismissal = false, want true")
			}

			requests := recorder.Requests()
			if len(requests) != 4 {
				t.Fatalf("request count = %d, want 4", len(requests))
			}

			for index := 0; index < 2; index++ {
				if err := contracttest.MatchMethod(requests[index], http.MethodGet); err != nil {
					t.Fatalf("request %d: %v", index, err)
				}
				if err := contracttest.MatchRequestURI(
					requests[index],
					"/session/session%2Fid/appium/device/is_keyboard_shown",
				); err != nil {
					t.Fatalf("request %d: %v", index, err)
				}
				if len(requests[index].Body) != 0 {
					t.Fatalf("GET request %d body = %q, want empty", index, requests[index].Body)
				}
				if contentType := requests[index].Header.Get("Content-Type"); contentType != "" {
					t.Fatalf("GET request %d Content-Type = %q, want absent", index, contentType)
				}
			}

			for index := 2; index < 4; index++ {
				if err := contracttest.MatchMethod(requests[index], http.MethodPost); err != nil {
					t.Fatalf("dismiss request %d: %v", index, err)
				}
				if err := contracttest.MatchRequestURI(
					requests[index],
					"/session/session%2Fid/appium/device/hide_keyboard",
				); err != nil {
					t.Fatalf("dismiss request %d: %v", index, err)
				}
				if err := contracttest.MatchJSONBody(requests[index], map[string]any{}); err != nil {
					t.Fatalf("dismiss request %d body: %v", index, err)
				}
				if err := contracttest.MatchHeader(requests[index], "Content-Type", "application/json"); err != nil {
					t.Fatalf("dismiss request %d header: %v", index, err)
				}
			}
		})
	}
}

func TestDP101KeyboardRejectsNonBooleanSuccessValues(t *testing.T) {
	for _, test := range []struct {
		name     string
		response string
	}{
		{name: "null", response: "null"},
		{name: "number", response: "0"},
		{name: "string", response: `"false"`},
		{name: "object", response: `{}`},
		{name: "array", response: `[]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, command := range []struct {
				name     string
				method   string
				path     string
				invoke   func(*appium.Session) (bool, error)
				identity string
			}{
				{
					name:     "shown",
					method:   http.MethodGet,
					path:     "is_keyboard_shown",
					identity: "keyboard_shown",
					invoke: func(session *appium.Session) (bool, error) {
						return session.KeyboardShown(context.Background())
					},
				},
				{
					name:     "dismiss",
					method:   http.MethodPost,
					path:     "hide_keyboard",
					identity: "dismiss_keyboard",
					invoke: func(session *appium.Session) (bool, error) {
						return session.DismissKeyboard(context.Background())
					},
				},
			} {
				t.Run(command.name, func(t *testing.T) {
					session, recorder := newDP101KeyboardSession(
						t,
						"XCUITest",
						appium.ClientOptions{},
						http.HandlerFunc(func(
							writer http.ResponseWriter,
							request *http.Request,
						) {
							if request.Method != command.method ||
								request.RequestURI != "/session/session%2Fid/appium/device/"+command.path {
								http.NotFound(writer, request)
								return
							}
							writer.Header().Set("Content-Type", "application/json")
							_, _ = writer.Write([]byte(`{"value":` + test.response + `}`))
						}),
					)

					result, err := command.invoke(session)
					if result {
						t.Fatalf("invalid response returned true")
					}
					if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
						t.Fatalf("error = %v, want response invalid", err)
					}
					if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
						t.Fatalf("delivery = %q, want acknowledged", appium.DeliveryOf(err))
					}

					var clientErr *appium.Error
					if !errors.As(err, &clientErr) {
						t.Fatalf("error type = %T, want *appium.Error", err)
					}
					if clientErr.Operation != command.identity {
						t.Fatalf("operation = %q, want %q", clientErr.Operation, command.identity)
					}
					if requests := recorder.Requests(); len(requests) != 1 {
						t.Fatalf("request count = %d, want 1", len(requests))
					}
				})
			}
		})
	}
}

func TestDP101KeyboardObserverUsesCanonicalIdentityOnDecodeFailure(t *testing.T) {
	observer := &commandObserverRecorder{}
	session, recorder := newDP101KeyboardSession(
		t,
		"XCUITest",
		appium.ClientOptions{Observer: observer},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"value":null}`))
		}),
	)
	observer.reset()
	recorder.Reset()

	_, err := session.KeyboardShown(context.Background())
	if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
		t.Fatalf("error = %v, want response invalid", err)
	}

	started, finished := observer.snapshot()
	if len(started) != 1 || len(finished) != 1 {
		t.Fatalf("observer events = started %d, finished %d; want 1, 1", len(started), len(finished))
	}
	if started[0].Operation != "keyboard_shown" || finished[0].Operation != "keyboard_shown" {
		t.Fatalf(
			"observer operations = %q, %q; want keyboard_shown",
			started[0].Operation,
			finished[0].Operation,
		)
	}
	if finished[0].ErrorCode != appium.CodeResponseInvalid {
		t.Fatalf("finished error code = %q, want response_invalid", finished[0].ErrorCode)
	}
	if finished[0].Delivery != appium.DeliveryAcknowledged {
		t.Fatalf("finished delivery = %q, want acknowledged", finished[0].Delivery)
	}
}

func TestDP101DismissKeyboardRemoteFailureDoesNotFallback(t *testing.T) {
	session, recorder := newDP101KeyboardSession(
		t,
		"UiAutomator2",
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(
				`{"value":{"error":"unknown command","message":"keyboard route unavailable"}}`,
			))
		}),
	)

	result, err := session.DismissKeyboard(context.Background())
	if result {
		t.Fatal("remote failure returned true")
	}
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
	if clientErr.Operation != "dismiss_keyboard" {
		t.Fatalf("operation = %q, want dismiss_keyboard", clientErr.Operation)
	}
	if requests := recorder.Requests(); len(requests) != 1 {
		t.Fatalf("remote failure must not trigger fallback: got %d requests", len(requests))
	}
}

func TestDP101DismissKeyboardUnknownDeliveryIsNotReplayed(t *testing.T) {
	hijacked := make(chan error, 1)
	session, recorder := newDP101KeyboardSession(
		t,
		"XCUITest",
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodPost ||
				request.RequestURI != "/session/session%2Fid/appium/device/hide_keyboard" {
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

	result, err := session.DismissKeyboard(context.Background())
	if hijackErr := <-hijacked; hijackErr != nil {
		t.Fatalf("hijack connection: %v", hijackErr)
	}
	if result {
		t.Fatal("unknown delivery returned true")
	}
	if err == nil || !appium.IsErrorCode(err, appium.CodeTransportFailed) {
		t.Fatalf("error = %v, want transport failure", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryUnknown {
		t.Fatalf("delivery = %q, want unknown", appium.DeliveryOf(err))
	}
	if requests := recorder.Requests(); len(requests) != 1 {
		t.Fatalf("unknown delivery must not replay dismiss request: got %d", len(requests))
	}
}

func TestDP101KeyboardCanceledBeforeDelivery(t *testing.T) {
	session, recorder := newDP101KeyboardSession(
		t,
		"XCUITest",
		appium.ClientOptions{},
		http.NotFoundHandler(),
	)
	recorder.Reset()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	shown, err := session.KeyboardShown(ctx)
	if shown || err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("KeyboardShown = %v, %v; want false and canceled", shown, err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("KeyboardShown delivery = %q, want not_sent", appium.DeliveryOf(err))
	}

	dismissed, err := session.DismissKeyboard(ctx)
	if dismissed || err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("DismissKeyboard = %v, %v; want false and canceled", dismissed, err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("DismissKeyboard delivery = %q, want not_sent", appium.DeliveryOf(err))
	}

	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("canceled keyboard commands must not be delivered: got %d", len(requests))
	}
}

func newDP101KeyboardSession(
	t *testing.T,
	automationName string,
	options appium.ClientOptions,
	keyboardHandler http.Handler,
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
		keyboardHandler.ServeHTTP(writer, request)
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
