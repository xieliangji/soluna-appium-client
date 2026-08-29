package soluna_appium_client_test

import (
	"bytes"
	"context"
	"math"
	"net/http"
	"sync/atomic"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

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
