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

func TestDP111OrientationRoutesReadFreshSnapshotsAndSetTypedValues(t *testing.T) {
	for _, test := range []struct {
		name           string
		automationName string
	}{
		{name: "XCUITest", automationName: "XCUITest"},
		{name: "UiAutomator2", automationName: "UiAutomator2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var reads atomic.Int32
			session, recorder := newDP111OrientationSession(
				t,
				test.automationName,
				appium.ClientOptions{},
				http.HandlerFunc(func(
					writer http.ResponseWriter,
					request *http.Request,
				) {
					if request.RequestURI != "/session/session%2Fid/orientation" {
						http.NotFound(writer, request)
						return
					}

					switch request.Method {
					case http.MethodGet:
						// Deliberately omit Content-Type: GET has no request body and
						// the client must not require or synthesize this header.
						if reads.Add(1) == 1 {
							_, _ = writer.Write([]byte(`{"value":"PORTRAIT"}`))
							return
						}
						_, _ = writer.Write([]byte(`{"value":"LANDSCAPE"}`))

					case http.MethodPost:
						writer.Header().Set("Content-Type", "application/json")
						_, _ = writer.Write([]byte(`{"value":null}`))

					default:
						http.NotFound(writer, request)
					}
				}),
			)

			first, err := session.Orientation(context.Background())
			if err != nil {
				t.Fatalf("first orientation snapshot: %v", err)
			}
			if first != appium.OrientationPortrait {
				t.Fatalf("first orientation = %q, want PORTRAIT", first)
			}

			if err := session.SetOrientation(
				context.Background(),
				appium.OrientationLandscape,
			); err != nil {
				t.Fatalf("set landscape orientation: %v", err)
			}

			second, err := session.Orientation(context.Background())
			if err != nil {
				t.Fatalf("second orientation snapshot: %v", err)
			}
			if second != appium.OrientationLandscape {
				t.Fatalf("second orientation = %q, want LANDSCAPE", second)
			}

			if err := session.SetOrientation(
				context.Background(),
				appium.OrientationPortrait,
			); err != nil {
				t.Fatalf("set portrait orientation: %v", err)
			}

			requests := recorder.Requests()
			if len(requests) != 4 {
				t.Fatalf("request count = %d, want 4", len(requests))
			}

			for _, index := range []int{0, 2} {
				request := requests[index]
				if err := contracttest.MatchMethod(request, http.MethodGet); err != nil {
					t.Fatalf("GET request %d: %v", index, err)
				}
				if err := contracttest.MatchRequestURI(
					request,
					"/session/session%2Fid/orientation",
				); err != nil {
					t.Fatalf("GET request %d: %v", index, err)
				}
				if len(request.Body) != 0 {
					t.Fatalf("GET request %d body = %q, want empty", index, request.Body)
				}
				if contentType := request.Header.Get("Content-Type"); contentType != "" {
					t.Fatalf(
						"GET request %d Content-Type = %q, want absent",
						index,
						contentType,
					)
				}
			}

			for _, expected := range []struct {
				index       int
				orientation appium.Orientation
			}{
				{index: 1, orientation: appium.OrientationLandscape},
				{index: 3, orientation: appium.OrientationPortrait},
			} {
				request := requests[expected.index]
				if err := contracttest.MatchMethod(request, http.MethodPost); err != nil {
					t.Fatalf("POST request %d: %v", expected.index, err)
				}
				if err := contracttest.MatchRequestURI(
					request,
					"/session/session%2Fid/orientation",
				); err != nil {
					t.Fatalf("POST request %d: %v", expected.index, err)
				}
				if err := contracttest.MatchJSONBody(
					request,
					map[string]any{"orientation": expected.orientation},
				); err != nil {
					t.Fatalf("POST request %d body: %v", expected.index, err)
				}
				if err := contracttest.MatchHeader(
					request,
					"Content-Type",
					"application/json",
				); err != nil {
					t.Fatalf("POST request %d header: %v", expected.index, err)
				}
			}
		})
	}
}

