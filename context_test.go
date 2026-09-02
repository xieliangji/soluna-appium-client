package appium_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestSessionContextCommandsUseAppiumRoutesAndFreshSnapshots(t *testing.T) {
	var contextsCalls atomic.Int32
	var currentCalls atomic.Int32

	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch {
			case request.Method == http.MethodGet &&
				request.RequestURI == "/session/session%2Fid/contexts":
				if contextsCalls.Add(1) == 1 {
					_, _ = writer.Write([]byte(
						`{"value":["","WEBVIEW_app","WEBVIEW_app","NATIVE_APP"]}`,
					))
					return
				}
				_, _ = writer.Write([]byte(`{"value":[]}`))

			case request.Method == http.MethodGet &&
				request.RequestURI == "/session/session%2Fid/context":
				if currentCalls.Add(1) == 1 {
					_, _ = writer.Write([]byte(`{"value":"NATIVE_APP"}`))
					return
				}
				_, _ = writer.Write([]byte(`{"value":"CHROMIUM"}`))

			case request.Method == http.MethodPost &&
				request.RequestURI == "/session/session%2Fid/context":
				_, _ = writer.Write([]byte(`{"value":null}`))

			default:
				http.NotFound(writer, request)
			}
		}),
	)

	firstContexts, err := session.Contexts(context.Background())
	if err != nil {
		t.Fatalf("get first contexts snapshot: %v", err)
	}
	expectedFirst := []string{"", "WEBVIEW_app", "WEBVIEW_app", "NATIVE_APP"}
	if !equalStrings(firstContexts, expectedFirst) {
		t.Fatalf("first contexts = %#v, want %#v", firstContexts, expectedFirst)
	}

	secondContexts, err := session.Contexts(context.Background())
	if err != nil {
		t.Fatalf("get second contexts snapshot: %v", err)
	}
	if secondContexts == nil || len(secondContexts) != 0 {
		t.Fatalf("second contexts must be a non-nil empty slice: %#v", secondContexts)
	}

	current, err := session.CurrentContext(context.Background())
	if err != nil {
		t.Fatalf("get first current context: %v", err)
	}
	if current != "NATIVE_APP" {
		t.Fatalf("first current context = %q, want %q", current, "NATIVE_APP")
	}

	if err := session.SwitchContext(
		context.Background(),
		" WEBVIEW/应用 ",
	); err != nil {
		t.Fatalf("switch context: %v", err)
	}
	if err := session.SwitchContext(context.Background(), ""); err != nil {
		t.Fatalf("switch to empty context name: %v", err)
	}

	current, err = session.CurrentContext(context.Background())
	if err != nil {
		t.Fatalf("get second current context: %v", err)
	}
	if current != "CHROMIUM" {
		t.Fatalf("second current context = %q, want %q", current, "CHROMIUM")
	}

	requests := recorder.Requests()
	if len(requests) != 6 {
		t.Fatalf("request count = %d, want 6", len(requests))
	}

	expected := []struct {
		method string
		uri    string
		body   any
	}{
		{http.MethodGet, "/session/session%2Fid/contexts", nil},
		{http.MethodGet, "/session/session%2Fid/contexts", nil},
		{http.MethodGet, "/session/session%2Fid/context", nil},
		{http.MethodPost, "/session/session%2Fid/context", map[string]any{"name": " WEBVIEW/应用 "}},
		{http.MethodPost, "/session/session%2Fid/context", map[string]any{"name": ""}},
		{http.MethodGet, "/session/session%2Fid/context", nil},
	}
	for index, want := range expected {
		request := requests[index]
		if err := contracttest.MatchMethod(request, want.method); err != nil {
			t.Fatalf("request %d: %v", index, err)
		}
		if err := contracttest.MatchRequestURI(request, want.uri); err != nil {
			t.Fatalf("request %d: %v", index, err)
		}
		if want.body == nil {
			if len(request.Body) != 0 {
				t.Fatalf("request %d body = %q, want empty", index, request.Body)
			}
			if contentType := request.Header.Get("Content-Type"); contentType != "" {
				t.Fatalf("request %d Content-Type = %q, want absent", index, contentType)
			}
			continue
		}
		if err := contracttest.MatchJSONBody(request, want.body); err != nil {
			t.Fatalf("request %d: %v", index, err)
		}
		if err := contracttest.MatchHeader(
			request,
			"Content-Type",
			"application/json",
		); err != nil {
			t.Fatalf("request %d: %v", index, err)
		}
	}
}

