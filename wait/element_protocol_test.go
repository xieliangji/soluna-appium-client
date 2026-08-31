package wait_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
	"github.com/xieliangji/soluna-appium-client/wait"
)

func TestElementUsesPublicSessionFindForEachRetry(t *testing.T) {
	var findCalls atomic.Int32
	recorder := contracttest.NewRecorder(
		http.HandlerFunc(
			func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")

				switch {
				case request.Method == http.MethodPost &&
					request.RequestURI == "/session":
					_, _ = writer.Write([]byte(
						`{"value":{"sessionId":"session/id","capabilities":{"automationName":"XCUITest"}}}`,
					))

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session%2Fid/elements":
					if findCalls.Add(1) == 1 {
						_, _ = writer.Write([]byte(`{"value":[]}`))
						return
					}
					_, _ = writer.Write([]byte(
						`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"element/id"}]}`,
					))

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/window/rect":
					_, _ = writer.Write([]byte(
						`{"value":{"x":0,"y":0,"width":390,"height":844}}`,
					))

				case request.Method == http.MethodGet &&
					request.RequestURI == "/session/session%2Fid/element/element%2Fid/rect":
					_, _ = writer.Write([]byte(
						`{"value":{"x":10,"y":20,"width":100,"height":40}}`,
					))

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
	recorder.Reset()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	element, err := wait.Element(
		ctx,
		2*time.Millisecond,
		session,
		appium.ID("login"),
	)
	if err != nil {
		t.Fatalf("wait for element: %v", err)
	}
	if element == nil || element.ID() != "element/id" {
		t.Fatalf("unexpected element: %#v", element)
	}

	requests := recorder.Requests()
	if len(requests) != 4 {
		t.Fatalf("request count = %d, want 4", len(requests))
	}
	expected := []string{
		"/session/session%2Fid/elements",
		"/session/session%2Fid/elements",
		"/session/session%2Fid/window/rect",
		"/session/session%2Fid/element/element%2Fid/rect",
	}
	for index, uri := range expected {
		if got := requests[index].RequestURI; got != uri {
			t.Fatalf("request %d URI = %q, want %q", index, got, uri)
		}
	}
}

func TestElementDoesNotRetryStaleFindFailure(t *testing.T) {
	var findCalls atomic.Int32
	server := contracttest.NewServer(
		http.HandlerFunc(
			func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")

				switch {
				case request.Method == http.MethodPost &&
					request.RequestURI == "/session":
					_, _ = writer.Write([]byte(
						`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`,
					))

				case request.Method == http.MethodPost &&
					request.RequestURI == "/session/session/elements":
					findCalls.Add(1)
					writer.WriteHeader(http.StatusNotFound)
					_, _ = writer.Write([]byte(
						`{"value":{"error":"stale element reference","message":"stale"}}`,
					))

				default:
					http.NotFound(writer, request)
				}
			},
		),
	)
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

	element, err := wait.Element(
		context.Background(),
		time.Hour,
		session,
		appium.ID("login"),
	)
	if element != nil {
		t.Fatalf("unexpected element: %v", element)
	}
	if !appium.IsErrorCode(err, appium.CodeElementStale) {
		t.Fatalf("error = %v, want stale error", err)
	}
	if got := findCalls.Load(); got != 1 {
		t.Fatalf("find requests = %d, want 1", got)
	}
}
