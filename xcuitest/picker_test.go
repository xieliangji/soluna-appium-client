package xcuitest_test

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
	"github.com/xieliangji/soluna-appium-client/xcuitest"
)

const pickerOperation = "ios_select_picker_wheel_value"

func TestIOSSelectPickerWheelValueProtocol(t *testing.T) {
	observer := &operationObserver{}
	session, element, recorder := newPickerSession(t, "XCUITest", appium.ClientOptions{Observer: observer},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("{\"value\": \n null \t}"))
		}))
	sessionCopy, elementCopy := *session, *element
	for _, test := range []struct {
		name      string
		direction xcuitest.PickerWheelDirection
		offset    xcuitest.PickerWheelOffset
	}{
		{"next", xcuitest.PickerWheelNext, 0.2},
		{"previous at maximum offset", xcuitest.PickerWheelPrevious, 0.5},
		{"small positive offset", xcuitest.PickerWheelNext, 0.005},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder.Reset()
			observer.reset()
			if err := xcuitest.IOSSelectPickerWheelValue(context.Background(), &sessionCopy, &elementCopy, test.direction, test.offset); err != nil {
				t.Fatalf("select picker wheel value: %v", err)
			}
			requests := recorder.Requests()
			if len(requests) != 1 {
				t.Fatalf("expected one selection request, got %d", len(requests))
			}
			request := requests[0]
			for _, err := range []error{
				contracttest.MatchMethod(request, http.MethodPost),
				contracttest.MatchRequestURI(request, "/session/picker%2Fsession/execute/sync"),
				contracttest.MatchHeader(request, "Content-Type", "application/json"),
				contracttest.MatchJSONBody(request, map[string]any{
					"script": "mobile: selectPickerWheelValue",
					"args": []any{map[string]any{
						"elementId": "wheel/id", "order": string(test.direction), "offset": float64(test.offset),
					}},
				}),
			} {
				if err != nil {
					t.Fatal(err)
				}
			}
			if len(observer.started) != 1 || len(observer.finished) != 1 ||
				observer.started[0].Operation != pickerOperation || observer.finished[0].Operation != pickerOperation ||
				observer.finished[0].ErrorCode != "" {
				t.Fatalf("unexpected observer events: %+v / %+v", observer.started, observer.finished)
			}
		})
	}
}

func TestIOSSelectPickerWheelValueRejectsLocalArgumentsWithoutRequests(t *testing.T) {
	session, element, recorder := newPickerSession(t, "XCUITest", appium.ClientOptions{}, http.NotFoundHandler())
	otherSession, foreignElement, otherRecorder := newPickerSession(t, "XCUITest", appium.ClientOptions{}, http.NotFoundHandler())
	if session.ID() != otherSession.ID() || element.ID() != foreignElement.ID() {
		t.Fatal("fixture must use colliding IDs across endpoints")
	}
	for _, test := range []struct {
		name      string
		session   *appium.Session
		element   *appium.Element
		direction xcuitest.PickerWheelDirection
		offset    xcuitest.PickerWheelOffset
	}{
		{"nil session", nil, element, xcuitest.PickerWheelNext, 0.2},
		{"zero session", &appium.Session{}, element, xcuitest.PickerWheelNext, 0.2},
		{"nil element", session, nil, xcuitest.PickerWheelNext, 0.2},
		{"zero element", session, &appium.Element{}, xcuitest.PickerWheelNext, 0.2},
		{"foreign element with same IDs", session, foreignElement, xcuitest.PickerWheelNext, 0.2},
		{"empty direction", session, element, "", 0.2},
		{"uppercase direction", session, element, "NEXT", 0.2},
		{"padded direction", session, element, "next ", 0.2},
		{"unknown direction", session, element, "up", 0.2},
		{"zero offset", session, element, xcuitest.PickerWheelNext, 0},
		{"negative offset", session, element, xcuitest.PickerWheelNext, -0.1},
		{"above maximum", session, element, xcuitest.PickerWheelNext, xcuitest.PickerWheelOffset(math.Nextafter(0.5, 1))},
		{"NaN", session, element, xcuitest.PickerWheelNext, xcuitest.PickerWheelOffset(math.NaN())},
		{"positive infinity", session, element, xcuitest.PickerWheelNext, xcuitest.PickerWheelOffset(math.Inf(1))},
		{"negative infinity", session, element, xcuitest.PickerWheelNext, xcuitest.PickerWheelOffset(math.Inf(-1))},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := xcuitest.IOSSelectPickerWheelValue(context.Background(), test.session, test.element, test.direction, test.offset)
			assertPickerError(t, err, appium.CodeInvalidArgument, appium.DeliveryNotSent, 0)
			if len(recorder.Requests()) != 0 || len(otherRecorder.Requests()) != 0 {
				t.Fatal("local argument failure sent a request")
			}
		})
	}
}

