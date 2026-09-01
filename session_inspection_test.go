package appium_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestSessionInspectionCommands(t *testing.T) {
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
							`{"value":{"x":-10.5,"y":20.25,"width":390,"height":844}}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/screenshot":
					_, _ = writer.Write(
						[]byte(
							`{"value":"iVBORw=="}`,
						),
					)

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/source":
					_, _ = writer.Write(
						[]byte(
							`{"value":"<App><Button name=\"登录\"/></App>"}`,
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

	rect, err := session.WindowRect(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"get window rect: %v",
			err,
		)
	}

	expectedRect := appium.Rect{
		X:      -10.5,
		Y:      20.25,
		Width:  390,
		Height: 844,
	}

	if rect != expectedRect {
		t.Fatalf(
			"unexpected window rect: expected %+v, got %+v",
			expectedRect,
			rect,
		)
	}

	screenshot, err := session.Screenshot(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"get screenshot: %v",
			err,
		)
	}

	expectedScreenshot := []byte{
		0x89,
		0x50,
		0x4e,
		0x47,
	}

	if !bytes.Equal(
		screenshot,
		expectedScreenshot,
	) {
		t.Fatalf(
			"unexpected screenshot data: expected %v, got %v",
			expectedScreenshot,
			screenshot,
		)
	}

	source, err := session.PageSource(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"get page source: %v",
			err,
		)
	}

	const expectedSource = `<App><Button name="登录"/></App>`

	if source != expectedSource {
		t.Fatalf(
			"unexpected page source: expected %q, got %q",
			expectedSource,
			source,
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
		"/session/session%2Fid/window/rect",
		"/session/session%2Fid/screenshot",
		"/session/session%2Fid/source",
	}

	for index, expectedURI := range expectedURIs {
		request := requests[index+1]

		if err := contracttest.MatchMethod(
			request,
			http.MethodGet,
		); err != nil {
			t.Fatal(err)
		}

		if err := contracttest.MatchRequestURI(
			request,
			expectedURI,
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
				"unexpected Content-Type header for %s: %v",
				expectedURI,
				values,
			)
		}
	}
}

func TestSessionInspectionRejectsInvalidValues(t *testing.T) {
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

				case "/session/session/window/rect":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"x":0,"y":0,"width":-1,"height":100}}`,
						),
					)

				case "/session/session/screenshot":
					_, _ = writer.Write(
						[]byte(
							`{"value":"***"}`,
						),
					)

				case "/session/session/source":
					_, _ = writer.Write(
						[]byte(
							`{"value":123}`,
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

	assertInvalidResponse := func(
		name string,
		err error,
	) {
		t.Helper()

		if err == nil {
			t.Fatalf(
				"%s: expected invalid response error",
				name,
			)
		}

		if !appium.IsErrorCode(
			err,
			appium.CodeResponseInvalid,
		) {
			t.Fatalf(
				"%s: unexpected error code: %v",
				name,
				err,
			)
		}

		if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryAcknowledged {
			t.Fatalf(
				"%s: unexpected delivery state: expected %q, got %q",
				name,
				appium.DeliveryAcknowledged,
				delivery,
			)
		}
	}

	_, err = session.WindowRect(
		context.Background(),
	)
	assertInvalidResponse(
		"window rect",
		err,
	)

	_, err = session.Screenshot(
		context.Background(),
	)
	assertInvalidResponse(
		"screenshot",
		err,
	)

	_, err = session.PageSource(
		context.Background(),
	)
	assertInvalidResponse(
		"page source",
		err,
	)
}
