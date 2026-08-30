package soluna_appium_client_test

import (
	"context"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestElementFindUsesParentScopeAndStopsAtFirstWindowIntersection(
	t *testing.T,
) {
	neverRectRequests := 0

	recorder, parent := newElementScopeTestParent(
		t,
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				switch {
				case request.Method == http.MethodPost &&
					request.RequestURI ==
						"/session/session%2Fid/element/parent%2F1/elements":
					_, _ = writer.Write(
						[]byte(
							`{"value":[` +
								`{"element-6066-11e4-a52e-4f735466cecf":"child/off"},` +
								`{"element-6066-11e4-a52e-4f735466cecf":"child/visible"},` +
								`{"element-6066-11e4-a52e-4f735466cecf":"child/never"}` +
								`]}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI ==
						"/session/session%2Fid/element/child%2Foff/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":100,"y":10,"width":20,"height":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI ==
						"/session/session%2Fid/element/child%2Fvisible/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":90,"y":20,"width":20,"height":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI ==
						"/session/session%2Fid/element/child%2Fnever/rect":
					neverRectRequests++
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":10,"y":10,"width":20,"height":20}}`,
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

	element, err := parent.Find(
		context.Background(),
		appium.XPath(
			".//XCUIElementTypeButton",
		),
	)
	if err != nil {
		t.Fatalf(
			"find descendant element: %v",
			err,
		)
	}

	if element.ID() != "child/visible" {
		t.Fatalf(
			"unexpected element ID: expected %q, got %q",
			"child/visible",
			element.ID(),
		)
	}

	if neverRectRequests != 0 {
		t.Fatalf(
			"find must stop after first intersecting candidate: got %d later rect requests",
			neverRectRequests,
		)
	}

	requests := recorder.Requests()
	if len(requests) != 4 {
		t.Fatalf(
			"unexpected request count: expected 4, got %d",
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
		"/session/session%2Fid/element/parent%2F1/elements",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		requests[0],
		map[string]any{
			"using": "xpath",
			"value": ".//XCUIElementTypeButton",
		},
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchHeader(
		requests[0],
		"Content-Type",
		"application/json",
	); err != nil {
		t.Fatal(err)
	}

	expectedURIs := []string{
		"/session/session%2Fid/element/parent%2F1/elements",
		"/session/session%2Fid/window/rect",
		"/session/session%2Fid/element/child%2Foff/rect",
		"/session/session%2Fid/element/child%2Fvisible/rect",
	}

	for index, expectedURI := range expectedURIs {
		if err := contracttest.MatchRequestURI(
			requests[index],
			expectedURI,
		); err != nil {
			t.Fatal(err)
		}
	}
}

func TestElementFindEmptyResultsDoNotReadWindowRect(
	t *testing.T,
) {
	recorder, parent := newElementScopeTestParent(
		t,
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				switch {
				case request.Method == http.MethodPost &&
					request.RequestURI ==
						"/session/session%2Fid/element/parent%2F1/elements":
					_, _ = writer.Write(
						[]byte(`{"value":[]}`),
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

	elements, err := parent.FindElements(
		context.Background(),
		appium.ID("missing"),
	)
	if err != nil {
		t.Fatalf(
			"find missing descendant elements: %v",
			err,
		)
	}

	if elements == nil {
		t.Fatal(
			"expected non-nil empty element slice",
		)
	}

	if len(elements) != 0 {
		t.Fatalf(
			"unexpected element count: expected 0, got %d",
			len(elements),
		)
	}

	requests := recorder.Requests()
	if len(requests) != 1 {
		t.Fatalf(
			"empty FindElements must not read WindowRect: got %d requests",
			len(requests),
		)
	}

	if err := contracttest.MatchRequestURI(
		requests[0],
		"/session/session%2Fid/element/parent%2F1/elements",
	); err != nil {
		t.Fatal(err)
	}

	recorder.Reset()

	element, err := parent.Find(
		context.Background(),
		appium.ID("missing"),
	)
	if err == nil {
		t.Fatal(
			"expected element not found error",
		)
	}

	if element != nil {
		t.Fatalf(
			"missing descendant must not return element: %v",
			element,
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeElementNotFound,
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

	requests = recorder.Requests()
	if len(requests) != 1 {
		t.Fatalf(
			"empty Find must not read WindowRect: got %d requests",
			len(requests),
		)
	}
}

func TestElementFindElementsFiltersByWindowIntersection(
	t *testing.T,
) {
	recorder, parent := newElementScopeTestParent(
		t,
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				switch {
				case request.Method == http.MethodPost &&
					request.RequestURI ==
						"/session/session%2Fid/element/parent%2F1/elements":
					_, _ = writer.Write(
						[]byte(
							`{"value":[` +
								`{"element-6066-11e4-a52e-4f735466cecf":"child/first"},` +
								`{"element-6066-11e4-a52e-4f735466cecf":"child/off"},` +
								`{"element-6066-11e4-a52e-4f735466cecf":"child/third"}` +
								`]}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI ==
						"/session/session%2Fid/element/child%2Ffirst/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":10,"y":10,"width":20,"height":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI ==
						"/session/session%2Fid/element/child%2Foff/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":100,"y":10,"width":20,"height":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI ==
						"/session/session%2Fid/element/child%2Fthird/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":95,"y":90,"width":20,"height":20}}`,
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

	elements, err := parent.FindElements(
		context.Background(),
		appium.ClassName(
			"XCUIElementTypeButton",
		),
	)
	if err != nil {
		t.Fatalf(
			"find descendant elements: %v",
			err,
		)
	}

	if len(elements) != 2 {
		t.Fatalf(
			"unexpected element count: expected 2, got %d",
			len(elements),
		)
	}

	if elements[0].ID() != "child/first" {
		t.Fatalf(
			"unexpected first element ID: %q",
			elements[0].ID(),
		)
	}

	if elements[1].ID() != "child/third" {
		t.Fatalf(
			"unexpected second element ID: %q",
			elements[1].ID(),
		)
	}

	requests := recorder.Requests()
	if len(requests) != 5 {
		t.Fatalf(
			"unexpected request count: expected 5, got %d",
			len(requests),
		)
	}
}

func TestElementFindElementsFailsWhenCandidateRectIsInvalid(
	t *testing.T,
) {
	neverRectRequests := 0

	recorder, parent := newElementScopeTestParent(
		t,
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				switch {
				case request.Method == http.MethodPost &&
					request.RequestURI ==
						"/session/session%2Fid/element/parent%2F1/elements":
					_, _ = writer.Write(
						[]byte(
							`{"value":[` +
								`{"element-6066-11e4-a52e-4f735466cecf":"child/good"},` +
								`{"element-6066-11e4-a52e-4f735466cecf":"child/invalid"},` +
								`{"element-6066-11e4-a52e-4f735466cecf":"child/never"}` +
								`]}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI ==
						"/session/session%2Fid/element/child%2Fgood/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":10,"y":10,"width":20,"height":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI ==
						"/session/session%2Fid/element/child%2Finvalid/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":20,"y":20,"height":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI ==
						"/session/session%2Fid/element/child%2Fnever/rect":
					neverRectRequests++
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":30,"y":30,"width":20,"height":20}}`,
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

	elements, err := parent.FindElements(
		context.Background(),
		appium.ID("item"),
	)
	if err == nil {
		t.Fatal(
			"expected invalid candidate rect error",
		)
	}

	if elements != nil {
		t.Fatalf(
			"rect failure must not return partial elements: %v",
			elements,
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

	if neverRectRequests != 0 {
		t.Fatalf(
			"rect failure must stop candidate inspection: got %d later rect requests",
			neverRectRequests,
		)
	}

	requests := recorder.Requests()
	if len(requests) != 4 {
		t.Fatalf(
			"unexpected request count: expected 4, got %d",
			len(requests),
		)
	}
}

// newElementScopeTestParent 创建 Element 作用域查找测试使用的父元素。
//
// 父元素通过公开 Session.Find 获取，确保测试不依赖未导出的
// Element 构造细节。返回前会清空 Recorder，使每个测试只观察
// 当前 Element.Find 或 Element.FindElements 产生的请求。
func newElementScopeTestParent(
	t *testing.T,
	next http.Handler,
) (*contracttest.Recorder, *appium.Element) {
	t.Helper()

	handler := http.HandlerFunc(
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
				request.RequestURI == "/session/session%2Fid/elements":
				_, _ = writer.Write(
					[]byte(
						`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"parent/1"}]}`,
					),
				)

			case request.Method == http.MethodGet &&
				request.RequestURI == "/session/session%2Fid/window/rect":
				_, _ = writer.Write(
					[]byte(
						`{"value":{"x":0,"y":0,"width":100,"height":100}}`,
					),
				)

			case request.Method == http.MethodGet &&
				request.RequestURI ==
					"/session/session%2Fid/element/parent%2F1/rect":
				_, _ = writer.Write(
					[]byte(
						`{"value":{"x":10,"y":10,"width":80,"height":80}}`,
					),
				)

			default:
				if next != nil {
					next.ServeHTTP(
						writer,
						request,
					)
					return
				}

				http.NotFound(
					writer,
					request,
				)
			}
		},
	)

	recorder := contracttest.NewRecorder(handler)

	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)

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

	parent, err := session.Find(
		context.Background(),
		appium.ID("parent"),
	)
	if err != nil {
		t.Fatalf(
			"find parent element: %v",
			err,
		)
	}

	recorder.Reset()

	return recorder, parent
}