func TestIOSSelectPickerWheelValueUsesConfirmedDriver(t *testing.T) {
	for _, driver := range []string{"UiAutomator2", "xcuitest", "XCUITest ", "CustomDriver"} {
		t.Run(driver, func(t *testing.T) {
			// The creation request asks for XCUITest; the response is authoritative.
			session, element, recorder := newPickerSession(t, driver, appium.ClientOptions{}, http.NotFoundHandler())
			err := xcuitest.IOSSelectPickerWheelValue(context.Background(), session, element, xcuitest.PickerWheelNext, 0.2)
			assertPickerError(t, err, appium.CodeUnsupported, appium.DeliveryNotSent, 0)
			if len(recorder.Requests()) != 0 {
				t.Fatal("driver mismatch sent a request")
			}
		})
	}
}

func TestIOSSelectPickerWheelValueRejectsContextAndClosedSessionWithoutRequests(t *testing.T) {
	session, element, recorder := newPickerSession(t, "XCUITest", appium.ClientOptions{}, http.NotFoundHandler())
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Unix(0, 0))
	t.Cleanup(expire)
	for _, test := range []struct {
		name  string
		ctx   context.Context
		code  appium.ErrorCode
		cause error
	}{
		{"nil context", nil, appium.CodeInvalidArgument, nil},
		{"canceled", canceled, appium.CodeCanceled, context.Canceled},
		{"expired", expired, appium.CodeDeadlineExceeded, context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := xcuitest.IOSSelectPickerWheelValue(test.ctx, session, element, xcuitest.PickerWheelNext, 0.2)
			assertPickerError(t, err, test.code, appium.DeliveryNotSent, 0)
			if test.cause != nil && !errors.Is(err, test.cause) {
				t.Fatalf("context cause was lost: %v", err)
			}
			if len(recorder.Requests()) != 0 {
				t.Fatal("invalid context sent a request")
			}
		})
	}
	copy := *session
	if err := session.Close(context.Background()); err != nil {
		t.Fatalf("close session: %v", err)
	}
	recorder.Reset()
	err := xcuitest.IOSSelectPickerWheelValue(context.Background(), &copy, element, xcuitest.PickerWheelNext, 0.2)
	assertPickerError(t, err, appium.CodeSessionLost, appium.DeliveryNotSent, 0)
	if len(recorder.Requests()) != 0 {
		t.Fatal("closed session sent a request")
	}
}

func TestIOSSelectPickerWheelValueErrorsStayInCommandChain(t *testing.T) {
	for _, test := range []struct {
		name       string
		body       string
		status     int
		code       appium.ErrorCode
		remoteCode string
	}{
		{"boolean", `{"value":true}`, 200, appium.CodeResponseInvalid, ""},
		{"number", `{"value":0}`, 200, appium.CodeResponseInvalid, ""},
		{"string", `{"value":"null"}`, 200, appium.CodeResponseInvalid, ""},
		{"object", `{"value":{}}`, 200, appium.CodeResponseInvalid, ""},
		{"array", `{"value":[]}`, 200, appium.CodeResponseInvalid, ""},
		{"missing value", `{}`, 200, appium.CodeResponseInvalid, ""},
		{"malformed envelope", `{"value":`, 200, appium.CodeResponseInvalid, ""},
		{"stale", `{"value":{"error":"stale element reference","message":"stale"}}`, 404, appium.CodeElementStale, "stale element reference"},
		{"missing element", `{"value":{"error":"no such element","message":"missing"}}`, 404, appium.CodeElementNotFound, "no such element"},
		{"session lost", `{"value":{"error":"invalid session id","message":"missing"}}`, 404, appium.CodeSessionLost, "invalid session id"},
		{"unsupported", `{"value":{"error":"unknown command","message":"unavailable"}}`, 404, appium.CodeUnsupported, "unknown command"},
		{"wrong element type", `{"value":{"error":"invalid argument","message":"not a picker wheel"}}`, 400, appium.CodeInvalidArgument, "invalid argument"},
		{"no value change", `{"value":{"error":"invalid element state","message":"no change"}}`, 400, appium.CodeCommandFailed, "invalid element state"},
	} {
		t.Run(test.name, func(t *testing.T) {
			observer := &operationObserver{}
			session, element, recorder := newPickerSession(t, "XCUITest", appium.ClientOptions{Observer: observer},
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(test.status)
					_, _ = w.Write([]byte(test.body))
				}))
			observer.reset()
			err := xcuitest.IOSSelectPickerWheelValue(context.Background(), session, element, xcuitest.PickerWheelNext, 0.2)
			commandErr := assertPickerError(t, err, test.code, appium.DeliveryAcknowledged, test.status)
			if commandErr.RemoteCode != test.remoteCode {
				t.Fatalf("remote code: got %q, want %q", commandErr.RemoteCode, test.remoteCode)
			}
			if len(recorder.Requests()) != 1 {
				t.Fatal("selection failure retried or probed")
			}
			if len(observer.started) != 1 || len(observer.finished) != 1 {
				t.Fatalf("unexpected observer event counts: %+v / %+v", observer.started, observer.finished)
			}
			finished := observer.finished[0]
			if observer.started[0].Operation != pickerOperation || finished.Operation != pickerOperation ||
				finished.ErrorCode != commandErr.Code || finished.StatusCode != commandErr.StatusCode || finished.Delivery != commandErr.Delivery {
				t.Fatalf("observer disagrees with caller error: %+v / %+v", finished, commandErr)
			}
		})
	}
}

