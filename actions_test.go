package appium_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestTouchActionsProtocol(t *testing.T) {
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
					request.RequestURI == "/session/session%2Fid/actions":
					_, _ = writer.Write(
						[]byte(`{"value":null}`),
					)

				case request.Method == http.MethodDelete &&
					request.RequestURI == "/session/session%2Fid/actions":
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

	// TouchAction 的零值必须被识别为无效动作，不能静默编码成
	// pointerMove(0, 0) 这种具有副作用的操作。
	recorder.Reset()

	err = session.PerformActions(
		context.Background(),
		appium.TouchSequence(
			"finger",
			appium.TouchAction{},
		),
	)
	if err == nil {
		t.Fatal("expected zero TouchAction to be rejected")
	}

	if !appium.IsErrorCode(err, appium.CodeInvalidArgument) {
		t.Fatalf("unexpected zero TouchAction error: %v", err)
	}

	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryNotSent {
		t.Fatalf(
			"zero TouchAction must not be delivered: got %q",
			delivery,
		)
	}

	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf(
			"zero TouchAction must not reach remote: got %d requests",
			len(requests),
		)
	}

	recorder.Reset()

	if err := session.Tap(
		context.Background(),
		appium.Point{
			X: 10,
			Y: 20,
		},
	); err != nil {
		t.Fatalf(
			"tap: %v",
			err,
		)
	}

	if err := session.LongPress(
		context.Background(),
		appium.Point{
			X: 30,
			Y: 40,
		},
		750*time.Millisecond,
	); err != nil {
		t.Fatalf(
			"long press: %v",
			err,
		)
	}

	if err := session.Swipe(
		context.Background(),
		appium.Point{
			X: 50,
			Y: 60,
		},
		appium.Point{
			X: 150,
			Y: 260,
		},
		300*time.Millisecond,
	); err != nil {
		t.Fatalf(
			"swipe: %v",
			err,
		)
	}

	if err := session.ReleaseActions(
		context.Background(),
	); err != nil {
		t.Fatalf(
			"release actions: %v",
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

	tapRequest := requests[0]

	if err := contracttest.MatchMethod(
		tapRequest,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		tapRequest,
		"/session/session%2Fid/actions",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		tapRequest,
		map[string]any{
			"actions": []any{
				map[string]any{
					"type": "pointer",
					"id":   "finger",
					"parameters": map[string]any{
						"pointerType": "touch",
					},
					"actions": []any{
						map[string]any{
							"type":     "pointerMove",
							"duration": 0,
							"x":        10,
							"y":        20,
							"origin":   "viewport",
						},
						map[string]any{
							"type":   "pointerDown",
							"button": 0,
						},
						map[string]any{
							"type":   "pointerUp",
							"button": 0,
						},
					},
				},
			},
		},
	); err != nil {
		t.Fatal(err)
	}

	longPressRequest := requests[1]

	if err := contracttest.MatchRequestURI(
		longPressRequest,
		"/session/session%2Fid/actions",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		longPressRequest,
		map[string]any{
			"actions": []any{
				map[string]any{
					"type": "pointer",
					"id":   "finger",
					"parameters": map[string]any{
						"pointerType": "touch",
					},
					"actions": []any{
						map[string]any{
							"type":     "pointerMove",
							"duration": 0,
							"x":        30,
							"y":        40,
							"origin":   "viewport",
						},
						map[string]any{
							"type":   "pointerDown",
							"button": 0,
						},
						map[string]any{
							"type":     "pause",
							"duration": 750,
						},
						map[string]any{
							"type":   "pointerUp",
							"button": 0,
						},
					},
				},
			},
		},
	); err != nil {
		t.Fatal(err)
	}

	swipeRequest := requests[2]

	if err := contracttest.MatchRequestURI(
		swipeRequest,
		"/session/session%2Fid/actions",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		swipeRequest,
		map[string]any{
			"actions": []any{
				map[string]any{
					"type": "pointer",
					"id":   "finger",
					"parameters": map[string]any{
						"pointerType": "touch",
					},
					"actions": []any{
						map[string]any{
							"type":     "pointerMove",
							"duration": 0,
							"x":        50,
							"y":        60,
							"origin":   "viewport",
						},
						map[string]any{
							"type":   "pointerDown",
							"button": 0,
						},
						map[string]any{
							"type":     "pointerMove",
							"duration": 300,
							"x":        150,
							"y":        260,
							"origin":   "viewport",
						},
						map[string]any{
							"type":   "pointerUp",
							"button": 0,
						},
					},
				},
			},
		},
	); err != nil {
		t.Fatal(err)
	}

	releaseRequest := requests[3]

	if err := contracttest.MatchMethod(
		releaseRequest,
		http.MethodDelete,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		releaseRequest,
		"/session/session%2Fid/actions",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchBody(
		releaseRequest,
		nil,
	); err != nil {
		t.Fatal(err)
	}

	if values := releaseRequest.Header.Values(
		"Content-Type",
	); len(values) != 0 {
		t.Fatalf(
			"unexpected Content-Type header on release actions: %v",
			values,
		)
	}
}

func TestActionsRejectNonMillisecondDurationBeforeDelivery(t *testing.T) {
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

	// 只观察后续 Actions 调用是否产生网络请求。
	recorder.Reset()

	err = session.Swipe(
		context.Background(),
		appium.Point{
			X: 0,
			Y: 0,
		},
		appium.Point{
			X: 100,
			Y: 100,
		},
		1500*time.Microsecond,
	)
	if err == nil {
		t.Fatal(
			"expected invalid action duration error",
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

	requests := recorder.Requests()
	if len(requests) != 0 {
		t.Fatalf(
			"invalid action must not be delivered: got %d requests",
			len(requests),
		)
	}
}
