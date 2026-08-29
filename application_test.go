package soluna_appium_client_test

import (
	"context"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestApplicationCommands(t *testing.T) {
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
					request.RequestURI == "/session/session%2Fid/appium/device/activate_app":
					_, _ = writer.Write(
						[]byte(`{"value":null}`),
					)

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session%2Fid/appium/device/terminate_app":
					_, _ = writer.Write(
						[]byte(`{"value":null}`),
					)

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session%2Fid/appium/device/app_state":
					_, _ = writer.Write(
						[]byte(`{"value":4}`),
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

	const appID = "com.example.demo"

	if err := session.ActivateApp(
		context.Background(),
		appID,
	); err != nil {
		t.Fatalf(
			"activate app: %v",
			err,
		)
	}

	if err := session.TerminateApp(
		context.Background(),
		appID,
	); err != nil {
		t.Fatalf(
			"terminate app: %v",
			err,
		)
	}

	state, err := session.AppState(
		context.Background(),
		appID,
	)
	if err != nil {
		t.Fatalf(
			"get app state: %v",
			err,
		)
	}

	if state != appium.AppStateForeground {
		t.Fatalf(
			"unexpected app state: expected %d, got %d",
			appium.AppStateForeground,
			state,
		)
	}

	requests := recorder.Requests()
	if len(requests) != 4 {
		t.Fatalf(
			"unexpected request count: expected 4, got %d",
			len(requests),
		)
	}

	activateRequest := requests[1]

	if err := contracttest.MatchMethod(
		activateRequest,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		activateRequest,
		"/session/session%2Fid/appium/device/activate_app",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		activateRequest,
		map[string]any{
			"appId": appID,
		},
	); err != nil {
		t.Fatal(err)
	}

	terminateRequest := requests[2]

	if err := contracttest.MatchMethod(
		terminateRequest,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		terminateRequest,
		"/session/session%2Fid/appium/device/terminate_app",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		terminateRequest,
		map[string]any{
			"appId": appID,
		},
	); err != nil {
		t.Fatal(err)
	}

	stateRequest := requests[3]

	if err := contracttest.MatchMethod(
		stateRequest,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		stateRequest,
		"/session/session%2Fid/appium/device/app_state",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		stateRequest,
		map[string]any{
			"appId": appID,
		},
	); err != nil {
		t.Fatal(err)
	}
}

func TestAppStateRejectsUnknownValue(t *testing.T) {
	server := contracttest.NewServer(
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.Header().Set(
					"Content-Type",
					"application/json",
				)

				switch request.RequestURI {
				case "/session":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"sessionId":"session","capabilities":{"automationName":"UiAutomator2"}}}`,
						),
					)

				case "/session/session/appium/device/app_state":
					_, _ = writer.Write(
						[]byte(`{"value":5}`),
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
				"platformName":          "Android",
				"appium:automationName": "UiAutomator2",
			},
		),
	)
	if err != nil {
		t.Fatalf(
			"create session: %v",
			err,
		)
	}

	_, err = session.AppState(
		context.Background(),
		"com.example.demo",
	)
	if err == nil {
		t.Fatal(
			"expected invalid app state response error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeResponseInvalid,
	) {
		t.Fatalf(
			"unexpected error code: %v",
			err,
		)
	}

	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryAcknowledged {
		t.Fatalf(
			"unexpected delivery state: expected %q, got %q",
			appium.DeliveryAcknowledged,
			delivery,
		)
	}
}

func TestApplicationCommandsRejectEmptyAppIDBeforeDelivery(t *testing.T) {
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

				if request.RequestURI == "/session" {
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

	err = session.ActivateApp(
		context.Background(),
		"",
	)
	if err == nil {
		t.Fatal(
			"expected empty app ID error",
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
			"empty app ID must not be delivered: got %d requests",
			len(requests),
		)
	}
}