func TestIOSSelectPickerWheelValueHonorsResponseLimit(t *testing.T) {
	session, element, recorder := newPickerSession(t, "XCUITest", appium.ClientOptions{Limits: appium.Limits{MaxResponseBytes: 512}},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"value":null,"padding":"` + strings.Repeat("x", 600) + `"}`))
		}))
	err := xcuitest.IOSSelectPickerWheelValue(context.Background(), session, element, xcuitest.PickerWheelNext, 0.2)
	assertPickerError(t, err, appium.CodeResponseTooLarge, appium.DeliveryAcknowledged, http.StatusOK)
	if len(recorder.Requests()) != 1 {
		t.Fatal("oversized response caused additional requests")
	}
}

func TestIOSSelectPickerWheelValueDoesNotReplayAfterCancellation(t *testing.T) {
	entered := make(chan struct{})
	session, element, recorder := newPickerSession(t, "XCUITest", appium.ClientOptions{},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(entered)
			<-r.Context().Done()
		}))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result := make(chan error, 1)
	go func() {
		result <- xcuitest.IOSSelectPickerWheelValue(ctx, session, element, xcuitest.PickerWheelNext, 0.2)
	}()
	<-entered
	cancel()
	err := <-result
	assertPickerError(t, err, appium.CodeCanceled, appium.DeliveryUnknown, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation cause was lost: %v", err)
	}
	if len(recorder.Requests()) != 1 {
		t.Fatal("uncertain selection delivery caused additional requests")
	}
}

func TestIOSSelectPickerWheelValueRejectsCleanupOnlySession(t *testing.T) {
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.RequestURI == "/session" {
			// Preserve the remote ID while failing usable-session initialization.
			_, _ = w.Write([]byte(`{"value":{"sessionId":"cleanup/session","capabilities":null}}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"value":{"error":"unknown error","message":"cleanup failed"}}`))
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{}))
	if err == nil || session == nil {
		t.Fatalf("expected cleanup-only session and creation error, got %v / %v", session, err)
	}
	recorder.Reset()
	err = xcuitest.IOSSelectPickerWheelValue(context.Background(), session, nil, xcuitest.PickerWheelNext, 0.2)
	assertPickerError(t, err, appium.CodeInvalidArgument, appium.DeliveryNotSent, 0)
	if len(recorder.Requests()) != 0 {
		t.Fatal("cleanup-only session sent a selection request")
	}
}

func assertPickerError(t *testing.T, err error, code appium.ErrorCode, delivery appium.DeliveryState, status int) *appium.Error {
	t.Helper()
	var commandErr *appium.Error
	if !errors.As(err, &commandErr) || commandErr == nil {
		t.Fatalf("expected structured picker error, got %v", err)
	}
	if commandErr.Operation != pickerOperation || commandErr.Code != code || commandErr.Delivery != delivery || commandErr.StatusCode != status {
		t.Fatalf("unexpected picker error: %+v; want %s / %s / %d", commandErr, code, delivery, status)
	}
	return commandErr
}

func newPickerSession(t *testing.T, driver string, options appium.ClientOptions, selection http.Handler) (*appium.Session, *appium.Element, *contracttest.Recorder) {
	t.Helper()
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.RequestURI == "/session":
			_, _ = w.Write([]byte(`{"value":{"sessionId":"picker/session","capabilities":{"automationName":"` + driver + `"}}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/session/picker%2Fsession/context":
			_, _ = w.Write([]byte(`{"value":"NATIVE_APP"}`))
		case r.Method == http.MethodPost && r.RequestURI == "/session/picker%2Fsession/elements":
			_, _ = w.Write([]byte(`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"wheel/id"}]}`))
		case r.Method == http.MethodGet && r.RequestURI == "/session/picker%2Fsession/window/rect":
			_, _ = w.Write([]byte(`{"value":{"x":0,"y":0,"width":100,"height":100}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/session/picker%2Fsession/element/wheel%2Fid/rect":
			_, _ = w.Write([]byte(`{"value":{"x":10,"y":10,"width":40,"height":40}}`))
		case r.Method == http.MethodDelete && r.RequestURI == "/session/picker%2Fsession":
			_, _ = w.Write([]byte(`{"value":null}`))
		case r.Method == http.MethodPost && r.RequestURI == "/session/picker%2Fsession/execute/sync":
			selection.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
	client, err := server.NewClient(options)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{"appium:automationName": "XCUITest"}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	element, err := session.Find(context.Background(), appium.IOSClassChain("**/XCUIElementTypePickerWheel"))
	if err != nil {
		t.Fatalf("find picker wheel: %v", err)
	}
	recorder.Reset()
	return session, element, recorder
}
