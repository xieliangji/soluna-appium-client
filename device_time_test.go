package appium_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptrace"
	"sync/atomic"
	"testing"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

const (
	dp121DeviceTimePath   = "/session/session%2Fid/appium/device/system_time"
	dp121DeviceTimeLayout = "2006-01-02T15:04:05-07:00"
)

func TestDP121DeviceTimeReturnsValidatedFreshSnapshots(t *testing.T) {
	for _, automationName := range []string{
		"XCUITest",
		"UiAutomator2",
		"OtherDriver",
	} {
		t.Run(automationName, func(t *testing.T) {
			responses := []string{
				"2026-09-04T16:07:08+08:00",
				"2024-02-29T23:59:59-03:30",
				"2025-01-01T00:00:00+00:00",
			}
			expected := []time.Time{
				time.Date(
					2026,
					time.September,
					4,
					16,
					7,
					8,
					0,
					time.FixedZone("expected", 8*60*60),
				),
				time.Date(
					2024,
					time.February,
					29,
					23,
					59,
					59,
					0,
					time.FixedZone("expected", -(3*60+30)*60),
				),
				time.Date(
					2025,
					time.January,
					1,
					0,
					0,
					0,
					0,
					time.UTC,
				),
			}

			var reads atomic.Int32
			session, recorder := newDP121DeviceTimeSession(
				t,
				automationName,
				appium.ClientOptions{},
				http.HandlerFunc(func(
					writer http.ResponseWriter,
					request *http.Request,
				) {
					if request.Method != http.MethodGet ||
						request.RequestURI != dp121DeviceTimePath {
						http.NotFound(writer, request)
						return
					}

					index := int(reads.Add(1)) - 1
					if index >= len(responses) {
						http.Error(
							writer,
							"unexpected extra device time read",
							http.StatusInternalServerError,
						)
						return
					}

					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(
						`{"value":"` + responses[index] + `"}`,
					))
				}),
			)

			for index, want := range expected {
				got, err := session.DeviceTime(context.Background())
				if err != nil {
					t.Fatalf("device time snapshot %d: %v", index, err)
				}
				if !got.Equal(want) {
					t.Fatalf(
						"device time snapshot %d = %v, want instant %v",
						index,
						got,
						want,
					)
				}
				if formatted := got.Format(dp121DeviceTimeLayout); formatted != responses[index] {
					t.Fatalf(
						"device time snapshot %d format = %q, want %q",
						index,
						formatted,
						responses[index],
					)
				}
			}

			requests := recorder.Requests()
			if len(requests) != len(expected) {
				t.Fatalf(
					"request count = %d, want %d",
					len(requests),
					len(expected),
				)
			}

			for index, request := range requests {
				if err := contracttest.MatchMethod(
					request,
					http.MethodGet,
				); err != nil {
					t.Fatalf("request %d method: %v", index, err)
				}
				if err := contracttest.MatchRequestURI(
					request,
					dp121DeviceTimePath,
				); err != nil {
					t.Fatalf("request %d URI: %v", index, err)
				}
				if err := contracttest.MatchBody(request, nil); err != nil {
					t.Fatalf("request %d body: %v", index, err)
				}
				if values := request.Header.Values("Content-Type"); len(values) != 0 {
					t.Fatalf(
						"request %d Content-Type = %q, want absent",
						index,
						values,
					)
				}
			}
		})
	}
}

