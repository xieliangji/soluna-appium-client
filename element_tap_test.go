package soluna_appium_client_test

import (
	"context"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestElementTapUsesDeterministicWindowIntersectionPoint(t *testing.T) {
	rectRequestCount := 0

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

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session/elements":
					_, _ = writer.Write(
						[]byte(
							`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"element"}]}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session/window/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":0,"y":0,"width":100,"height":100}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session/element/element/rect":
					rectRequestCount++

					switch rectRequestCount {
					case 1:
						// Find 使用：元素完整位于 Window 内。
						_, _ = writer.Write(
							[]byte(
								`{"value":{"x":10,"y":10,"width":20,"height":20}}`,
							),
						)

					case 2:
						// Tap 使用：元素右侧超出 Window。
						// 交集为 [90,100) × [20,40)，中心为 (95,30)。
						_, _ = writer.Write(
							[]byte(
								`{"value":{"x":90,"y":20,"width":20,"height":20}}`,
							),
						)

					case 3:
						// 指定比例 Tap 使用：
						// 交集为 [80,100) × [70,100)。
						_, _ = writer.Write(
							[]byte(
								`{"value":{"x":80,"y":70,"width":40,"height":60}}`,
							),
						)

					case 4:
						// 元素已经完全移出 Window。
						_, _ = writer.Write(
							[]byte(
								`{"value":{"x":120,"y":20,"width":20,"height":20}}`,
							),
						)

					default:
						t.Fatalf(
							"unexpected element rect request count: %d",
							rectRequestCount,
						)
					}

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session/actions":
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

	element, err := session.Find(
		context.Background(),
		appium.ID("item"),
	)
	if err != nil {
		t.Fatalf(
			"find element: %v",
			err,
		)
	}

	recorder.Reset()

	// 默认 Tap 必须点击当前 Element/Window 交集区域中心。
	if err := element.Tap(
		context.Background(),
	); err != nil {
		t.Fatalf(
			"tap element: %v",
			err,
		)
	}

	requests := recorder.Requests()
	if len(requests) != 3 {
		t.Fatalf(
			"unexpected default tap request count: expected 3, got %d",
			len(requests),
		)
	}

	if err := contracttest.MatchRequestURI(
		requests[0],
		"/session/session/window/rect",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		requests[1],
		"/session/session/element/element/rect",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		requests[2],
		"/session/session/actions",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		requests[2],
		expectedTapActionsBody(
			95,
			30,
		),
	); err != nil {
		t.Fatal(err)
	}

	recorder.Reset()

	// 指定比例必须相对于交集区域计算，而不是元素原始 Rect。
	if err := element.TapInWindowIntersection(
		context.Background(),
		0.25,
		0.75,
	); err != nil {
		t.Fatalf(
			"tap element at ratio: %v",
			err,
		)
	}

	requests = recorder.Requests()
	if len(requests) != 3 {
		t.Fatalf(
			"unexpected ratio tap request count: expected 3, got %d",
			len(requests),
		)
	}

	if err := contracttest.MatchJSONBody(
		requests[2],
		expectedTapActionsBody(
			85,
			93,
		),
	); err != nil {
		t.Fatal(err)
	}

	recorder.Reset()

	// Find 成功后元素仍可能移动。
	// Tap 必须重新读取几何状态，并在没有交集时拒绝发送 Actions。
	err = element.Tap(
		context.Background(),
	)
	if err == nil {
		t.Fatal(
			"expected tap without window intersection to fail",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeInvalidArgument,
	) {
		t.Fatalf(
			"unexpected no-intersection error code: %v",
			err,
		)
	}

	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryNotSent {
		t.Fatalf(
			"unexpected no-intersection delivery state: expected %q, got %q",
			appium.DeliveryNotSent,
			delivery,
		)
	}

	requests = recorder.Requests()
	if len(requests) != 2 {
		t.Fatalf(
			"unexpected no-intersection request count: expected 2, got %d",
			len(requests),
		)
	}

	if err := contracttest.MatchRequestURI(
		requests[0],
		"/session/session/window/rect",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		requests[1],
		"/session/session/element/element/rect",
	); err != nil {
		t.Fatal(err)
	}

	recorder.Reset()

	// 非法比例属于纯本地参数错误，
	// 连 Window Rect 都不应该读取。
	for _, testCase := range []struct {
		name   string
		xRatio float64
		yRatio float64
	}{
		{
			name:   "negative x",
			xRatio: -0.1,
			yRatio: 0.5,
		},
		{
			name:   "x greater than one",
			xRatio: 1.1,
			yRatio: 0.5,
		},
		{
			name:   "negative y",
			xRatio: 0.5,
			yRatio: -0.1,
		},
		{
			name:   "y greater than one",
			xRatio: 0.5,
			yRatio: 1.1,
		},
	} {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				err := element.TapInWindowIntersection(
					context.Background(),
					testCase.xRatio,
					testCase.yRatio,
				)
				if err == nil {
					t.Fatal(
						"expected invalid tap ratio error",
					)
				}

				if !appium.IsErrorCode(
					err,
					appium.CodeInvalidArgument,
				) {
					t.Fatalf(
						"unexpected invalid ratio error code: %v",
						err,
					)
				}

				if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryNotSent {
					t.Fatalf(
						"unexpected invalid ratio delivery state: expected %q, got %q",
						appium.DeliveryNotSent,
						delivery,
					)
				}
			},
		)
	}

	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf(
			"invalid tap ratios must not reach remote: got %d requests",
			len(requests),
		)
	}
}

func expectedTapActionsBody(
	x int,
	y int,
) map[string]any {
	return map[string]any{
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
						"x":        x,
						"y":        y,
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
	}
}
