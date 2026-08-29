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

func TestSessionCloseMarksClosedAfterAcknowledgedInvalidSession(t *testing.T) {
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
							`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
						),
					)

				case request.Method == http.MethodDelete &&
					request.RequestURI == "/session/session":
					writer.WriteHeader(
						http.StatusNotFound,
					)

					_, _ = writer.Write(
						[]byte(
							`{"value":{"error":"invalid session id","message":"session no longer exists"}}`,
						),
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

	recorder.Reset()

	err = session.Close(
		context.Background(),
	)
	if err == nil {
		t.Fatal(
			"expected invalid session error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeSessionLost,
	) {
		t.Fatalf(
			"unexpected close error code: %v",
			err,
		)
	}

	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryAcknowledged {
		t.Fatalf(
			"unexpected close delivery state: expected %q, got %q",
			appium.DeliveryAcknowledged,
			delivery,
		)
	}

	// 远端已经明确确认 Session 不存在，
	// 因此本地状态必须被视为已经关闭。
	if err := session.Close(
		context.Background(),
	); err != nil {
		t.Fatalf(
			"second close after acknowledged invalid session: %v",
			err,
		)
	}

	// 已关闭 Session 的普通命令必须在本地拒绝，
	// 不允许再次探测或向远端发送请求。
	_, err = session.WindowRect(
		context.Background(),
	)
	if err == nil {
		t.Fatal(
			"expected closed session error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeSessionLost,
	) {
		t.Fatalf(
			"unexpected closed session error code: %v",
			err,
		)
	}

	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryNotSent {
		t.Fatalf(
			"unexpected closed session delivery state: expected %q, got %q",
			appium.DeliveryNotSent,
			delivery,
		)
	}

	requests := recorder.Requests()
	if len(requests) != 1 {
		t.Fatalf(
			"unexpected request count: expected 1, got %d",
			len(requests),
		)
	}

	if err := contracttest.MatchMethod(
		requests[0],
		http.MethodDelete,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		requests[0],
		"/session/session",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchBody(
		requests[0],
		nil,
	); err != nil {
		t.Fatal(err)
	}
}

func TestCreateSessionReturnsCleanupOnlySessionWhenAutomaticCleanupFails(
	t *testing.T,
) {
	deleteCount := 0

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
					// 已经成功创建远端 Session，
					// 但 capabilities 缺少 automationName。
					_, _ = writer.Write(
						[]byte(
							`{"value":{"sessionId":"orphan/id","capabilities":{"platformName":"iOS"}}}`,
						),
					)

				case request.Method == http.MethodDelete &&
					request.RequestURI == "/session/orphan%2Fid":
					deleteCount++

					if deleteCount == 1 {
						// 第一次 DELETE 是 CreateSession 内部的自动清理。
						writer.WriteHeader(
							http.StatusInternalServerError,
						)

						_, _ = writer.Write(
							[]byte(
								`{"value":{"error":"unknown error","message":"cleanup failed"}}`,
							),
						)
						return
					}

					// 第二次 DELETE 是调用方使用返回的 cleanup-only
					// Session 显式执行 Close。
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

	if err == nil {
		t.Fatal(
			"expected create session error",
		)
	}

	// 创建响应本身无效这一事实仍然必须保留。
	if !appium.IsErrorCode(
		err,
		appium.CodeResponseInvalid,
	) {
		t.Fatalf(
			"unexpected create session error: %v",
			err,
		)
	}

	// 自动清理失败后不能丢失已经创建的物理 Session。
	if session == nil {
		t.Fatal(
			"expected cleanup-only session after automatic cleanup failure",
		)
	}

	if session.ID() != "orphan/id" {
		t.Fatalf(
			"unexpected cleanup-only session ID: %q",
			session.ID(),
		)
	}

	if session.Capabilities() != nil {
		t.Fatalf(
			"cleanup-only session must not expose capabilities: %#v",
			session.Capabilities(),
		)
	}

	if session.AutomationName() != "" {
		t.Fatalf(
			"cleanup-only session must not expose automation name: %q",
			session.AutomationName(),
		)
	}

	// cleanup-only Session 不能用于任何普通 WebDriver 命令。
	_, commandErr := session.WindowRect(
		context.Background(),
	)
	if commandErr == nil {
		t.Fatal(
			"expected cleanup-only session command error",
		)
	}

	if !appium.IsErrorCode(
		commandErr,
		appium.CodeInvalidArgument,
	) {
		t.Fatalf(
			"unexpected cleanup-only command error: %v",
			commandErr,
		)
	}

	if delivery := appium.DeliveryOf(commandErr); delivery != appium.DeliveryNotSent {
		t.Fatalf(
			"unexpected command delivery state: expected %q, got %q",
			appium.DeliveryNotSent,
			delivery,
		)
	}

	// 但必须允许调用方显式 Close 这个远端物理 Session。
	if err := session.Close(
		context.Background(),
	); err != nil {
		t.Fatalf(
			"explicit cleanup-only session close: %v",
			err,
		)
	}

	// 成功 Close 后仍保持幂等。
	if err := session.Close(
		context.Background(),
	); err != nil {
		t.Fatalf(
			"second cleanup-only session close: %v",
			err,
		)
	}

	requests := recorder.Requests()

	// POST create + 自动 DELETE 失败 + 显式 DELETE 成功。
	if len(requests) != 3 {
		t.Fatalf(
			"unexpected request count: expected 3, got %d",
			len(requests),
		)
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

	if err := contracttest.MatchMethod(
		requests[2],
		http.MethodDelete,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		requests[2],
		"/session/orphan%2Fid",
	); err != nil {
		t.Fatal(err)
	}
}
