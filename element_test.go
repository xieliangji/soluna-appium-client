package soluna_appium_client_test

import (
	"context"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestElementCommands(t *testing.T) {
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
							`{"value":{"sessionId":"session/id","capabilities":{"platformName":"iOS","automationName":"XCUITest"}}}`,
						),
					)

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session%2Fid/element":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"element-6066-11e4-a52e-4f735466cecf":"element/id"}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/element/element%2Fid/text":
					_, _ = writer.Write(
						[]byte(
							`{"value":"hello"}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/element/element%2Fid/attribute/name%2Fvalue":
					_, _ = writer.Write(
						[]byte(
							`{"value":"attribute-value"}`,
						),
					)

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session%2Fid/elements":
					_, _ = writer.Write(
						[]byte(
							`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"element/id"}]}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/window/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":0,"y":0,"width":390,"height":844}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/element/element%2Fid/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":10.5,"y":20.25,"width":100,"height":40}}`,
						),
					)

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session%2Fid/element/element%2Fid/clear":
					_, _ = writer.Write(
						[]byte(`{"value":null}`),
					)

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session%2Fid/element/element%2Fid/value":
					_, _ = writer.Write(
						[]byte(`{"value":null}`),
					)

				case request.Method == http.MethodDelete &&
					request.RequestURI == "/session/session%2Fid":
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
		appium.XPath(
			`//*[@name='login']`,
		),
	)
	if err != nil {
		t.Fatalf(
			"find element: %v",
			err,
		)
	}

	if element.ID() != "element/id" {
		t.Fatalf(
			"unexpected element ID: %q",
			element.ID(),
		)
	}

	// Find 的具体协议将在 window-aware 查找测试中单独验证。
	// 这里从此只验证已经获得 Element 后的元素命令。
	recorder.Reset()

	text, err := element.Text(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"get element text: %v",
			err,
		)
	}

	if text != "hello" {
		t.Fatalf(
			"unexpected element text: %q",
			text,
		)
	}

	attribute, exists, err := element.Attribute(
		context.Background(),
		"name/value",
	)
	if err != nil {
		t.Fatalf(
			"get element attribute: %v",
			err,
		)
	}

	if !exists {
		t.Fatal(
			"expected element attribute to exist",
		)
	}

	if attribute != "attribute-value" {
		t.Fatalf(
			"unexpected element attribute: %q",
			attribute,
		)
	}

	rect, err := element.Rect(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"get element rect: %v",
			err,
		)
	}

	expectedRect := appium.Rect{
		X:      10.5,
		Y:      20.25,
		Width:  100,
		Height: 40,
	}

	if rect != expectedRect {
		t.Fatalf(
			"unexpected element rect: expected %+v, got %+v",
			expectedRect,
			rect,
		)
	}

	if err := element.Clear(
		context.Background(),
	); err != nil {
		t.Fatalf(
			"clear element: %v",
			err,
		)
	}

	if err := element.SendKeys(
		context.Background(),
		"hello 世界",
	); err != nil {
		t.Fatalf(
			"send element keys: %v",
			err,
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

	requests := recorder.Requests()
	if len(requests) != 6 {
		t.Fatalf(
			"unexpected request count: expected 6, got %d",
			len(requests),
		)
	}

	if err := contracttest.MatchRequestURI(
		requests[0],
		"/session/session%2Fid/element/element%2Fid/text",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		requests[1],
		"/session/session%2Fid/element/element%2Fid/attribute/name%2Fvalue",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		requests[2],
		"/session/session%2Fid/element/element%2Fid/rect",
	); err != nil {
		t.Fatal(err)
	}

	clearRequest := requests[3]

	if err := contracttest.MatchMethod(
		clearRequest,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		clearRequest,
		"/session/session%2Fid/element/element%2Fid/clear",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		clearRequest,
		map[string]any{},
	); err != nil {
		t.Fatal(err)
	}

	sendKeysRequest := requests[4]

	if err := contracttest.MatchMethod(
		sendKeysRequest,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		sendKeysRequest,
		"/session/session%2Fid/element/element%2Fid/value",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		sendKeysRequest,
		map[string]any{
			"text": "hello 世界",
		},
	); err != nil {
		t.Fatal(err)
	}
}

func TestFindRejectsLegacyElementReference(t *testing.T) {
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
							`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
						),
					)

				case "/session/session/element":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"ELEMENT":"legacy-element"}}`,
						),
					)

				case "/session/session/elements":
					_, _ = writer.Write(
						[]byte(
							`{"value":[{"ELEMENT":"legacy-element"}]}`,
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

	_, err = session.Find(
		context.Background(),
		appium.ID("login"),
	)
	if err == nil {
		t.Fatal(
			"expected legacy element reference to be rejected",
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

func TestFindElements(t *testing.T) {
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
					request.RequestURI == "/session/session%2Fid/elements":
					_, _ = writer.Write(
						[]byte(
							`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"element/1"},{"element-6066-11e4-a52e-4f735466cecf":"element/2"}]}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/window/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":0,"y":0,"width":390,"height":844}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/element/element%2F1/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":10,"y":20,"width":100,"height":40}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/element/element%2F2/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":10,"y":100,"width":100,"height":40}}`,
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

	elements, err := session.FindElements(
		context.Background(),
		appium.XPath(
			"//XCUIElementTypeButton",
		),
	)
	if err != nil {
		t.Fatalf(
			"find elements: %v",
			err,
		)
	}

	if len(elements) != 2 {
		t.Fatalf(
			"unexpected element count: expected 2, got %d",
			len(elements),
		)
	}

	if elements[0].ID() != "element/1" {
		t.Fatalf(
			"unexpected first element ID: %q",
			elements[0].ID(),
		)
	}

	if elements[1].ID() != "element/2" {
		t.Fatalf(
			"unexpected second element ID: %q",
			elements[1].ID(),
		)
	}

	requests := recorder.Requests()
	if len(requests) == 0 {
		t.Fatal(
			"expected find elements request",
		)
	}

	request := requests[0]

	if err := contracttest.MatchMethod(
		request,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		request,
		"/session/session%2Fid/elements",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		request,
		map[string]any{
			"using": "xpath",
			"value": "//XCUIElementTypeButton",
		},
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchHeader(
		request,
		"Content-Type",
		"application/json",
	); err != nil {
		t.Fatal(err)
	}
}

func TestFindElementsReturnsEmptySlice(t *testing.T) {
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
					request.RequestURI == "/session/session/elements":
					_, _ = writer.Write(
						[]byte(
							`{"value":[]}`,
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

	elements, err := session.FindElements(
		context.Background(),
		appium.ID("missing"),
	)
	if err != nil {
		t.Fatalf(
			"find missing elements: %v",
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
}

func TestFindElementsRejectsInvalidElementReference(t *testing.T) {
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
					request.RequestURI == "/session/session/elements":
					_, _ = writer.Write(
						[]byte(
							`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"valid"},{"ELEMENT":"legacy"}]}`,
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

	elements, err := session.FindElements(
		context.Background(),
		appium.ID("item"),
	)
	if err == nil {
		t.Fatal(
			"expected invalid element reference error",
		)
	}

	if elements != nil {
		t.Fatalf(
			"invalid response must not return partial elements: %v",
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
}

func TestFindSkipsOffWindowCandidatesAndStopsAtFirstIntersection(t *testing.T) {
	neverRectRequests := 0

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
							`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"offscreen"},{"element-6066-11e4-a52e-4f735466cecf":"intersecting"},{"element-6066-11e4-a52e-4f735466cecf":"never"}]}`,
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
					request.RequestURI == "/session/session/element/offscreen/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":120,"y":10,"width":20,"height":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session/element/intersecting/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":90,"y":20,"width":20,"height":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session/element/never/rect":
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

	if element.ID() != "intersecting" {
		t.Fatalf(
			"unexpected element ID: %q",
			element.ID(),
		)
	}

	if neverRectRequests != 0 {
		t.Fatalf(
			"find must stop after first intersecting element: got %d later rect requests",
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

	expectedURIs := []string{
		"/session/session/elements",
		"/session/session/window/rect",
		"/session/session/element/offscreen/rect",
		"/session/session/element/intersecting/rect",
	}

	for index, expectedURI := range expectedURIs {
		if err := contracttest.MatchRequestURI(
			requests[index],
			expectedURI,
		); err != nil {
			t.Fatal(err)
		}
	}

	if err := contracttest.MatchJSONBody(
		requests[0],
		map[string]any{
			"using": "id",
			"value": "item",
		},
	); err != nil {
		t.Fatal(err)
	}
}

func TestFindRejectsCandidatesWithoutPositiveWindowIntersection(t *testing.T) {
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
							`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"right-edge"},{"element-6066-11e4-a52e-4f735466cecf":"bottom-edge"}]}`,
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
					request.RequestURI == "/session/session/element/right-edge/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":100,"y":20,"width":10,"height":10}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session/element/bottom-edge/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":20,"y":100,"width":10,"height":10}}`,
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

	element, err := session.Find(
		context.Background(),
		appium.ID("item"),
	)
	if err == nil {
		t.Fatal(
			"expected element not found error",
		)
	}

	if element != nil {
		t.Fatalf(
			"unexpected element: %v",
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
}

func TestFindElementsFiltersByWindowIntersection(t *testing.T) {
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
							`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"first"},{"element-6066-11e4-a52e-4f735466cecf":"offscreen"},{"element-6066-11e4-a52e-4f735466cecf":"partial"},{"element-6066-11e4-a52e-4f735466cecf":"zero-width"}]}`,
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
					request.RequestURI == "/session/session/element/first/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":10,"y":10,"width":20,"height":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session/element/offscreen/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":-30,"y":10,"width":20,"height":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session/element/partial/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":95,"y":20,"width":20,"height":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session/element/zero-width/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":50,"y":20,"width":0,"height":20}}`,
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

	elements, err := session.FindElements(
		context.Background(),
		appium.ID("item"),
	)
	if err != nil {
		t.Fatalf(
			"find elements: %v",
			err,
		)
	}

	if len(elements) != 2 {
		t.Fatalf(
			"unexpected element count: expected 2, got %d",
			len(elements),
		)
	}

	if elements[0].ID() != "first" {
		t.Fatalf(
			"unexpected first element ID: %q",
			elements[0].ID(),
		)
	}

	if elements[1].ID() != "partial" {
		t.Fatalf(
			"unexpected second element ID: %q",
			elements[1].ID(),
		)
	}
}

func TestFindElementsFailsWhenCandidateRectIsInvalid(t *testing.T) {
	neverRectRequests := 0

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
							`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"valid"},{"element-6066-11e4-a52e-4f735466cecf":"invalid"},{"element-6066-11e4-a52e-4f735466cecf":"never"}]}`,
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
					request.RequestURI == "/session/session/element/valid/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":10,"y":10,"width":20,"height":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session/element/invalid/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":10,"y":10,"width":20}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session/element/never/rect":
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

	elements, err := session.FindElements(
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
			"failed lookup must not return partial elements: %v",
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
			"lookup must stop after invalid candidate rect: got %d later rect requests",
			neverRectRequests,
		)
	}
}

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
