package appium_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestScreenshotToProtocolAndScreenshotReuseDecoder(t *testing.T) {
	const encoded = "c29sdW5hLXNjcmVlbnNob3Q="

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
					request.RequestURI == "/session/session%2Fid/screenshot":
					_, _ = writer.Write(
						[]byte(
							`{"value":"` + encoded + `"}`,
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

	expected := []byte("soluna-screenshot")

	var output bytes.Buffer
	written, err := session.ScreenshotTo(
		context.Background(),
		&output,
	)
	if err != nil {
		t.Fatalf(
			"get screenshot to writer: %v",
			err,
		)
	}
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

	screenshot, err := session.Screenshot(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"get screenshot: %v",
			err,
		)
	}
	if !bytes.Equal(screenshot, expected) {
		t.Fatalf(
			"unexpected screenshot: expected %q, got %q",
			expected,
			screenshot,
		)
	}

	requests := recorder.Requests()
	if len(requests) != 3 {
		t.Fatalf(
			"unexpected request count: expected 3, got %d",
			len(requests),
		)
	}

	for _, request := range requests[1:] {
		if err := contracttest.MatchMethod(
			request,
			http.MethodGet,
		); err != nil {
			t.Fatal(err)
		}
		if err := contracttest.MatchRequestURI(
			request,
			"/session/session%2Fid/screenshot",
		); err != nil {
			t.Fatal(err)
		}
		if err := contracttest.MatchBody(request, nil); err != nil {
			t.Fatal(err)
		}
		if values := request.Header.Values("Content-Type"); len(values) != 0 {
			t.Fatalf(
				"unexpected Content-Type header: %v",
				values,
			)
		}
	}
}

func TestScreenshotToReportsPartialWriterFailure(t *testing.T) {
	const (
		encoded = "c29sdW5hLXNjcmVlbnNob3Q="
		limit   = 7
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
						`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
					))
				case "/session/session/screenshot":
					_, _ = writer.Write([]byte(`{"value":"` + encoded + `"}`))
				default:
					http.NotFound(writer, request)
				}
			},
		),
	)
	server := contracttest.NewServer(recorder)
	defer server.Close()

	client, err := server.NewClient(appium.ClientOptions{})
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

	failure := errors.New("destination is full")
	destination := &screenshotFailingWriter{
		limit: limit,
		err:   failure,
	}
	written, err := session.ScreenshotTo(
		context.Background(),
		destination,
	)
	if err == nil {
		t.Fatal("expected destination writer failure")
	}
	if written != int64(limit) {
		t.Fatalf(
			"unexpected partial count: expected %d, got %d",
			limit,
			written,
		)
	}
	if !bytes.Equal(destination.data, []byte("soluna-")) {
		t.Fatalf(
			"unexpected partial data: %q",
			destination.data,
		)
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
}

func TestScreenshotToReportsContextEndingDuringDecode(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 64<<10)
	encoded := base64.StdEncoding.EncodeToString(data)

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
						`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
					))
				case "/session/session/screenshot":
					_, _ = writer.Write([]byte(`{"value":"` + encoded + `"}`))
				default:
					http.NotFound(writer, request)
				}
			},
		),
	)
	server := contracttest.NewServer(recorder)
	defer server.Close()

	client, err := server.NewClient(appium.ClientOptions{})
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	destination := &screenshotCancelingWriter{
		cancel: cancel,
	}
	written, err := session.ScreenshotTo(ctx, destination)
	if err == nil {
		t.Fatal("expected context cancellation")
	}
	if written == 0 {
		t.Fatal("expected some screenshot data before cancellation")
	}
	if !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("unexpected error code: %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
}

func TestScreenshotUsesDedicatedResponseLimit(t *testing.T) {
	response := `{"value":"c29sdW5h"}`
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
						`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
					))
				case "/session/session/screenshot":
					_, _ = writer.Write([]byte(response))
				default:
					http.NotFound(writer, request)
				}
			},
		),
	)
	server := contracttest.NewServer(recorder)
	defer server.Close()

	client, err := server.NewClient(appium.ClientOptions{
		Limits: appium.Limits{
			MaxResponseBytes:           1 << 20,
			MaxScreenshotResponseBytes: int64(len(response)) - 1,
		},
	})
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

	_, err = session.Screenshot(context.Background())
	if err == nil {
		t.Fatal("expected screenshot response limit error")
	}
	if !appium.IsErrorCode(err, appium.CodeResponseTooLarge) {
		t.Fatalf("unexpected error code: %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
}

func TestScreenshotToRejectsNilWriterBeforeDelivery(t *testing.T) {
	recorder := contracttest.NewRecorder(
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.Header().Set("Content-Type", "application/json")
				if request.RequestURI == "/session" {
					_, _ = writer.Write([]byte(
						`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
					))
					return
				}
				http.NotFound(writer, request)
			},
		),
	)
	server := contracttest.NewServer(recorder)
	defer server.Close()

	client, err := server.NewClient(appium.ClientOptions{})
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
	recorder.Reset()

	_, err = session.ScreenshotTo(context.Background(), nil)
	if err == nil {
		t.Fatal("expected nil writer error")
	}
	if !appium.IsErrorCode(err, appium.CodeInvalidArgument) {
		t.Fatalf("unexpected error code: %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("nil writer must not send request: got %d", len(requests))
	}
}

func TestNewClientRejectsNegativeScreenshotLimit(t *testing.T) {
	client, err := appium.NewClient(
		"http://127.0.0.1:4723",
		appium.ClientOptions{
			Limits: appium.Limits{
				MaxScreenshotResponseBytes: -1,
			},
		},
	)
	if err == nil {
		t.Fatal("expected invalid screenshot limit error")
	}
	if client != nil {
		t.Fatal("invalid screenshot limit must not return a client")
	}
	if !appium.IsErrorCode(err, appium.CodeInvalidConfig) {
		t.Fatalf("unexpected error code: %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
}

type screenshotFailingWriter struct {
	limit int
	err   error
	data  []byte
}

func (w *screenshotFailingWriter) Write(p []byte) (int, error) {
	remaining := w.limit - len(w.data)
	if remaining <= 0 {
		return 0, w.err
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	w.data = append(w.data, p...)
	return len(p), w.err
}

type screenshotCancelingWriter struct {
	cancel context.CancelFunc
	data   []byte
}

func (w *screenshotCancelingWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	w.cancel()
	return len(p), nil
}

var _ io.Writer = (*screenshotFailingWriter)(nil)
var _ io.Writer = (*screenshotCancelingWriter)(nil)
