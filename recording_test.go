package appium_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestRecordingProtocol(t *testing.T) {
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
					request.RequestURI == "/session/session%2Fid/appium/start_recording_screen":
					// StartRecording 可以忽略 Driver 返回的上一段录屏数据，
					// 但返回值仍然必须是合法字符串。
					_, _ = writer.Write(
						[]byte(
							`{"value":"cHJldmlvdXM="}`,
						),
					)

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session%2Fid/appium/stop_recording_screen":
					_, _ = writer.Write(
						[]byte(
							`{"value":"c29sdW5hLXJlY29yZGluZw=="}`,
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

	if err := session.StartRecording(
		context.Background(),
		appium.RecordingOptions{
			TimeLimit: 12 * time.Second,
		},
	); err != nil {
		t.Fatalf(
			"start recording: %v",
			err,
		)
	}

	var output bytes.Buffer

	written, err := session.StopRecordingTo(
		context.Background(),
		&output,
	)
	if err != nil {
		t.Fatalf(
			"stop recording: %v",
			err,
		)
	}

	expectedMedia := []byte(
		"soluna-recording",
	)

	if written != int64(len(expectedMedia)) {
		t.Fatalf(
			"unexpected written byte count: expected %d, got %d",
			len(expectedMedia),
			written,
		)
	}

	if !bytes.Equal(
		output.Bytes(),
		expectedMedia,
	) {
		t.Fatalf(
			"unexpected recording data: expected %q, got %q",
			expectedMedia,
			output.Bytes(),
		)
	}

	requests := recorder.Requests()
	if len(requests) != 3 {
		t.Fatalf(
			"unexpected request count: expected 3, got %d",
			len(requests),
		)
	}

	startRequest := requests[1]

	if err := contracttest.MatchMethod(
		startRequest,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		startRequest,
		"/session/session%2Fid/appium/start_recording_screen",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchHeader(
		startRequest,
		"Content-Type",
		"application/json",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		startRequest,
		map[string]any{
			"options": map[string]any{
				"timeLimit": 12,
			},
		},
	); err != nil {
		t.Fatal(err)
	}

	stopRequest := requests[2]

	if err := contracttest.MatchMethod(
		stopRequest,
		http.MethodPost,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		stopRequest,
		"/session/session%2Fid/appium/stop_recording_screen",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchHeader(
		stopRequest,
		"Content-Type",
		"application/json",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchJSONBody(
		stopRequest,
		map[string]any{},
	); err != nil {
		t.Fatal(err)
	}
}

func TestStopRecordingToMapsWriterFailureToOutputError(t *testing.T) {
	const encoded = "c29sdW5hLXJlY29yZGluZw=="

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
				case "/session/session/appium/stop_recording_screen":
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

	failure := errors.New("recording destination is closed")
	destination := &recordingOutputErrorWriter{err: failure}
	written, err := session.StopRecordingTo(
		context.Background(),
		destination,
	)
	if err == nil {
		t.Fatal("expected destination writer failure")
	}
	if written != 0 {
		t.Fatalf("unexpected written count: %d", written)
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
	if clientErr.Message == "WebDriver response value is invalid" {
		t.Fatal("writer failure must not use response-invalid message")
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
}

type recordingOutputErrorWriter struct {
	err error
}

func (w *recordingOutputErrorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestRecordingRejectsInvalidArgumentsBeforeDelivery(t *testing.T) {
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

	recorder.Reset()

	err = session.StartRecording(
		context.Background(),
		appium.RecordingOptions{
			TimeLimit: 1500 * time.Millisecond,
		},
	)
	if err == nil {
		t.Fatal(
			"expected invalid recording time limit error",
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
			"invalid time limit must not be delivered: got %d requests",
			len(requests),
		)
	}

	_, err = session.StopRecordingTo(
		context.Background(),
		nil,
	)
	if err == nil {
		t.Fatal(
			"expected nil recording writer error",
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
			"nil writer must not be delivered: got %d requests",
			len(requests),
		)
	}
}
