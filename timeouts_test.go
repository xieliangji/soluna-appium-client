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