func TestDP111SetOrientationRejectsValuesOutsideTypedSet(t *testing.T) {
	for _, test := range []struct {
		name        string
		orientation appium.Orientation
	}{
		{name: "zero", orientation: ""},
		{name: "lowercase", orientation: "portrait"},
		{name: "leading whitespace", orientation: " LANDSCAPE"},
		{name: "trailing whitespace", orientation: "PORTRAIT "},
		{name: "unknown", orientation: "AUTO"},
		{name: "invalid UTF-8", orientation: appium.Orientation(string([]byte{0xff}))},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, recorder := newDP111OrientationSession(
				t,
				"XCUITest",
				appium.ClientOptions{},
				http.NotFoundHandler(),
			)

			err := session.SetOrientation(context.Background(), test.orientation)
			if err == nil || !appium.IsErrorCode(err, appium.CodeInvalidArgument) {
				t.Fatalf("error = %v, want invalid argument", err)
			}
			if appium.DeliveryOf(err) != appium.DeliveryNotSent {
				t.Fatalf("delivery = %q, want not_sent", appium.DeliveryOf(err))
			}

			var clientErr *appium.Error
			if !errors.As(err, &clientErr) {
				t.Fatalf("error type = %T, want *appium.Error", err)
			}
			if clientErr.Operation != "set_orientation" {
				t.Fatalf("operation = %q, want set_orientation", clientErr.Operation)
			}
			if requests := recorder.Requests(); len(requests) != 0 {
				t.Fatalf("invalid orientation sent %d requests, want 0", len(requests))
			}
		})
	}
}

func TestDP111OrientationRejectsInvalidSuccessValues(t *testing.T) {
	for _, test := range []struct {
		name     string
		response string
	}{
		{name: "null", response: "null"},
		{name: "boolean", response: "true"},
		{name: "number", response: "0"},
		{name: "object", response: `{}`},
		{name: "array", response: `[]`},
		{name: "empty", response: `""`},
		{name: "lowercase", response: `"portrait"`},
		{name: "whitespace", response: `"LANDSCAPE "`},
		{name: "unknown", response: `"AUTO"`},
		{name: "unpaired surrogate", response: `"\ud800"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, recorder := newDP111OrientationSession(
				t,
				"UiAutomator2",
				appium.ClientOptions{},
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(`{"value":` + test.response + `}`))
				}),
			)

			orientation, err := session.Orientation(context.Background())
			if orientation != "" {
				t.Fatalf("orientation = %q, want zero value", orientation)
			}
			assertDP111ResponseInvalid(
				t,
				err,
				"get_orientation",
			)
			if requests := recorder.Requests(); len(requests) != 1 {
				t.Fatalf("request count = %d, want 1", len(requests))
			}
		})
	}
}

func TestDP111SetOrientationRejectsNonNullSuccessValues(t *testing.T) {
	for _, test := range []struct {
		name     string
		response string
	}{
		{name: "true", response: "true"},
		{name: "false", response: "false"},
		{name: "number", response: "0"},
		{name: "string", response: `"PORTRAIT"`},
		{name: "object", response: `{}`},
		{name: "array", response: `[]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, recorder := newDP111OrientationSession(
				t,
				"XCUITest",
				appium.ClientOptions{},
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(`{"value":` + test.response + `}`))
				}),
			)

			err := session.SetOrientation(
				context.Background(),
				appium.OrientationPortrait,
			)
			assertDP111ResponseInvalid(
				t,
				err,
				"set_orientation",
			)
			if requests := recorder.Requests(); len(requests) != 1 {
				t.Fatalf("request count = %d, want 1", len(requests))
			}
		})
	}
}

