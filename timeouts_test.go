package soluna_appium_client_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestSessionTimeoutSetters(t *testing.T) {
	recorder := contracttest.NewRecorder(
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.Header().Set(
					"Content-Type",
					"application/json",
				)

				switch {
				case request.Method == http.MethodPost &&
					request.RequestURI == "/session":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"sessionId":"session/id","capabilities":{"automationName":"XCUITest"}}}`,
						),
					)

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session%2Fid/timeouts":
					_, _ = writer.Write(
						[]byte(`{"value":null}`),
					)

				default:
					http.NotFound(
						writer,
						request,
					)
				}
			},
		),
	)

	server := contracttest.NewServer(recorder)
	defer server.Close()

	client, err := server.NewClient(
		appium.ClientOptions{},
	)
	if err != nil {
		t.Fatalf(
			"create client: %v",
			err,
		)
	}

	session, err := client.CreateSession(
		context.Background(),
		appium.MatchCapabilities(
			appium.Capabilities{
				"platformName":          "iOS",
				"appium:automationName": "XCUITest",
			},
		),
	)
	if err != nil {
		t.Fatalf(
			"create session: %v",
			err,
		)
	}

	if err := session.SetScriptTimeout(
		context.Background(),
		0,
	); err != nil {
		t.Fatalf(
			"set script timeout: %v",
			err,
		)
	}

	if err := session.SetPageLoadTimeout(
		context.Background(),
		12*time.Second+345*time.Millisecond,
	); err != nil {
		t.Fatalf(
			"set page load timeout: %v",
			err,
		)
	}

	if err := session.SetImplicitWait(
		context.Background(),
		2500*time.Millisecond,
	); err != nil {
		t.Fatalf(
			"set implicit wait: %v",
			err,
		)
	}

	requests := recorder.Requests()
	if len(requests) != 4 {
		t.Fatalf(
			"unexpected request count: expected 4, got %d",
			len(requests),
		)
	}

	scriptRequest := requests[1]

	if err := contracttest.MatchMethod(
		scriptRequest,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		scriptRequest,
		"/session/session%2Fid/timeouts",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchHeader(
		scriptRequest,
		"Content-Type",
		"application/json",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		scriptRequest,
		map[string]any{
			"script": 0,
		},
	); err != nil {
		t.Fatal(err)
	}

	pageLoadRequest := requests[2]

	if err := contracttest.MatchRequestURI(
		pageLoadRequest,
		"/session/session%2Fid/timeouts",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		pageLoadRequest,
		map[string]any{
			"pageLoad": 12345,
		},
	); err != nil {
		t.Fatal(err)
	}

	implicitRequest := requests[3]

	if err := contracttest.MatchRequestURI(
		implicitRequest,
		"/session/session%2Fid/timeouts",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		implicitRequest,
		map[string]any{
			"implicit": 2500,
		},
	); err != nil {
		t.Fatal(err)
	}
}

func TestSessionTimeoutsRejectInvalidDurationBeforeDelivery(t *testing.T) {
	recorder := contracttest.NewRecorder(
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.Header().Set(
					"Content-Type",
					"application/json",
				)

				if request.Method == http.MethodPost &&
					request.RequestURI == "/session" {
					_, _ = writer.Write(
						[]byte(
							`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
						),
					)
					return
				}

				http.NotFound(
					writer,
					request,
				)
			},
		),
	)

	server := contracttest.NewServer(recorder)
	defer server.Close()

	client, err := server.NewClient(
		appium.ClientOptions{},
	)
	if err != nil {
		t.Fatalf(
			"create client: %v",
			err,
		)
	}

	session, err := client.CreateSession(
		context.Background(),
		appium.MatchCapabilities(
			appium.Capabilities{
				"platformName":          "iOS",
				"appium:automationName": "XCUITest",
			},
		),
	)
	if err != nil {
		t.Fatalf(
			"create session: %v",
			err,
		)
	}

	recorder.Reset()

	err = session.SetScriptTimeout(
		context.Background(),
		-time.Millisecond,
	)
	if err == nil {
		t.Fatal(
			"expected negative timeout error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeInvalidArgument,
	) {
		t.Fatalf(
			"unexpected error code: %v",
			err,
		)
	}

	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryNotSent {
		t.Fatalf(
			"unexpected delivery state: expected %q, got %q",
			appium.DeliveryNotSent,
			delivery,
		)
	}

	err = session.SetImplicitWait(
		context.Background(),
		1500*time.Microsecond,
	)
	if err == nil {
		t.Fatal(
			"expected non-millisecond timeout error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeInvalidArgument,
	) {
		t.Fatalf(
			"unexpected error code: %v",
			err,
		)
	}

	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryNotSent {
		t.Fatalf(
			"unexpected delivery state: expected %q, got %q",
			appium.DeliveryNotSent,
			delivery,
		)
	}

	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf(
			"invalid timeouts must not be delivered: got %d requests",
			len(requests),
		)
	}
}

func TestSessionTimeoutsProtocolAndNoCache(t *testing.T) {
	requestCount := 0
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.RequestURI == "/session":
			_, _ = writer.Write([]byte(`{"value":{"sessionId":"session/id","capabilities":{"automationName":"XCUITest"}}}`))
		case request.Method == http.MethodGet && request.RequestURI == "/session/session%2Fid/timeouts":
			requestCount++
			if requestCount == 1 {
				_, _ = writer.Write([]byte(`{"value":{"command":12345,"implicit":2500}}`))
				return
			}
			_, _ = writer.Write([]byte(`{"value":{"command":2,"implicit":3}}`))
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

	first, err := session.Timeouts(context.Background())
	if err != nil {
		t.Fatalf("read first timeouts: %v", err)
	}
	if first.Command != 12345*time.Millisecond || first.Implicit != 2500*time.Millisecond {
		t.Fatalf("unexpected first timeouts: %#v", first)
	}
	second, err := session.Timeouts(context.Background())
	if err != nil {
		t.Fatalf("read second timeouts: %v", err)
	}
	if second.Command != 2*time.Millisecond || second.Implicit != 3*time.Millisecond {
		t.Fatalf("unexpected second timeouts: %#v", second)
	}
	if requestCount != 2 {
		t.Fatalf("expected two remote reads, got %d", requestCount)
	}
	for _, request := range recorder.Requests()[1:] {
		if len(request.Body) != 0 {
			t.Fatalf("GET timeouts must not send a request body: %s", request.Body)
		}
	}
}

func TestSessionTimeoutsRejectInvalidResponses(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{name: "missing field", response: `{"value":{"command":1}}`},
		{name: "command null", response: `{"value":{"command":null,"implicit":2}}`},
		{name: "implicit null", response: `{"value":{"command":1,"implicit":null}}`},
		{name: "negative", response: `{"value":{"command":-1,"implicit":2}}`},
		{name: "fractional", response: `{"value":{"command":1.5,"implicit":2}}`},
		{name: "duration overflow", response: `{"value":{"command":9223372036855,"implicit":2}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session, _ := newTimeoutTestSession(t, test.response)
			_, err := session.Timeouts(context.Background())
			if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
				t.Fatalf("expected response invalid error, got %v", err)
			}
			if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
				t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
			}
		})
	}
}

func TestSessionTimeoutsCanceledBeforeDelivery(t *testing.T) {
	session, recorder := newTimeoutTestSession(t, `{"value":{"command":1,"implicit":2}}`)
	recorder.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := session.Timeouts(ctx)
	if err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("canceled timeout read must not be delivered: got %d requests", len(requests))
	}
}

func newTimeoutTestSession(t *testing.T, response string) (*appium.Session, *contracttest.Recorder) {
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
