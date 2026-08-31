package appium_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestElementScreenshotToProtocolAndScreenshotReuseDecoder(t *testing.T) {
	fixture := newElementScreenshotFixture(
		t,
		appium.ClientOptions{},
		nil,
	)

	var output bytes.Buffer
	written, err := fixture.element.ScreenshotTo(
		context.Background(),
		&output,
	)
	if err != nil {
		t.Fatalf("get element screenshot to writer: %v", err)
	}

	expected := []byte("soluna-element")
	if written != int64(len(expected)) {
		t.Fatalf(
			"unexpected written count: expected %d, got %d",
			len(expected),
			written,
		)
	}
	if !bytes.Equal(output.Bytes(), expected) {
		t.Fatalf(
			"unexpected streamed screenshot: expected %q, got %q",
			expected,
			output.Bytes(),
		)
	}

	screenshot, err := fixture.element.Screenshot(
		context.Background(),
	)
	if err != nil {
		t.Fatalf("get element screenshot: %v", err)
	}
	if !bytes.Equal(screenshot, expected) {
		t.Fatalf(
			"unexpected screenshot: expected %q, got %q",
			expected,
			screenshot,
		)
	}

	requests := fixture.recorder.Requests()
	if len(requests) != 2 {
		t.Fatalf(
			"unexpected request count: expected 2, got %d",
			len(requests),
		)
	}

	for _, request := range requests {
		if err := contracttest.MatchMethod(request, http.MethodGet); err != nil {
			t.Fatal(err)
		}
		if err := contracttest.MatchRequestURI(
			request,
			"/session/session%2Fid/element/element%2Fid/screenshot",
		); err != nil {
			t.Fatal(err)
		}
		if err := contracttest.MatchBody(request, nil); err != nil {
			t.Fatal(err)
		}
		if values := request.Header.Values("Content-Type"); len(values) != 0 {
			t.Fatalf("unexpected Content-Type header: %v", values)
		}
	}
}

func TestElementScreenshotPropagatesStaleRemoteError(t *testing.T) {
	fixture := newElementScreenshotFixture(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(writer).Encode(
					map[string]any{
						"value": map[string]any{
							"error":   "stale element reference",
							"message": "element is no longer attached",
						},
					},
				)
			},
		),
	)

	_, err := fixture.element.Screenshot(
		context.Background(),
	)
	if err == nil {
		t.Fatal("expected stale element error")
	}
	if !appium.IsErrorCode(err, appium.CodeElementStale) {
		t.Fatalf("unexpected error code: %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}

	var clientErr *appium.Error
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected structured client error: %v", err)
	}
	if clientErr.Operation != "element_screenshot" {
		t.Fatalf("unexpected operation: %q", clientErr.Operation)
	}
	if clientErr.RemoteCode != "stale element reference" {
		t.Fatalf("unexpected remote code: %q", clientErr.RemoteCode)
	}

	if requests := fixture.recorder.Requests(); len(requests) != 1 {
		t.Fatalf("stale screenshot must not be retried: got %d requests", len(requests))
	}
}

func TestElementScreenshotPropagatesGenericRemoteError(t *testing.T) {
	fixture := newElementScreenshotFixture(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.WriteHeader(http.StatusNotImplemented)
				_ = json.NewEncoder(writer).Encode(
					map[string]any{
						"value": map[string]any{
							"error":   "unknown command",
							"message": "element screenshot is unsupported",
						},
					},
				)
			},
		),
	)

	_, err := fixture.element.ScreenshotTo(
		context.Background(),
		&bytes.Buffer{},
	)
	if err == nil {
		t.Fatal("expected remote command error")
	}
	if !appium.IsErrorCode(err, appium.CodeUnsupported) {
		t.Fatalf("unexpected error code: %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
}

func TestElementScreenshotToReportsDecodeAndWriterFailures(t *testing.T) {
	t.Run("invalid base64", func(t *testing.T) {
		fixture := newElementScreenshotFixture(
			t,
			appium.ClientOptions{},
			http.HandlerFunc(
				func(
					writer http.ResponseWriter,
					request *http.Request,
				) {
					_, _ = writer.Write([]byte(`{"value":"c29sdW5h$="}`))
				},
			),
		)

		var output bytes.Buffer
		written, err := fixture.element.ScreenshotTo(
			context.Background(),
			&output,
		)
		if err == nil {
			t.Fatal("expected invalid base64 error")
		}
		if written != 0 || output.Len() != 0 {
			t.Fatalf(
				"invalid base64 should not write data: count=%d output=%q",
				written,
				output.Bytes(),
			)
		}
		if !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
			t.Fatalf("unexpected error code: %v", err)
		}
		if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
			t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
		}
	})

	t.Run("writer failure", func(t *testing.T) {
		fixture := newElementScreenshotFixture(
			t,
			appium.ClientOptions{},
			http.HandlerFunc(
				func(
					writer http.ResponseWriter,
					request *http.Request,
				) {
					_, _ = writer.Write([]byte(`{"value":"c29sdW5hLWVsZW1lbnQ="}`))
				},
			),
		)

		failure := errors.New("destination is full")
		const limit = 7
		destination := &screenshotFailingWriter{
			limit: limit,
			err:   failure,
		}

		written, err := fixture.element.ScreenshotTo(
			context.Background(),
			destination,
		)
		if err == nil {
			t.Fatal("expected destination writer failure")
		}
		if written != limit {
			t.Fatalf("unexpected partial count: expected %d, got %d", limit, written)
		}
		if !bytes.Equal(destination.data, []byte("soluna-")) {
			t.Fatalf("unexpected partial data: %q", destination.data)
		}
		if !appium.IsErrorCode(err, appium.CodeOutputFailed) {
			t.Fatalf("unexpected error code: %v", err)
		}
		if !errors.Is(err, failure) {
			t.Fatalf("writer error was not preserved: %v", err)
		}
		var clientErr *appium.Error
		if !errors.As(err, &clientErr) {
			t.Fatalf("expected structured client error: %v", err)
		}
		if clientErr.Cause != failure {
			t.Fatalf("writer cause was not preserved: got %v", clientErr.Cause)
		}
		if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
			t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
		}
	})
}