func TestDP111OrientationObserverUsesCanonicalIdentities(t *testing.T) {
	for _, test := range []struct {
		name     string
		identity string
		response string
		invoke   func(*appium.Session) error
	}{
		{
			name:     "get",
			identity: "get_orientation",
			response: `{"value":"AUTO"}`,
			invoke: func(session *appium.Session) error {
				_, err := session.Orientation(context.Background())
				return err
			},
		},
		{
			name:     "set",
			identity: "set_orientation",
			response: `{"value":true}`,
			invoke: func(session *appium.Session) error {
				return session.SetOrientation(
					context.Background(),
					appium.OrientationLandscape,
				)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			observer := &commandObserverRecorder{}
			session, recorder := newDP111OrientationSession(
				t,
				"XCUITest",
				appium.ClientOptions{Observer: observer},
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(test.response))
				}),
			)
			observer.reset()
			recorder.Reset()

			err := test.invoke(session)
			assertDP111ResponseInvalid(t, err, test.identity)

			started, finished := observer.snapshot()
			if len(started) != 1 || len(finished) != 1 {
				t.Fatalf(
					"observer events = started %d, finished %d; want 1, 1",
					len(started),
					len(finished),
				)
			}
			if started[0].Operation != test.identity ||
				finished[0].Operation != test.identity {
				t.Fatalf(
					"observer operations = %q, %q; want %s",
					started[0].Operation,
					finished[0].Operation,
					test.identity,
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
		})
	}
}

func TestDP111SetOrientationRemoteFailureDoesNotFallback(t *testing.T) {
	session, recorder := newDP111OrientationSession(
		t,
		"UiAutomator2",
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(
				`{"value":{"error":"unknown command","message":"orientation route unavailable"}}`,
			))
		}),
	)

	err := session.SetOrientation(
		context.Background(),
		appium.OrientationLandscape,
	)
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
	if clientErr.Operation != "set_orientation" {
		t.Fatalf("operation = %q, want set_orientation", clientErr.Operation)
	}
	if requests := recorder.Requests(); len(requests) != 1 {
		t.Fatalf("remote failure must not trigger fallback: got %d requests", len(requests))
	}
}

func TestDP111SetOrientationUnknownDeliveryIsNotReplayed(t *testing.T) {
	hijacked := make(chan error, 1)
	var calls atomic.Int32
	session, recorder := newDP111OrientationSession(
		t,
		"XCUITest",
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			calls.Add(1)
			if request.Method != http.MethodPost ||
				request.RequestURI != "/session/session%2Fid/orientation" {
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

	err := session.SetOrientation(
		context.Background(),
		appium.OrientationPortrait,
	)
	if hijackErr := <-hijacked; hijackErr != nil {
		t.Fatalf("hijack connection: %v", hijackErr)
	}
	if err == nil || !appium.IsErrorCode(err, appium.CodeTransportFailed) {
		t.Fatalf("error = %v, want transport failure", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryUnknown {
		t.Fatalf("delivery = %q, want unknown", appium.DeliveryOf(err))
	}
	var clientErr *appium.Error
	if !errors.As(err, &clientErr) || clientErr.Operation != "set_orientation" {
		t.Fatalf("error = %#v, want set_orientation *appium.Error", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("handler call count = %d, want 1", calls.Load())
	}
	if requests := recorder.Requests(); len(requests) != 1 {
		t.Fatalf("unknown delivery must not replay orientation request: got %d", len(requests))
	}
}

func TestDP111OrientationCanceledBeforeDelivery(t *testing.T) {
	session, recorder := newDP111OrientationSession(
		t,
		"XCUITest",
		appium.ClientOptions{},
		http.NotFoundHandler(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	orientation, err := session.Orientation(ctx)
	if orientation != "" || err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("Orientation = %q, %v; want zero value and canceled", orientation, err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("Orientation delivery = %q, want not_sent", appium.DeliveryOf(err))
	}

	err = session.SetOrientation(ctx, appium.OrientationPortrait)
	if err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("SetOrientation error = %v, want canceled", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("SetOrientation delivery = %q, want not_sent", appium.DeliveryOf(err))
	}

	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("canceled orientation commands sent %d requests, want 0", len(requests))
	}
}

func assertDP111ResponseInvalid(
	t *testing.T,
	err error,
	operation string,
) {
	t.Helper()

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
	if clientErr.Operation != operation {
		t.Fatalf("operation = %q, want %q", clientErr.Operation, operation)
	}
}

func newDP111OrientationSession(
	t *testing.T,
	automationName string,
	options appium.ClientOptions,
	orientationHandler http.Handler,
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
		orientationHandler.ServeHTTP(writer, request)
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
