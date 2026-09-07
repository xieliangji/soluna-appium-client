package xcuitest_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
	"github.com/xieliangji/soluna-appium-client/xcuitest"
)

type alertLabelCommand struct {
	action    string
	operation string
	run       func(context.Context, *appium.Session, string) error
}

func alertLabelCommands() []alertLabelCommand {
	return []alertLabelCommand{
		{"accept", "ios_accept_alert_with_label", xcuitest.IOSAcceptAlertWithLabel},
		{"dismiss", "ios_dismiss_alert_with_label", xcuitest.IOSDismissAlertWithLabel},
	}
}

func TestIOSAlertWithLabelProtocol(t *testing.T) {
	for _, command := range alertLabelCommands() {
		t.Run(command.action, func(t *testing.T) {
			observer := &operationObserver{}
			session, recorder := newAlertLabelSession(t, "XCUITest", appium.ClientOptions{Observer: observer},
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = w.Write([]byte("{\"value\": \n null \t}"))
				}))
			copy := *session
			for _, label := range []string{"Continue", "continue", "  允许 🧪\t", " \t\n"} {
				t.Run(label, func(t *testing.T) {
					recorder.Reset()
					observer.reset()
					if err := command.run(context.Background(), &copy, label); err != nil {
						t.Fatalf("handle alert by label: %v", err)
					}
					requests := recorder.Requests()
					if len(requests) != 1 {
						t.Fatalf("expected one alert request, got %d", len(requests))
					}
					request := requests[0]
					for _, err := range []error{
						contracttest.MatchMethod(request, http.MethodPost),
						contracttest.MatchRequestURI(request, "/session/alert%2Fsession/execute/sync"),
						contracttest.MatchHeader(request, "Content-Type", "application/json"),
						contracttest.MatchJSONBody(request, map[string]any{
							"script": "mobile: alert",
							"args": []any{map[string]any{
								"action": command.action, "buttonLabel": label,
							}},
						}),
					} {
						if err != nil {
							t.Fatal(err)
						}
					}
					if len(observer.started) != 1 || len(observer.finished) != 1 ||
						observer.started[0].Operation != command.operation || observer.finished[0].Operation != command.operation ||
						observer.finished[0].ErrorCode != "" || observer.finished[0].StatusCode != http.StatusOK ||
						observer.finished[0].Delivery != appium.DeliveryAcknowledged {
						t.Fatalf("unexpected observer events: %+v / %+v", observer.started, observer.finished)
					}
				})
			}
		})
	}
}

func TestIOSAlertWithLabelRejectsLocalArgumentsWithoutRequests(t *testing.T) {
	session, recorder := newAlertLabelSession(t, "XCUITest", appium.ClientOptions{}, http.NotFoundHandler())
	for _, command := range alertLabelCommands() {
		t.Run(command.action, func(t *testing.T) {
			for _, test := range []struct {
				name    string
				session *appium.Session
				label   string
			}{
				{"nil session", nil, "Continue"},
				{"zero session", &appium.Session{}, "Continue"},
				{"empty label", session, ""},
				{"invalid UTF-8", session, string([]byte{'a', 0xff})},
			} {
				t.Run(test.name, func(t *testing.T) {
					err := command.run(context.Background(), test.session, test.label)
					assertAlertLabelError(t, err, command.operation, appium.CodeInvalidArgument, appium.DeliveryNotSent, 0)
					if len(recorder.Requests()) != 0 {
						t.Fatal("local argument failure sent a request")
					}
				})
			}
		})
	}
}