func TestDP121DeviceTimeRejectsInvalidResultsWithoutFormatGuessing(t *testing.T) {
	for _, test := range []struct {
		name     string
		response string
	}{
		{name: "null", response: `null`},
		{name: "boolean", response: `true`},
		{name: "number", response: `1`},
		{name: "object", response: `{}`},
		{name: "array", response: `[]`},
		{name: "empty string", response: `""`},
		{name: "UTC designator", response: `"2026-09-04T16:07:08Z"`},
		{name: "compact offset", response: `"2026-09-04T16:07:08+0800"`},
		{name: "fractional seconds", response: `"2026-09-04T16:07:08.123+08:00"`},
		{name: "space separator", response: `"2026-09-04 16:07:08+08:00"`},
		{name: "leading whitespace", response: `" 2026-09-04T16:07:08+08:00"`},
		{name: "trailing whitespace", response: `"2026-09-04T16:07:08+08:00 "`},
		{name: "invalid month", response: `"2026-13-04T16:07:08+08:00"`},
		{name: "invalid leap day", response: `"2025-02-29T16:07:08+08:00"`},
		{name: "leap second", response: `"2026-09-04T16:07:60+08:00"`},
		{name: "offset hour out of range", response: `"2026-09-04T16:07:08+24:00"`},
		{name: "offset minute out of range", response: `"2026-09-04T16:07:08+08:60"`},
		{name: "negative zero offset", response: `"2026-09-04T16:07:08-00:00"`},
		{name: "unpaired surrogate", response: `"\ud800"`},
		{name: "invalid UTF-8", response: "\"2026-09-04T16:07:08+08:\xff0\""},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, recorder := newDP121DeviceTimeSession(
				t,
				"XCUITest",
				appium.ClientOptions{},
				http.HandlerFunc(func(
					writer http.ResponseWriter,
					request *http.Request,
				) {
					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(
						`{"value":` + test.response + `}`,
					))
				}),
			)

			got, err := session.DeviceTime(context.Background())
			if !got.IsZero() {
				t.Fatalf("device time on failure = %v, want zero", got)
			}
			assertDP121ResponseInvalid(t, err)
			if requests := recorder.Requests(); len(requests) != 1 {
				t.Fatalf("request count = %d, want 1", len(requests))
			}
		})
	}
}

func TestDP121DeviceTimeDecoderFinishesInsideObservedCommand(t *testing.T) {
	observer := &commandObserverRecorder{}
	session, recorder := newDP121DeviceTimeSession(
		t,
		"UiAutomator2",
		appium.ClientOptions{Observer: observer},
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(
				`{"value":"raw-device-output"}`,
			))
		}),
	)
	observer.reset()
	recorder.Reset()

	_, err := session.DeviceTime(context.Background())
	assertDP121ResponseInvalid(t, err)

	started, finished := observer.snapshot()
	if len(started) != 1 || len(finished) != 1 {
		t.Fatalf(
			"observer events = started %d, finished %d; want 1, 1",
			len(started),
			len(finished),
		)
	}
	if started[0].Operation != "get_device_time" ||
		finished[0].Operation != "get_device_time" {
		t.Fatalf(
			"observer operations = %q, %q; want get_device_time",
			started[0].Operation,
			finished[0].Operation,
		)
	}
	if finished[0].ErrorCode != appium.CodeResponseInvalid {
		t.Fatalf(
			"finished error code = %q, want response_invalid",
			finished[0].ErrorCode,
		)
	}
	if finished[0].Delivery != appium.DeliveryAcknowledged {
		t.Fatalf(
			"finished delivery = %q, want acknowledged",
			finished[0].Delivery,
		)
	}
}

func TestDP121DeviceTimeRemoteFailureDoesNotFallbackToHost(t *testing.T) {
	for _, automationName := range []string{"XCUITest", "UiAutomator2"} {
		t.Run(automationName, func(t *testing.T) {
			session, recorder := newDP121DeviceTimeSession(
				t,
				automationName,
				appium.ClientOptions{},
				http.HandlerFunc(func(
					writer http.ResponseWriter,
					request *http.Request,
				) {
					writer.Header().Set("Content-Type", "application/json")
					if request.RequestURI == dp121DeviceTimePath {
						writer.WriteHeader(http.StatusNotFound)
						_, _ = writer.Write([]byte(
							`{"value":{"error":"unknown command","message":"device time unavailable"}}`,
						))
						return
					}

					// Any alternate compatibility path would appear successful.
					_, _ = writer.Write([]byte(
						`{"value":"2026-09-04T16:07:08+08:00"}`,
					))
				}),
			)

			got, err := session.DeviceTime(context.Background())
			if !got.IsZero() {
				t.Fatalf("device time on failure = %v, want zero", got)
			}
			if err == nil || !appium.IsErrorCode(err, appium.CodeUnsupported) {
				t.Fatalf("error = %v, want unsupported", err)
			}
			if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
				t.Fatalf(
					"delivery = %q, want acknowledged",
					appium.DeliveryOf(err),
				)
			}

			var clientErr *appium.Error
			if !errors.As(err, &clientErr) {
				t.Fatalf("error type = %T, want *appium.Error", err)
			}
			if clientErr.Operation != "get_device_time" {
				t.Fatalf(
					"operation = %q, want get_device_time",
					clientErr.Operation,
				)
			}

			requests := recorder.Requests()
			if len(requests) != 1 {
				t.Fatalf(
					"remote failure triggered fallback: got %d requests, want 1",
					len(requests),
				)
			}
			if requests[0].RequestURI != dp121DeviceTimePath {
				t.Fatalf(
					"request URI = %q, want %q",
					requests[0].RequestURI,
					dp121DeviceTimePath,
				)
			}
		})
	}
}