func TestElementScreenshotUsesLimitAndRejectsNilWriter(t *testing.T) {
	const response = `{"value":"c29sdW5h"}`

	fixture := newElementScreenshotFixture(
		t,
		appium.ClientOptions{
			Limits: appium.Limits{
				MaxScreenshotResponseBytes: int64(len(response)) - 1,
			},
		},
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				_, _ = writer.Write([]byte(response))
			},
		),
	)

	_, err := fixture.element.Screenshot(context.Background())
	if err == nil {
		t.Fatal("expected screenshot response limit error")
	}
	if !appium.IsErrorCode(err, appium.CodeResponseTooLarge) {
		t.Fatalf("unexpected error code: %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}

	fixture.recorder.Reset()
	_, err = fixture.element.ScreenshotTo(context.Background(), nil)
	if err == nil {
		t.Fatal("expected nil writer error")
	}
	if !appium.IsErrorCode(err, appium.CodeInvalidArgument) {
		t.Fatalf("unexpected error code: %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
	if requests := fixture.recorder.Requests(); len(requests) != 0 {
		t.Fatalf("nil writer must not send request: got %d", len(requests))
	}
}

func newElementScreenshotFixture(
	t *testing.T,
	options appium.ClientOptions,
	screenshotHandler http.Handler,
) struct {
	recorder *contracttest.Recorder
	element  *appium.Element
} {
	t.Helper()

	const (
		sessionURI = "/session/session%2Fid"
		elementURI = sessionURI + "/element/element%2Fid"
	)

	recorder := contracttest.NewRecorder(
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.Header().Set("Content-Type", "application/json")

				switch request.RequestURI {
				case "/session":
					_, _ = writer.Write([]byte(
						`{"value":{"sessionId":"session/id","capabilities":{"automationName":"XCUITest"}}}`,
					))

				case sessionURI + "/elements":
					_, _ = writer.Write([]byte(
						`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"element/id"}]}`,
					))

				case sessionURI + "/window/rect":
					_, _ = writer.Write([]byte(
						`{"value":{"x":0,"y":0,"width":390,"height":844}}`,
					))

				case elementURI + "/rect":
					_, _ = writer.Write([]byte(
						`{"value":{"x":10,"y":20,"width":100,"height":40}}`,
					))

				case elementURI + "/screenshot":
					if screenshotHandler == nil {
						_, _ = writer.Write([]byte(
							`{"value":"c29sdW5hLWVsZW1lbnQ="}`,
						))
						return
					}
					screenshotHandler.ServeHTTP(writer, request)

				default:
					http.NotFound(writer, request)
				}
			},
		),
	)

	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)

	client, err := server.NewClient(options)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	session, err := client.CreateSession(
		context.Background(),
		appium.MatchCapabilities(appium.Capabilities{
			"platformName":          "iOS",
			"appium:automationName": "XCUITest",
		}),
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	element, err := session.Find(
		context.Background(),
		appium.ID("target"),
	)
	if err != nil {
		t.Fatalf("find element: %v", err)
	}

	recorder.Reset()

	return struct {
		recorder *contracttest.Recorder
		element  *appium.Element
	}{
		recorder: recorder,
		element:  element,
	}
}