func TestIOSAlertWithLabelUsesConfirmedDriver(t *testing.T) {
	for _, driver := range []string{"UiAutomator2", "xcuitest", "XCUITest ", "CustomDriver"} {
		t.Run(driver, func(t *testing.T) {
			// The creation request asks for XCUITest; the response is authoritative.
			session, recorder := newAlertLabelSession(t, driver, appium.ClientOptions{}, http.NotFoundHandler())
			for _, command := range alertLabelCommands() {
				t.Run(command.action, func(t *testing.T) {
					err := command.run(context.Background(), session, "Continue")
					assertAlertLabelError(t, err, command.operation, appium.CodeUnsupported, appium.DeliveryNotSent, 0)
					if len(recorder.Requests()) != 0 {
						t.Fatal("driver mismatch sent a request")
					}
				})
			}
		})
	}
}

func TestIOSAlertWithLabelRejectsContextAndClosedSessionWithoutRequests(t *testing.T) {
	session, recorder := newAlertLabelSession(t, "XCUITest", appium.ClientOptions{}, http.NotFoundHandler())
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Unix(0, 0))
	t.Cleanup(expire)
	for _, command := range alertLabelCommands() {
		t.Run(command.action, func(t *testing.T) {
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
					err := command.run(test.ctx, session, "Continue")
					assertAlertLabelError(t, err, command.operation, test.code, appium.DeliveryNotSent, 0)
					if test.cause != nil && !errors.Is(err, test.cause) {
						t.Fatalf("context cause was lost: %v", err)
					}
					if len(recorder.Requests()) != 0 {
						t.Fatal("invalid context sent a request")
					}
				})
			}
		})
	}
	copy := *session
	if err := session.Close(context.Background()); err != nil {
		t.Fatalf("close session: %v", err)
	}
	recorder.Reset()
	for _, command := range alertLabelCommands() {
		t.Run("closed "+command.action, func(t *testing.T) {
			err := command.run(context.Background(), &copy, "Continue")
			assertAlertLabelError(t, err, command.operation, appium.CodeSessionLost, appium.DeliveryNotSent, 0)
			if len(recorder.Requests()) != 0 {
				t.Fatal("closed session sent a request")
			}
		})
	}
}

func TestIOSAlertWithLabelErrorsStayInCommandChain(t *testing.T) {
	for _, command := range alertLabelCommands() {
		t.Run(command.action, func(t *testing.T) {
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
				{"no alert", `{"value":{"error":"no such alert","message":"missing"}}`, 404, appium.CodeAlertNotFound, "no such alert"},
				{"unmatched label", `{"value":{"error":"invalid element state","message":"no button has this label"}}`, 400, appium.CodeCommandFailed, "invalid element state"},
				{"invalid argument", `{"value":{"error":"invalid argument","message":"invalid parameters"}}`, 400, appium.CodeInvalidArgument, "invalid argument"},
				{"unsupported", `{"value":{"error":"unknown command","message":"unavailable"}}`, 404, appium.CodeUnsupported, "unknown command"},
				{"session lost", `{"value":{"error":"invalid session id","message":"missing"}}`, 404, appium.CodeSessionLost, "invalid session id"},
				{"remote failure", `{"value":{"error":"unknown error","message":"failed"}}`, 500, appium.CodeCommandFailed, "unknown error"},
			} {
				t.Run(test.name, func(t *testing.T) {
					observer := &operationObserver{}
					session, recorder := newAlertLabelSession(t, "XCUITest", appium.ClientOptions{Observer: observer},
						http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(test.status)
							_, _ = w.Write([]byte(test.body))
						}))
					observer.reset()
					err := command.run(context.Background(), session, "Continue")
					commandErr := assertAlertLabelError(t, err, command.operation, test.code, appium.DeliveryAcknowledged, test.status)
					if commandErr.RemoteCode != test.remoteCode {
						t.Fatalf("remote code: got %q, want %q", commandErr.RemoteCode, test.remoteCode)
					}
					if len(recorder.Requests()) != 1 {
						t.Fatal("alert failure retried or probed")
					}
					if len(observer.started) != 1 || len(observer.finished) != 1 {
						t.Fatalf("unexpected observer event counts: %+v / %+v", observer.started, observer.finished)
					}
					finished := observer.finished[0]
					if observer.started[0].Operation != command.operation || finished.Operation != command.operation ||
						finished.ErrorCode != commandErr.Code || finished.StatusCode != commandErr.StatusCode || finished.Delivery != commandErr.Delivery {
						t.Fatalf("observer disagrees with caller error: %+v / %+v", finished, commandErr)
					}
				})
			}
		})
	}
}