func TestSessionContextCommandsRejectInvalidSuccessValues(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		uri      string
		response string
		invoke   func(*appium.Session) error
	}{
		{
			name:     "contexts null",
			method:   http.MethodGet,
			uri:      "/session/session/contexts",
			response: `null`,
			invoke: func(session *appium.Session) error {
				contexts, err := session.Contexts(context.Background())
				if contexts != nil {
					t.Fatalf("invalid contexts returned partial result: %#v", contexts)
				}
				return err
			},
		},
		{
			name:     "contexts object",
			method:   http.MethodGet,
			uri:      "/session/session/contexts",
			response: `{}`,
			invoke: func(session *appium.Session) error {
				_, err := session.Contexts(context.Background())
				return err
			},
		},
		{
			name:     "contexts non-string item",
			method:   http.MethodGet,
			uri:      "/session/session/contexts",
			response: `["NATIVE_APP",1]`,
			invoke: func(session *appium.Session) error {
				contexts, err := session.Contexts(context.Background())
				if contexts != nil {
					t.Fatalf("invalid contexts returned partial result: %#v", contexts)
				}
				return err
			},
		},
		{
			name:     "contexts unpaired surrogate",
			method:   http.MethodGet,
			uri:      "/session/session/contexts",
			response: `["\ud800"]`,
			invoke: func(session *appium.Session) error {
				_, err := session.Contexts(context.Background())
				return err
			},
		},
		{
			name:     "current context null",
			method:   http.MethodGet,
			uri:      "/session/session/context",
			response: `null`,
			invoke: func(session *appium.Session) error {
				_, err := session.CurrentContext(context.Background())
				return err
			},
		},
		{
			name:     "current context array",
			method:   http.MethodGet,
			uri:      "/session/session/context",
			response: `[]`,
			invoke: func(session *appium.Session) error {
				_, err := session.CurrentContext(context.Background())
				return err
			},
		},
		{
			name:     "current context unpaired surrogate",
			method:   http.MethodGet,
			uri:      "/session/session/context",
			response: `"\udfff"`,
			invoke: func(session *appium.Session) error {
				_, err := session.CurrentContext(context.Background())
				return err
			},
		},
		{
			name:     "switch context non-null",
			method:   http.MethodPost,
			uri:      "/session/session/context",
			response: `{}`,
			invoke: func(session *appium.Session) error {
				return session.SwitchContext(context.Background(), "WEBVIEW_app")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session, recorder := newContextTestSession(
				t,
				appium.ClientOptions{},
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					if request.Method != test.method || request.RequestURI != test.uri {
						http.NotFound(writer, request)
						return
					}
					_, _ = writer.Write([]byte(`{"value":` + test.response + `}`))
				}),
			)

			err := test.invoke(session)
			if err == nil {
				t.Fatal("expected response validation error")
			}
			if !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
				t.Fatalf("error = %v, want response invalid", err)
			}
			if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryAcknowledged {
				t.Fatalf("delivery = %q, want %q", delivery, appium.DeliveryAcknowledged)
			}
			if requests := recorder.Requests(); len(requests) != 1 {
				t.Fatalf("request count = %d, want 1", len(requests))
			}
		})
	}
}

func TestSwitchContextRejectsInvalidUTF8BeforeDelivery(t *testing.T) {
	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.NotFoundHandler(),
	)

	err := session.SwitchContext(
		context.Background(),
		string([]byte{0xff}),
	)
	if err == nil || !appium.IsErrorCode(err, appium.CodeInvalidArgument) {
		t.Fatalf("error = %v, want invalid argument", err)
	}
	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryNotSent {
		t.Fatalf("delivery = %q, want %q", delivery, appium.DeliveryNotSent)
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("invalid UTF-8 must not be sent: got %d requests", len(requests))
	}
}

func TestSwitchContextPreservesRemoteFailureWithoutFallback(t *testing.T) {
	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(
				`{"value":{"error":"no such context","message":"missing"}}`,
			))
		}),
	)

	err := session.SwitchContext(context.Background(), "WEBVIEW_missing")
	if err == nil || !appium.IsErrorCode(err, appium.CodeCommandFailed) {
		t.Fatalf("error = %v, want remote command failure", err)
	}
	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryAcknowledged {
		t.Fatalf("delivery = %q, want %q", delivery, appium.DeliveryAcknowledged)
	}
	var clientErr *appium.Error
	if !errors.As(err, &clientErr) {
		t.Fatalf("error type = %T, want *appium.Error", err)
	}
	if clientErr.Operation != "switch_context" || clientErr.RemoteCode != "no such context" {
		t.Fatalf("unexpected remote error: %#v", clientErr)
	}
	if requests := recorder.Requests(); len(requests) != 1 {
		t.Fatalf("switch failure must not trigger fallback: got %d requests", len(requests))
	}
}

func TestSwitchContextTransportFailureLeavesDeliveryUnknown(t *testing.T) {
	hijacked := make(chan error, 1)
	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			connection, _, err := http.NewResponseController(writer).Hijack()
			if err != nil {
				hijacked <- err
				return
			}
			hijacked <- nil
			_ = connection.Close()
		}),
	)

	err := session.SwitchContext(context.Background(), "WEBVIEW_app")
	if hijackErr := <-hijacked; hijackErr != nil {
		t.Fatalf("hijack connection: %v", hijackErr)
	}
	if err == nil || !appium.IsErrorCode(err, appium.CodeTransportFailed) {
		t.Fatalf("error = %v, want transport failure", err)
	}
	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryUnknown {
		t.Fatalf("delivery = %q, want %q", delivery, appium.DeliveryUnknown)
	}
	if requests := recorder.Requests(); len(requests) != 1 {
		t.Fatalf("uncertain switch must not be replayed: got %d requests", len(requests))
	}
}

func newContextTestSession(
	t *testing.T,
	options appium.ClientOptions,
	next http.Handler,
) (*appium.Session, *contracttest.Recorder) {
	t.Helper()

	recorder := contracttest.NewRecorder(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost && request.RequestURI == "/session" {
			_, _ = writer.Write([]byte(
				`{"value":{"sessionId":"session/id","capabilities":{"automationName":"XCUITest"}}}`,
			))
			return
		}
		if next == nil {
			http.NotFound(writer, request)
			return
		}
		next.ServeHTTP(writer, request)
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
			"appium:automationName": "XCUITest",
		}),
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	recorder.Reset()
	return session, recorder
}

func writeJSONValue(
	t *testing.T,
	writer http.ResponseWriter,
	value any,
) {
	t.Helper()
	if err := json.NewEncoder(writer).Encode(map[string]any{"value": value}); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func equalStrings(first []string, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}
