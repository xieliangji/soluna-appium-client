package soluna_appium_client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestAlertProtocol(t *testing.T) {
	recorder := contracttest.NewRecorder(
		http.HandlerFunc(
			func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				switch request.RequestURI {
				case "/session":
					_, _ = writer.Write([]byte(
						`{"value":{"sessionId":"session/id","capabilities":{"automationName":"XCUITest"}}}`,
					))
				case "/session/session%2Fid/alert/text":
					if request.Method == http.MethodGet {
						_, _ = writer.Write([]byte(`{"value":"Confirm?"}`))
						return
					}
					_, _ = writer.Write([]byte(`{"value":null}`))
				case "/session/session%2Fid/alert/accept",
					"/session/session%2Fid/alert/dismiss":
					_, _ = writer.Write([]byte(`{"value":null}`))
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
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(
		appium.Capabilities{
			"platformName":          "iOS",
			"appium:automationName": "XCUITest",
		},
	))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	text, err := session.AlertText(context.Background())
	if err != nil {
		t.Fatalf("get alert text: %v", err)
	}
	if text != "Confirm?" {
		t.Fatalf("unexpected alert text: %q", text)
	}
	if err := session.AcceptAlert(context.Background()); err != nil {
		t.Fatalf("accept alert: %v", err)
	}
	if err := session.DismissAlert(context.Background()); err != nil {
		t.Fatalf("dismiss alert: %v", err)
	}
	if err := session.SetAlertText(context.Background(), "Proceed"); err != nil {
		t.Fatalf("set alert text: %v", err)
	}

	requests := recorder.Requests()
	if len(requests) != 5 {
		t.Fatalf("unexpected request count: expected 5, got %d", len(requests))
	}

	assertAlertRequest(t, requests[1], http.MethodGet, "/session/session%2Fid/alert/text", nil)
	assertAlertRequest(t, requests[2], http.MethodPost, "/session/session%2Fid/alert/accept", nil)
	assertAlertRequest(t, requests[3], http.MethodPost, "/session/session%2Fid/alert/dismiss", nil)
	assertAlertRequest(t, requests[4], http.MethodPost, "/session/session%2Fid/alert/text", map[string]any{"text": "Proceed"})
}

func TestAlertTextRejectsNullAndInvalidValues(t *testing.T) {
	for _, response := range []string{`{"value":null}`, `{"value":123}`} {
		t.Run(response, func(t *testing.T) {
			recorder := contracttest.NewRecorder(
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					writer.Header().Set("Content-Type", "application/json")
					if request.RequestURI == "/session" {
						_, _ = writer.Write([]byte(`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`))
						return
					}
					_, _ = writer.Write([]byte(response))
				}),
			)
			server := contracttest.NewServer(recorder)
			t.Cleanup(server.Close)
			client, err := server.NewClient(appium.ClientOptions{})
			if err != nil {
				t.Fatalf("create client: %v", err)
			}
			session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
				"platformName":          "iOS",
				"appium:automationName": "XCUITest",
			}))
			if err != nil {
				t.Fatalf("create session: %v", err)
			}

			_, err = session.AlertText(context.Background())
			if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
				t.Fatalf("expected response invalid error, got %v", err)
			}
			if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
				t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
			}
		})
	}
}

func TestAlertCommandsRequireNullSuccessValue(t *testing.T) {
	recorder := contracttest.NewRecorder(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			if request.RequestURI == "/session" {
				_, _ = writer.Write([]byte(`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`))
				return
			}
			_, _ = writer.Write([]byte(`{"value":"ok"}`))
		}),
	)
	server := contracttest.NewServer(recorder)
	defer server.Close()
	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"platformName":          "iOS",
		"appium:automationName": "XCUITest",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := session.AcceptAlert(context.Background()); err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
		t.Fatalf("expected response invalid error, got %v", err)
	}
}

func TestAlertNoSuchAlertMapsToDedicatedError(t *testing.T) {
	server := contracttest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.RequestURI {
		case "/session":
			_, _ = writer.Write([]byte(`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"value":{"error":"no such alert","message":"no alert present"}}`))
		}
	}))
	defer server.Close()
	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"platformName":          "iOS",
		"appium:automationName": "XCUITest",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err = session.AlertText(context.Background())
	if err == nil || !appium.IsErrorCode(err, appium.CodeAlertNotFound) {
		t.Fatalf("expected alert not found error, got %v", err)
	}
	if appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
		t.Fatalf("unexpected delivery: %q", appium.DeliveryOf(err))
	}
}

func TestAlertLocalFailureDoesNotSendRemoteRequest(t *testing.T) {
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`))
	}))
	server := contracttest.NewServer(recorder)
	defer server.Close()
	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"platformName":          "iOS",
		"appium:automationName": "XCUITest",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	recorder.Reset()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := session.AlertText(ctx); err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
	if err := session.AcceptAlert(ctx); err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
	if err := session.SetAlertText(ctx, "x"); err == nil || !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("canceled alert commands must not be delivered: got %d", len(requests))
	}
}

func assertAlertRequest(t *testing.T, request contracttest.RecordedRequest, method, uri string, body map[string]any) {
	t.Helper()
	if request.Method != method || request.RequestURI != uri {
		t.Fatalf("unexpected request: method=%q uri=%q", request.Method, request.RequestURI)
	}
	if body == nil {
		if len(request.Body) != 0 {
			t.Fatalf("expected empty request body, got %s", request.Body)
		}
		return
	}
	var decoded map[string]any
	if err := json.Unmarshal(request.Body, &decoded); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if decoded["text"] != body["text"] {
		t.Fatalf("unexpected request body: %#v", decoded)
	}
}