func TestIOSAlertWithLabelHonorsResponseLimit(t *testing.T) {
	for _, command := range alertLabelCommands() {
		t.Run(command.action, func(t *testing.T) {
			session, recorder := newAlertLabelSession(t, "XCUITest", appium.ClientOptions{Limits: appium.Limits{MaxResponseBytes: 512}},
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = w.Write([]byte(`{"value":null,"padding":"` + strings.Repeat("x", 600) + `"}`))
				}))
			err := command.run(context.Background(), session, "Continue")
			assertAlertLabelError(t, err, command.operation, appium.CodeResponseTooLarge, appium.DeliveryAcknowledged, http.StatusOK)
			if len(recorder.Requests()) != 1 {
				t.Fatal("oversized response caused additional requests")
			}
		})
	}
}

func TestIOSAlertWithLabelDoesNotReplayAfterCancellation(t *testing.T) {
	for _, command := range alertLabelCommands() {
		t.Run(command.action, func(t *testing.T) {
			entered := make(chan struct{})
			session, recorder := newAlertLabelSession(t, "XCUITest", appium.ClientOptions{},
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					close(entered)
					<-r.Context().Done()
				}))
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			result := make(chan error, 1)
			go func() {
				result <- command.run(ctx, session, "Continue")
			}()
			<-entered
			cancel()
			err := <-result
			assertAlertLabelError(t, err, command.operation, appium.CodeCanceled, appium.DeliveryUnknown, 0)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation cause was lost: %v", err)
			}
			if len(recorder.Requests()) != 1 {
				t.Fatal("uncertain alert delivery caused additional requests")
			}
		})
	}
}

func TestIOSAlertWithLabelRejectsCleanupOnlySession(t *testing.T) {
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
	for _, command := range alertLabelCommands() {
		t.Run(command.action, func(t *testing.T) {
			err := command.run(context.Background(), session, "Continue")
			assertAlertLabelError(t, err, command.operation, appium.CodeInvalidArgument, appium.DeliveryNotSent, 0)
			if len(recorder.Requests()) != 0 {
				t.Fatal("cleanup-only session sent an alert request")
			}
		})
	}
}

func assertAlertLabelError(t *testing.T, err error, operation string, code appium.ErrorCode, delivery appium.DeliveryState, status int) *appium.Error {
	t.Helper()
	var commandErr *appium.Error
	if !errors.As(err, &commandErr) || commandErr == nil {
		t.Fatalf("expected structured alert label error, got %v", err)
	}
	if commandErr.Operation != operation || commandErr.Code != code || commandErr.Delivery != delivery || commandErr.StatusCode != status {
		t.Fatalf("unexpected alert label error: %+v; want %s / %s / %s / %d", commandErr, operation, code, delivery, status)
	}
	return commandErr
}

func newAlertLabelSession(t *testing.T, driver string, options appium.ClientOptions, alert http.Handler) (*appium.Session, *contracttest.Recorder) {
	t.Helper()
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.RequestURI == "/session":
			_, _ = w.Write([]byte(`{"value":{"sessionId":"alert/session","capabilities":{"automationName":"` + driver + `"}}}`))
		case r.Method == http.MethodDelete && r.RequestURI == "/session/alert%2Fsession":
			_, _ = w.Write([]byte(`{"value":null}`))
		case r.Method == http.MethodPost && r.RequestURI == "/session/alert%2Fsession/execute/sync":
			alert.ServeHTTP(w, r)
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
	recorder.Reset()
	return session, recorder
}
