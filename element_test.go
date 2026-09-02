package appium_test

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

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/context":
					_, _ = writer.Write(
						[]byte(`{"value":"NATIVE_APP"}`),
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
