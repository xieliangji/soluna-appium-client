package appium_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestExecuteScriptWithOperationPreservesIdentity(t *testing.T) {
	session := newCommandErrorTestSession(
		t,
		func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write(
				[]byte(
					`{"value":{"error":"unknown command","message":"failed"}}`,
				),
			)
		},
	)

	_, err := session.ExecuteScriptWithOperation(
		context.Background(),
		"ios_press_button",
		"mobile: pressButton",
		[]any{
			map[string]any{
				"name": "home",
			},
		},
	)
	if err == nil {
		t.Fatal("expected execute method error")
	}

	var clientErr *appium.Error
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected structured appium error: %v", err)
	}

	if clientErr.Operation != "ios_press_button" {
		t.Fatalf(
			"unexpected operation: expected %q, got %q",
			"ios_press_button",
			clientErr.Operation,
		)
	}

	if clientErr.Delivery != appium.DeliveryAcknowledged {
		t.Fatalf(
			"unexpected delivery: expected %q, got %q",
			appium.DeliveryAcknowledged,
			clientErr.Delivery,
		)
	}
}

func TestExecuteScriptWithOperationRejectsUnstableIdentity(t *testing.T) {
	session := newCommandErrorTestSession(
		t,
		func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			t.Fatalf("invalid operation must not reach execute route")
		},
	)

	for _, testCase := range []struct {
		name      string
		operation string
	}{
		{
			name:      "empty",
			operation: "",
		},
		{
			name:      "leading space",
			operation: " ios_press_button",
		},
		{
			name:      "trailing space",
			operation: "ios_press_button ",
		},
		{
			name:      "uppercase",
			operation: "IOS_PRESS_BUTTON",
		},
		{
			name:      "hyphen",
			operation: "ios-press-button",
		},
		{
			name:      "newline",
			operation: "ios_press_button\nextra",
		},
		{
			name:      "too long",
			operation: strings.Repeat("a", 65),
		},
	} {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				_, err := session.ExecuteScriptWithOperation(
					context.Background(),
					testCase.operation,
					"mobile: pressButton",
					nil,
				)
				if err == nil {
					t.Fatal("expected invalid operation error")
				}

				if !appium.IsErrorCode(err, appium.CodeInvalidArgument) {
					t.Fatalf("unexpected error: %v", err)
				}

				if appium.DeliveryOf(err) != appium.DeliveryNotSent {
					t.Fatalf("invalid operation must not be delivered: %v", err)
				}

				var clientErr *appium.Error
				if !errors.As(err, &clientErr) {
					t.Fatalf("expected structured operation error: %v", err)
				}
				if clientErr.Operation != "execute_script" {
					t.Fatalf(
						"invalid identity must use canonical operation: %q",
						clientErr.Operation,
					)
				}
			},
		)
	}
}

func TestExecuteScriptWithOperationAndDecodeMapsDecoderFailure(t *testing.T) {
	session := newCommandErrorTestSession(
		t,
		func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(
				[]byte(`{"value":{"unexpected":true}}`),
			)
		},
	)

	decoderCalled := false
	err := session.ExecuteScriptWithOperationAndDecode(
		context.Background(),
		"ios_device_screen_info",
		"mobile: deviceScreenInfo",
		nil,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			decoderCalled = true
			return errors.New("typed response is invalid")
		},
	)
	if err == nil {
		t.Fatal("expected decoder failure")
	}
	if !decoderCalled {
		t.Fatal("expected response decoder to run in execution chain")
	}

	var clientErr *appium.Error
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected structured appium error: %v", err)
	}
	if clientErr.Code != appium.CodeResponseInvalid {
		t.Fatalf("unexpected error code: %q", clientErr.Code)
	}
	if clientErr.Operation != "ios_device_screen_info" {
		t.Fatalf("unexpected operation: %q", clientErr.Operation)
	}
	if clientErr.StatusCode != http.StatusOK {
		t.Fatalf(
			"unexpected HTTP status: expected %d, got %d",
			http.StatusOK,
			clientErr.StatusCode,
		)
	}
	if clientErr.Delivery != appium.DeliveryAcknowledged {
		t.Fatalf("unexpected delivery: %q", clientErr.Delivery)
	}
}