func TestDP121DeviceTimeUnknownDeliveryIsNotReplayed(t *testing.T) {
	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	t.Cleanup(baseTransport.CloseIdleConnections)

	var calls atomic.Int32
	httpClient := &http.Client{
		Transport: dp121RoundTripperFunc(func(
			request *http.Request,
		) (*http.Response, error) {
			if request.Method != http.MethodGet ||
				request.URL.EscapedPath() != dp121DeviceTimePath {
				return baseTransport.RoundTrip(request)
			}

			calls.Add(1)

			// Inject one deterministic attempted transport failure and report that it
			// reached the write boundary so the unified chain records unknown delivery.
			trace := httptrace.ContextClientTrace(request.Context())
			if trace != nil && trace.WroteRequest != nil {
				trace.WroteRequest(httptrace.WroteRequestInfo{})
			}

			return nil, errors.New(
				"synthetic transport failure after request attempt",
			)
		}),
	}

	session, recorder := newDP121DeviceTimeSession(
		t,
		"UiAutomator2",
		appium.ClientOptions{HTTPClient: httpClient},
		http.NotFoundHandler(),
	)

	got, err := session.DeviceTime(context.Background())
	if !got.IsZero() {
		t.Fatalf("device time on failure = %v, want zero", got)
	}
	if err == nil || !appium.IsErrorCode(err, appium.CodeTransportFailed) {
		t.Fatalf("error = %v, want transport failure", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryUnknown {
		t.Fatalf("delivery = %q, want unknown", appium.DeliveryOf(err))
	}
	if calls.Load() != 1 {
		t.Fatalf("transport call count = %d, want 1", calls.Load())
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf(
			"unknown delivery triggered %d fallback server requests, want 0",
			len(requests),
		)
	}
}

func TestDP121DeviceTimeCanceledBeforeDelivery(t *testing.T) {
	session, recorder := newDP121DeviceTimeSession(
		t,
		"XCUITest",
		appium.ClientOptions{},
		http.NotFoundHandler(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := session.DeviceTime(ctx)
	if !got.IsZero() {
		t.Fatalf("device time on cancellation = %v, want zero", got)
	}
	if err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("error = %v, want canceled", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf(
			"delivery = %q, want not_sent",
			appium.DeliveryOf(err),
		)
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("canceled read sent %d requests, want 0", len(requests))
	}
}

func assertDP121ResponseInvalid(t *testing.T, err error) {
	t.Helper()

	if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
		t.Fatalf("error = %v, want response invalid", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf(
			"delivery = %q, want acknowledged",
			appium.DeliveryOf(err),
		)
	}

	var clientErr *appium.Error
	if !errors.As(err, &clientErr) {
		t.Fatalf("error type = %T, want *appium.Error", err)
	}
	if clientErr.Operation != "get_device_time" {
		t.Fatalf(
			"operation = %q, want get_device_time",
			clientErr.Operation,
		)
	}
}

func newDP121DeviceTimeSession(
	t *testing.T,
	automationName string,
	options appium.ClientOptions,
	deviceTimeHandler http.Handler,
) (*appium.Session, *contracttest.Recorder) {
	t.Helper()

	recorder := contracttest.NewRecorder(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.Method == http.MethodPost && request.RequestURI == "/session" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(
				`{"value":{"sessionId":"session/id","capabilities":{` +
					`"automationName":"` + automationName + `"}}}`,
			))
			return
		}

		deviceTimeHandler.ServeHTTP(writer, request)
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)

	client, err := server.NewClient(options)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(
		context.Background(),
		appium.MatchCapabilities(appium.Capabilities{
			"platformName":          "synthetic",
			"appium:automationName": automationName,
		}),
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	recorder.Reset()
	return session, recorder
}

type dp121RoundTripperFunc func(
	*http.Request,
) (*http.Response, error)

func (roundTrip dp121RoundTripperFunc) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	return roundTrip(request)
}
