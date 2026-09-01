package appium_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestRemoteErrorCodeMapping(t *testing.T) {
	tests := []struct {
		remoteCode string
		expected   appium.ErrorCode
	}{
		{
			remoteCode: "invalid argument",
			expected:   appium.CodeInvalidArgument,
		},
		{
			remoteCode: "invalid selector",
			expected:   appium.CodeInvalidArgument,
		},
		{
			remoteCode: "invalid session id",
			expected:   appium.CodeSessionLost,
		},
		{
			remoteCode: "no such element",
			expected:   appium.CodeElementNotFound,
		},
		{
			remoteCode: "stale element reference",
			expected:   appium.CodeElementStale,
		},
		{
			remoteCode: "unknown command",
			expected:   appium.CodeUnsupported,
		},
		{
			remoteCode: "unknown method",
			expected:   appium.CodeUnsupported,
		},
		{
			remoteCode: "unsupported operation",
			expected:   appium.CodeUnsupported,
		},
		{
			remoteCode: "javascript error",
			expected:   appium.CodeCommandFailed,
		},
	}

	for _, test := range tests {
		t.Run(
			test.remoteCode,
			func(t *testing.T) {
				session := newCommandErrorTestSession(
					t,
					func(
						writer http.ResponseWriter,
						request *http.Request,
					) {
						writer.WriteHeader(
							http.StatusInternalServerError,
						)

						_ = json.NewEncoder(writer).Encode(
							map[string]any{
								"value": map[string]any{
									"error":   test.remoteCode,
									"message": "remote failure",
								},
							},
						)
					},
				)

				_, err := session.ExecuteScript(
					context.Background(),
					"return 1",
					nil,
				)
				if err == nil {
					t.Fatal(
						"expected remote command error",
					)
				}

				if !appium.IsErrorCode(
					err,
					test.expected,
				) {
					t.Fatalf(
						"unexpected error code for %q: %v",
						test.remoteCode,
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

				var clientErr *appium.Error

				if !errors.As(
					err,
					&clientErr,
				) {
					t.Fatalf(
						"expected *appium.Error, got %T",
						err,
					)
				}

				if clientErr.StatusCode != http.StatusInternalServerError {
					t.Fatalf(
						"unexpected HTTP status: expected %d, got %d",
						http.StatusInternalServerError,
						clientErr.StatusCode,
					)
				}

				if clientErr.RemoteCode != test.remoteCode {
					t.Fatalf(
						"unexpected remote code: expected %q, got %q",
						test.remoteCode,
						clientErr.RemoteCode,
					)
				}

				if clientErr.Operation != "execute_script" {
					t.Fatalf(
						"unexpected operation: %q",
						clientErr.Operation,
					)
				}
			},
		)
	}
}

func TestRemoteErrorDiagnosticDataIsRedacted(t *testing.T) {
	const secret = "secret-value"

	session := newCommandErrorTestSession(
		t,
		func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.WriteHeader(
				http.StatusNotFound,
			)

			_ = json.NewEncoder(writer).Encode(
				map[string]any{
					"value": map[string]any{
						"error": "no such element",
						"message": "lookup failed: password=" +
							secret,
						"stacktrace": "Authorization: Bearer " +
							secret,
						"data": map[string]any{
							"password": secret,
							"safe":     "visible",
						},
					},
				},
			)
		},
	)

	_, err := session.ExecuteScript(
		context.Background(),
		"return 1",
		nil,
	)
	if err == nil {
		t.Fatal(
			"expected remote command error",
		)
	}

	var clientErr *appium.Error

	if !errors.As(
		err,
		&clientErr,
	) {
		t.Fatalf(
			"expected *appium.Error, got %T",
			err,
		)
	}

	if clientErr.Code != appium.CodeElementNotFound {
		t.Fatalf(
			"unexpected error code: %q",
			clientErr.Code,
		)
	}

	if clientErr.RemoteCode != "no such element" {
		t.Fatalf(
			"unexpected remote code: %q",
			clientErr.RemoteCode,
		)
	}

	if clientErr.StatusCode != http.StatusNotFound {
		t.Fatalf(
			"unexpected HTTP status: %d",
			clientErr.StatusCode,
		)
	}

	if clientErr.Delivery != appium.DeliveryAcknowledged {
		t.Fatalf(
			"unexpected delivery state: %q",
			clientErr.Delivery,
		)
	}

	if clientErr.Message != "[REDACTED]" {
		t.Fatalf(
			"unexpected redacted message: %q",
			clientErr.Message,
		)
	}

	if strings.Contains(
		err.Error(),
		secret,
	) {
		t.Fatalf(
			"public error text contains sensitive value: %q",
			err.Error(),
		)
	}

	if len(clientErr.RemoteData) == 0 {
		t.Fatal(
			"expected sanitized remote diagnostic data",
		)
	}

	if bytes.Contains(
		clientErr.RemoteData,
		[]byte(secret),
	) {
		t.Fatalf(
			"remote diagnostic data contains sensitive value: %s",
			clientErr.RemoteData,
		)
	}

	var remoteData map[string]any

	if err := json.Unmarshal(
		clientErr.RemoteData,
		&remoteData,
	); err != nil {
		t.Fatalf(
			"decode sanitized remote data: %v",
			err,
		)
	}

	if remoteData["error"] != "no such element" {
		t.Fatalf(
			"unexpected sanitized remote error code: %#v",
			remoteData["error"],
		)
	}

	if remoteData["message"] != "[REDACTED]" {
		t.Fatalf(
			"unexpected sanitized message: %#v",
			remoteData["message"],
		)
	}

	if remoteData["stacktrace"] != "[REDACTED]" {
		t.Fatalf(
			"unexpected sanitized stacktrace: %#v",
			remoteData["stacktrace"],
		)
	}

	data, ok := remoteData["data"].(map[string]any)
	if !ok {
		t.Fatalf(
			"unexpected sanitized data type: %T",
			remoteData["data"],
		)
	}

	if data["password"] != "[REDACTED]" {
		t.Fatalf(
			"unexpected sanitized password: %#v",
			data["password"],
		)
	}

	if data["safe"] != "visible" {
		t.Fatalf(
			"non-sensitive diagnostic data was changed: %#v",
			data["safe"],
		)
	}
}

func TestMalformedRemoteErrorIsResponseInvalid(t *testing.T) {
	session := newCommandErrorTestSession(
		t,
		func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.WriteHeader(
				http.StatusInternalServerError,
			)

			// W3C 远端错误必须同时包含 error 和 message。
			_ = json.NewEncoder(writer).Encode(
				map[string]any{
					"value": map[string]any{
						"error": "unknown command",
					},
				},
			)
		},
	)

	_, err := session.ExecuteScript(
		context.Background(),
		"return 1",
		nil,
	)
	if err == nil {
		t.Fatal(
			"expected malformed remote response error",
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

	var clientErr *appium.Error

	if !errors.As(
		err,
		&clientErr,
	) {
		t.Fatalf(
			"expected *appium.Error, got %T",
			err,
		)
	}

	if clientErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf(
			"unexpected HTTP status: expected %d, got %d",
			http.StatusInternalServerError,
			clientErr.StatusCode,
		)
	}

	if clientErr.RemoteCode != "" {
		t.Fatalf(
			"malformed remote error must not expose a parsed remote code: %q",
			clientErr.RemoteCode,
		)
	}

	if len(clientErr.RemoteData) != 0 {
		t.Fatalf(
			"malformed remote error must not expose parsed remote data: %s",
			clientErr.RemoteData,
		)
	}
}

// newCommandErrorTestSession 创建专门用于远端错误测试的 Session。
func newCommandErrorTestSession(
	t *testing.T,
	commandHandler http.HandlerFunc,
) *appium.Session {
	t.Helper()

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

				switch {
				case request.Method == http.MethodPost &&
					request.RequestURI == "/session":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
						),
					)

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session/execute/sync":
					commandHandler(
						writer,
						request,
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

	t.Cleanup(
		server.Close,
	)

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

	return session
}