func TestExecuteScriptWithOperationAndDecodeRejectsNilDecoder(t *testing.T) {
	session := newCommandErrorTestSession(
		t,
		func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			t.Fatalf("nil decoder must not reach execute route")
		},
	)

	err := session.ExecuteScriptWithOperationAndDecode(
		context.Background(),
		"ios_device_screen_info",
		"mobile: deviceScreenInfo",
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected nil decoder error")
	}
	if !appium.IsErrorCode(err, appium.CodeInvalidArgument) {
		t.Fatalf("unexpected error code: %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}

	var clientErr *appium.Error
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected structured appium error: %v", err)
	}
	if clientErr.Operation != "ios_device_screen_info" {
		t.Fatalf("unexpected operation: %q", clientErr.Operation)
	}
}

func TestExecuteScriptProtocol(t *testing.T) {
	var executeCount atomic.Int32

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
					request.RequestURI == "/session/session%2Fid/execute/sync":
					switch executeCount.Add(1) {
					case 1:
						_, _ = writer.Write(
							[]byte(
								`{"value":{"ok":true,"items":[1,2]}}`,
							),
						)

					case 2:
						_, _ = writer.Write(
							[]byte(
								`{"value":"done"}`,
							),
						)

					default:
						http.Error(
							writer,
							"unexpected execute script request",
							http.StatusInternalServerError,
						)
					}

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

	firstResult, err := session.ExecuteScript(
		context.Background(),
		"return {ok: true}",
		nil,
	)
	if err != nil {
		t.Fatalf(
			"execute script with nil arguments: %v",
			err,
		)
	}

	expectedFirstResult := []byte(
		`{"ok":true,"items":[1,2]}`,
	)

	if !bytes.Equal(
		firstResult,
		expectedFirstResult,
	) {
		t.Fatalf(
			"unexpected first script result: expected %s, got %s",
			expectedFirstResult,
			firstResult,
		)
	}

	secondResult, err := session.ExecuteScript(
		context.Background(),
		"mobile: customCommand",
		[]any{
			map[string]any{
				"name":  "example",
				"count": 3,
			},
			"tail",
		},
	)
	if err != nil {
		t.Fatalf(
			"execute script with arguments: %v",
			err,
		)
	}

	if !bytes.Equal(
		secondResult,
		[]byte(`"done"`),
	) {
		t.Fatalf(
			"unexpected second script result: %s",
			secondResult,
		)
	}

	requests := recorder.Requests()
	if len(requests) != 3 {
		t.Fatalf(
			"unexpected request count: expected 3, got %d",
			len(requests),
		)
	}

	firstRequest := requests[1]

	if err := contracttest.MatchMethod(
		firstRequest,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		firstRequest,
		"/session/session%2Fid/execute/sync",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchHeader(
		firstRequest,
		"Content-Type",
		"application/json",
	); err != nil {
		t.Fatal(err)
	}

	// nil arguments 必须编码成空数组，不能发送 JSON null。
	if err := contracttest.MatchJSONBody(
		firstRequest,
		map[string]any{
			"script": "return {ok: true}",
			"args":   []any{},
		},
	); err != nil {
		t.Fatal(err)
	}

	secondRequest := requests[2]

	if err := contracttest.MatchRequestURI(
		secondRequest,
		"/session/session%2Fid/execute/sync",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		secondRequest,
		map[string]any{
			"script": "mobile: customCommand",
			"args": []any{
				map[string]any{
					"name":  "example",
					"count": 3,
				},
				"tail",
			},
		},
	); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteScriptRejectsUnencodableArgumentsBeforeDelivery(t *testing.T) {
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

	_, err = session.ExecuteScript(
		context.Background(),
		"return arguments[0]",
		[]any{
			math.Inf(1),
		},
	)
	if err == nil {
		t.Fatal(
			"expected unencodable script argument error",
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
			"unencodable arguments must not be delivered: got %d requests",
			len(requests),
		)
	}
}
