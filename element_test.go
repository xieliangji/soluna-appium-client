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
	if len(requests) != 8 {
		t.Fatalf(
			"unexpected request count: expected 8, got %d",
			len(requests),
		)
	}

	findRequest := requests[1]

	if err := contracttest.MatchMethod(
		findRequest,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		findRequest,
		"/session/session%2Fid/element",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		findRequest,
		map[string]any{
			"using": "xpath",
			"value": `//*[@name='login']`,
		},
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchHeader(
		findRequest,
		"Content-Type",
		"application/json",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		requests[2],
		"/session/session%2Fid/element/element%2Fid/text",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		requests[3],
		"/session/session%2Fid/element/element%2Fid/attribute/name%2Fvalue",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		requests[4],
		"/session/session%2Fid/element/element%2Fid/rect",
	); err != nil {
		t.Fatal(err)
	}

	clearRequest := requests[5]

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

	sendKeysRequest := requests[6]

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
	if len(requests) != 1 {
		t.Fatalf(
			"unexpected request count: expected 1, got %d",
			len(requests),
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
