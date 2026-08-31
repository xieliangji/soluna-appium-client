package xcuitest_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
	"github.com/xieliangji/soluna-appium-client/xcuitest"
)

func TestIOSPressButtonProtocol(t *testing.T) {
	observer := &operationObserver{}
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
							`{"value":{"sessionId":"session","capabilities":{"platformName":"iOS","automationName":"XCUITest"}}}`,
						),
					)

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session/execute/sync":
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
		appium.ClientOptions{
			Observer: observer,
		},
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
	observer.reset()

	if err := xcuitest.IOSPressButton(
		context.Background(),
		session,
		xcuitest.IOSButtonHome,
	); err != nil {
		t.Fatalf(
			"press iOS button: %v",
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
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		request,
		"/session/session/execute/sync",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		request,
		map[string]any{
			"script": "mobile: pressButton",
			"args": []any{
				map[string]any{
					"name": "home",
				},
			},
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

	if len(observer.started) != 1 || len(observer.finished) != 1 {
		t.Fatalf(
			"unexpected observer event counts: started=%d finished=%d",
			len(observer.started),
			len(observer.finished),
		)
	}
	if observer.started[0].Operation != "ios_press_button" {
		t.Fatalf(
			"unexpected started operation: %q",
			observer.started[0].Operation,
		)
	}
	if observer.finished[0].Operation != "ios_press_button" {
		t.Fatalf(
			"unexpected finished operation: %q",
			observer.finished[0].Operation,
		)
	}
}

type operationObserver struct {
	started  []appium.CommandStartedEvent
	finished []appium.CommandFinishedEvent
}

func (o *operationObserver) OnCommandStarted(
	event appium.CommandStartedEvent,
) {
	o.started = append(o.started, event)
}

func (o *operationObserver) OnCommandFinished(
	event appium.CommandFinishedEvent,
) {
	o.finished = append(o.finished, event)
}

func (o *operationObserver) reset() {
	o.started = nil
	o.finished = nil
}

func TestIOSPressButtonRejectsNonXCUITestSession(t *testing.T) {
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
							`{"value":{"sessionId":"session","capabilities":{"platformName":"Android","automationName":"UiAutomator2"}}}`,
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
				"platformName":          "Android",
				"appium:automationName": "UiAutomator2",
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

	err = xcuitest.IOSPressButton(
		context.Background(),
		session,
		xcuitest.IOSButtonHome,
	)
	if err == nil {
		t.Fatal(
			"expected unsupported driver error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeUnsupported,
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

	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf(
			"driver mismatch must not reach remote: got %d requests",
			len(requests),
		)
	}
}

func TestIOSPressButtonRejectsInvalidButton(t *testing.T) {
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
							`{"value":{"sessionId":"session","capabilities":{"platformName":"iOS","automationName":"XCUITest"}}}`,
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

	err = xcuitest.IOSPressButton(
		context.Background(),
		session,
		xcuitest.IOSButton("HOME"),
	)
	if err == nil {
		t.Fatal(
			"expected invalid button error",
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

	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf(
			"invalid button must not reach remote: got %d requests",
			len(requests),
		)
	}
}

func TestIOSDeviceScreenInfoProtocol(t *testing.T) {
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
							`{"value":{"sessionId":"session","capabilities":{"platformName":"iOS","automationName":"XCUITest"}}}`,
						),
					)

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session/execute/sync":
					_, _ = writer.Write(
						[]byte(
							`{"value":{"statusBarSize":{"width":414,"height":48},"scale":3}}`,
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

	info, err := xcuitest.IOSDeviceScreenInfo(
		context.Background(),
		session,
	)
	if err != nil {
		t.Fatalf(
			"get iOS device screen info: %v",
			err,
		)
	}

	expected := xcuitest.ScreenInfo{
		StatusBarSize: xcuitest.ScreenSize{
			Width:  414,
			Height: 48,
		},
		Scale: 3,
	}

	if info != expected {
		t.Fatalf(
			"unexpected screen info: expected %+v, got %+v",
			expected,
			info,
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
		"/session/session/execute/sync",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		request,
		map[string]any{
			"script": "mobile: deviceScreenInfo",
			"args":   []any{},
		},
	); err != nil {
		t.Fatal(err)
	}
}

func TestIOSDeviceScreenInfoRejectsInvalidResponse(t *testing.T) {
	testCases := []struct {
		name     string
		response string
	}{
		{
			name: "missing status bar size",
			response: `{"value":{
				"scale":3
			}}`,
		},
		{
			name: "missing status bar width",
			response: `{"value":{
				"statusBarSize":{"height":48},
				"scale":3
			}}`,
		},
		{
			name: "missing status bar height",
			response: `{"value":{
				"statusBarSize":{"width":414},
				"scale":3
			}}`,
		},
		{
			name: "missing scale",
			response: `{"value":{
				"statusBarSize":{"width":414,"height":48}
			}}`,
		},
		{
			name: "invalid scale type",
			response: `{"value":{
				"statusBarSize":{"width":414,"height":48},
				"scale":"3"
			}}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
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
										`{"value":{"sessionId":"session","capabilities":{"platformName":"iOS","automationName":"XCUITest"}}}`,
									),
								)

							case request.Method == http.MethodPost &&
								request.RequestURI == "/session/session/execute/sync":
								_, _ = writer.Write(
									[]byte(testCase.response),
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

				info, err := xcuitest.IOSDeviceScreenInfo(
					context.Background(),
					session,
				)
				if err == nil {
					t.Fatal(
						"expected invalid screen info response error",
					)
				}

				if info != (xcuitest.ScreenInfo{}) {
					t.Fatalf(
						"invalid response must not return partial screen info: %+v",
						info,
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

				var appiumErr *appium.Error
				if !errors.As(
					err,
					&appiumErr,
				) {
					t.Fatalf(
						"expected structured appium error: %v",
						err,
					)
				}

				if appiumErr.Operation != "ios_device_screen_info" {
					t.Fatalf(
						"unexpected operation: %q",
						appiumErr.Operation,
					)
				}

				requests := recorder.Requests()
				if len(requests) != 1 {
					t.Fatalf(
						"unexpected request count: expected 1, got %d",
						len(requests),
					)
				}
			},
		)
	}
}
