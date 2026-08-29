package soluna_appium_client_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestSessionCreateAndClose(t *testing.T) {
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

				switch request.Method {
				case http.MethodPost:
					if request.URL.Path != "/session" {
						http.NotFound(writer, request)
						return
					}

					_, _ = writer.Write(
						[]byte(
							`{"value":{"sessionId":"session/id","capabilities":{"platformName":"iOS","automationName":"XCUITest","deviceName":"iPhone 17","nested":{"source":"remote"}}}}`,
						),
					)

				case http.MethodDelete:
					_, _ = writer.Write(
						[]byte(`{"value":null}`),
					)

				default:
					http.NotFound(writer, request)
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

	requestedCapabilities := appium.W3CCapabilities{
		AlwaysMatch: appium.Capabilities{
			"platformName":          "iOS",
			"appium:automationName": "XCUITest",
		},
		FirstMatch: []appium.Capabilities{
			{
				"appium:deviceName": "iPhone 17",
			},
		},
	}

	session, err := client.CreateSession(
		context.Background(),
		requestedCapabilities,
	)
	if err != nil {
		t.Fatalf(
			"create session: %v",
			err,
		)
	}

	if session.ID() != "session/id" {
		t.Fatalf(
			"unexpected session ID: %q",
			session.ID(),
		)
	}

	if session.AutomationName() != "XCUITest" {
		t.Fatalf(
			"unexpected automation name: %q",
			session.AutomationName(),
		)
	}

	expectedCapabilities := appium.Capabilities{
		"platformName":   "iOS",
		"automationName": "XCUITest",
		"deviceName":     "iPhone 17",
		"nested": map[string]any{
			"source": "remote",
		},
	}

	capabilities := session.Capabilities()

	if !reflect.DeepEqual(
		capabilities,
		expectedCapabilities,
	) {
		t.Fatalf(
			"unexpected capabilities: expected %#v, got %#v",
			expectedCapabilities,
			capabilities,
		)
	}

	// Capabilities 必须返回深拷贝。
	nested, ok := capabilities["nested"].(map[string]any)
	if !ok {
		t.Fatalf(
			"unexpected nested capability type: %T",
			capabilities["nested"],
		)
	}

	nested["source"] = "mutated"

	again := session.Capabilities()
	againNested, ok := again["nested"].(map[string]any)
	if !ok {
		t.Fatalf(
			"unexpected nested capability type after copy: %T",
			again["nested"],
		)
	}

	if againNested["source"] != "remote" {
		t.Fatalf(
			"session capabilities were mutated through returned copy",
		)
	}

	if err := session.Close(
		context.Background(),
	); err != nil {
		t.Fatalf(
			"close session: %v",
			err,
		)
	}

	// Close 必须可以重复调用，而且不能重复发送 DELETE。
	if err := session.Close(
		context.Background(),
	); err != nil {
		t.Fatalf(
			"close session again: %v",
			err,
		)
	}

	requests := recorder.Requests()
	if len(requests) != 2 {
		t.Fatalf(
			"unexpected request count: expected 2, got %d",
			len(requests),
		)
	}

	createRequest := requests[0]

	if err := contracttest.MatchMethod(
		createRequest,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		createRequest,
		"/session",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchHeader(
		createRequest,
		"Content-Type",
		"application/json",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		createRequest,
		map[string]any{
			"capabilities": requestedCapabilities,
		},
	); err != nil {
		t.Fatal(err)
	}

	closeRequest := requests[1]

	if err := contracttest.MatchMethod(
		closeRequest,
		http.MethodDelete,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		closeRequest,
		"/session/session%2Fid",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchBody(
		closeRequest,
		nil,
	); err != nil {
		t.Fatal(err)
	}

	if values := closeRequest.Header.Values(
		"Content-Type",
	); len(values) != 0 {
		t.Fatalf(
			"unexpected Content-Type header on close: %v",
			values,
		)
	}
}

func TestCreateSessionCleansUpAfterInvalidCapabilities(t *testing.T) {
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

				switch request.Method {
				case http.MethodPost:
					_, _ = writer.Write(
						[]byte(
							`{"value":{"sessionId":"orphan/id","capabilities":{"platformName":"iOS"}}}`,
						),
					)

				case http.MethodDelete:
					_, _ = writer.Write(
						[]byte(`{"value":null}`),
					)

				default:
					http.NotFound(writer, request)
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

	if err == nil {
		t.Fatal(
			"expected invalid create session response error",
		)
	}

	if session != nil {
		t.Fatal(
			"expected nil session after successful automatic cleanup",
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

	requests := recorder.Requests()
	if len(requests) != 2 {
		t.Fatalf(
			"unexpected request count: expected 2, got %d",
			len(requests),
		)
	}

	if err := contracttest.MatchMethod(
		requests[0],
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		requests[0],
		"/session",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchMethod(
		requests[1],
		http.MethodDelete,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		requests[1],
		"/session/orphan%2Fid",
	); err != nil {
		t.Fatal(err)
	}
}
