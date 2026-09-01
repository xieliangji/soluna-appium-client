package appium_test

import (
	"context"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestSessionHealthyUsesWindowRectProbe(t *testing.T) {
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

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/window/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":0,"y":0,"width":390,"height":844}}`,
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

	if err := session.Healthy(
		context.Background(),
	); err != nil {
		t.Fatalf(
			"healthy session probe: %v",
			err,
		)
	}

	requests := recorder.Requests()
	if len(requests) != 1 {
		t.Fatalf(
			"unexpected request count: expected 1, got %d",
			len(requests),
		)
	}

	request := requests[0]

	if err := contracttest.MatchMethod(
		request,
		http.MethodGet,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		request,
		"/session/session%2Fid/window/rect",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchBody(
		request,
		nil,
	); err != nil {
		t.Fatal(err)
	}

	if values := request.Header.Values(
		"Content-Type",
	); len(values) != 0 {
		t.Fatalf(
			"unexpected Content-Type header: %v",
			values,
		)
	}
}

func TestSessionHealthyMarksClosedAfterAcknowledgedSessionLoss(
	t *testing.T,
) {
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

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session/window/rect":
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

	err = session.Healthy(
		context.Background(),
	)
	if err == nil {
		t.Fatal(
			"expected lost session health error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeSessionLost,
	) {
		t.Fatalf(
			"unexpected health error code: %v",
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

	if requests := recorder.Requests(); len(requests) != 1 {
		t.Fatalf(
			"unexpected request count: expected 1, got %d",
			len(requests),
		)
	}

	recorder.Reset()

	// 已经被远端明确确认丢失的 Session，
	// 后续 Healthy 必须直接在本地拒绝。
	err = session.Healthy(
		context.Background(),
	)
	if err == nil {
		t.Fatal(
			"expected locally closed session error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeSessionLost,
	) {
		t.Fatalf(
			"unexpected local health error code: %v",
			err,
		)
	}

	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryNotSent {
		t.Fatalf(
			"unexpected local delivery state: expected %q, got %q",
			appium.DeliveryNotSent,
			delivery,
		)
	}

	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf(
			"closed session health probe must not be delivered: got %d requests",
			len(requests),
		)
	}
}

func TestSessionHealthyDoesNotCloseAfterDriverFailure(t *testing.T) {
	probeCount := 0

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

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session/window/rect":
					probeCount++

					if probeCount == 1 {
						writer.WriteHeader(
							http.StatusInternalServerError,
						)

						_, _ = writer.Write(
							[]byte(
								`{"value":{"error":"unknown error","message":"driver command failed"}}`,
							),
						)
						return
					}

					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":0,"y":0,"width":390,"height":844}}`,
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

	err = session.Healthy(
		context.Background(),
	)
	if err == nil {
		t.Fatal(
			"expected driver health failure",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeCommandFailed,
	) {
		t.Fatalf(
			"unexpected driver health error: %v",
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

	// Generic Driver failure 不足以证明 Session 已经不存在。
	// 第二次 Healthy 必须仍然能够真正发送探测请求。
	if err := session.Healthy(
		context.Background(),
	); err != nil {
		t.Fatalf(
			"second health probe after driver failure: %v",
			err,
		)
	}

	requests := recorder.Requests()
	if len(requests) != 2 {
		t.Fatalf(
			"second health probe must reach remote: expected 2 requests, got %d",
			len(requests),
		)
	}
}
