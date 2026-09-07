package appium_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
	"github.com/xieliangji/soluna-appium-client/xcuitest"
)

func TestElementBelongsToUsesLocalSessionIdentity(t *testing.T) {
	recorder := contracttest.NewRecorder(ownershipHandler())
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)

	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	element, err := session.Find(context.Background(), appium.XPath("//button"))
	if err != nil {
		t.Fatalf("find element: %v", err)
	}

	sessionCopy := *session
	elementCopy := *element

	otherSession, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{}))
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}

	otherClient, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create second client: %v", err)
	}
	otherClientSession, err := otherClient.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{}))
	if err != nil {
		t.Fatalf("create second-client session: %v", err)
	}
	secondRecorder := contracttest.NewRecorder(ownershipHandler())
	secondServer := contracttest.NewServer(secondRecorder)
	t.Cleanup(secondServer.Close)
	secondClient, err := secondServer.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create second-endpoint client: %v", err)
	}
	secondEndpointSession, err := secondClient.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{}))
	if err != nil {
		t.Fatalf("create second-endpoint session: %v", err)
	}

	// 所有远端 Session 都故意返回相同 ID，确保查询使用本地身份。
	recorder.Reset()
	secondRecorder.Reset()
	tests := []struct {
		name    string
		element *appium.Element
		session *appium.Session
		want    bool
	}{
		{name: "creating session", element: element, session: session, want: true},
		{name: "session value copy", element: element, session: &sessionCopy, want: true},
		{name: "element value copy", element: &elementCopy, session: session, want: true},
		{name: "independent session with same client and ID", element: element, session: otherSession},
		{name: "different client with same endpoint and ID", element: element, session: otherClientSession},
		{name: "different endpoint with same ID", element: element, session: secondEndpointSession},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.element.BelongsTo(test.session); got != test.want {
				t.Errorf("BelongsTo = %v, want %v", got, test.want)
			}
		})
	}
	if count := len(recorder.Requests()) + len(secondRecorder.Requests()); count != 0 {
		t.Fatalf("local ownership queries sent %d requests", count)
	}
}

func TestElementBelongsToHandlesInvalidAndClosedHandlesWithoutRequests(t *testing.T) {
	recorder := contracttest.NewRecorder(ownershipHandler())
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	element, err := session.Find(context.Background(), appium.XPath("//button"))
	if err != nil {
		t.Fatalf("find element: %v", err)
	}
	recorder.Reset()

	var nilElement *appium.Element
	var nilSession *appium.Session
	var zeroElement appium.Element
	var zeroSession appium.Session
	for name, got := range map[string]bool{
		"nil element":  nilElement.BelongsTo(session),
		"nil session":  element.BelongsTo(nilSession),
		"zero element": zeroElement.BelongsTo(session),
		"zero session": element.BelongsTo(&zeroSession),
		"nil both":     nilElement.BelongsTo(nilSession),
	} {
		if got {
			t.Errorf("%s should not report ownership", name)
		}
	}
	if count := len(recorder.Requests()); count != 0 {
		t.Fatalf("invalid ownership queries sent %d requests", count)
	}

	sessionCopy := *session
	if err := session.Close(context.Background()); err != nil {
		t.Fatalf("close session: %v", err)
	}
	recorder.Reset()
	if !element.BelongsTo(session) || !element.BelongsTo(&sessionCopy) {
		t.Fatal("closing a session must not change local ownership")
	}
	if _, err := element.Text(context.Background()); !appium.IsErrorCode(err, appium.CodeSessionLost) ||
		appium.DeliveryOf(err) != appium.DeliveryNotSent {
		t.Fatalf("closed session command should fail locally: %v", err)
	}
	if count := len(recorder.Requests()); count != 0 {
		t.Fatalf("closed ownership query or command sent %d requests", count)
	}
}

func TestElementsKeepCreatingSessionWhenHandleIsReassigned(t *testing.T) {
	for _, scope := range []string{"session Find", "session FindElements", "element Find", "element FindElements"} {
		t.Run(scope, func(t *testing.T) {
			created := 0
			recorder := contracttest.NewRecorder(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodPost && r.RequestURI == "/session":
					created++
					_, _ = fmt.Fprintf(w, `{"value":{"sessionId":"session-%d","capabilities":{"automationName":"XCUITest"}}}`, created)
				case r.Method == http.MethodGet && r.RequestURI == "/session/session-1/context":
					_, _ = w.Write([]byte(`{"value":"NATIVE_APP"}`))
				case r.Method == http.MethodPost && r.RequestURI == "/session/session-1/elements":
					_, _ = w.Write([]byte(`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"wheel"}]}`))
				case r.Method == http.MethodPost && r.RequestURI == "/session/session-1/element/wheel/elements":
					_, _ = w.Write([]byte(`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"child"}]}`))
				case r.Method == http.MethodGet && (r.RequestURI == "/session/session-1/window/rect" ||
					r.RequestURI == "/session/session-1/element/wheel/rect" || r.RequestURI == "/session/session-1/element/child/rect"):
					_, _ = w.Write([]byte(`{"value":{"x":0,"y":0,"width":100,"height":100}}`))
				case r.Method == http.MethodGet && (r.RequestURI == "/session/session-1/element/wheel/text" || r.RequestURI == "/session/session-1/element/child/text"):
					_, _ = w.Write([]byte(`{"value":"original session"}`))
				case r.Method == http.MethodPost && (r.RequestURI == "/session/session-1/execute/sync" || r.RequestURI == "/session/session-2/execute/sync"):
					// 接受选择请求，使错误归属不会被远端拒绝掩盖。
					_, _ = w.Write([]byte(`{"value":null}`))
				case r.Method == http.MethodDelete && (r.RequestURI == "/session/session-1" || r.RequestURI == "/session/session-2"):
					_, _ = w.Write([]byte(`{"value":null}`))
				default:
					http.NotFound(w, r)
				}
			}))
			server := contracttest.NewServer(recorder)
			t.Cleanup(server.Close)
			client, err := server.NewClient(appium.ClientOptions{})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			first, err := client.CreateSession(ctx, appium.MatchCapabilities(appium.Capabilities{}))
			if err != nil {
				t.Fatal(err)
			}
			second, err := client.CreateSession(ctx, appium.MatchCapabilities(appium.Capabilities{}))
			if err != nil {
				t.Fatal(err)
			}
			handle := *first
			locator := appium.IOSClassChain("**/XCUIElementTypePickerWheel")
			var elements []*appium.Element
			switch scope {
			case "session Find":
				element, findErr := handle.Find(ctx, locator)
				elements, err = []*appium.Element{element}, findErr
			case "session FindElements":
				elements, err = handle.FindElements(ctx, locator)
			default:
				parent, findErr := handle.Find(ctx, locator)
				if findErr != nil {
					t.Fatal(findErr)
				}
				if scope == "element Find" {
					element, findErr := parent.Find(ctx, locator)
					elements, err = []*appium.Element{element}, findErr
				} else {
					elements, err = parent.FindElements(ctx, locator)
				}
			}
			if err != nil || len(elements) != 1 {
				t.Fatalf("find elements: %v; got %d elements", err, len(elements))
			}
			element := elements[0]
			handle = *second
			recorder.Reset()
			if !element.BelongsTo(first) || element.BelongsTo(second) || element.BelongsTo(&handle) {
				t.Error("reassigning the caller's Session variable changed existing Element ownership")
			}
			err = xcuitest.IOSSelectPickerWheelValue(ctx, &handle, element, xcuitest.PickerWheelNext, 0.2)
			if !appium.IsErrorCode(err, appium.CodeInvalidArgument) || appium.DeliveryOf(err) != appium.DeliveryNotSent {
				t.Errorf("cross-session selection was not rejected locally: %v", err)
			}
			if requests := recorder.Requests(); len(requests) != 0 {
				t.Errorf("cross-session selection sent %d requests", len(requests))
			}

			// 关闭被重新赋值的句柄只关闭第二个 Session，不影响原来的 Element。
			if err := handle.Close(ctx); err != nil {
				t.Fatal(err)
			}
			recorder.Reset()
			value, err := element.Text(ctx)
			if err != nil || value != "original session" {
				t.Fatalf("element command lost its original session: %q / %v", value, err)
			}
			if err := xcuitest.IOSSelectPickerWheelValue(ctx, first, element, xcuitest.PickerWheelNext, 0.2); err != nil {
				t.Fatalf("selection with original session failed: %v", err)
			}
			requests := recorder.Requests()
			if len(requests) != 2 || requests[0].RequestURI != "/session/session-1/element/"+element.ID()+"/text" ||
				requests[1].RequestURI != "/session/session-1/execute/sync" {
				t.Fatalf("commands used wrong session routes: %+v", requests)
			}

			// 固定创建身份仍须共享原 Session 的关闭状态。
			originalCopy := *first
			if err := originalCopy.Close(ctx); err != nil {
				t.Fatal(err)
			}
			recorder.Reset()
			if !element.BelongsTo(first) {
				t.Fatal("closing original session changed ownership")
			}
			_, err = element.Text(ctx)
			if !appium.IsErrorCode(err, appium.CodeSessionLost) || appium.DeliveryOf(err) != appium.DeliveryNotSent {
				t.Fatalf("element did not share original session closure: %v", err)
			}
			if len(recorder.Requests()) != 0 {
				t.Fatal("closed original session sent an element command")
			}
		})
	}
}

func ownershipHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.RequestURI == "/session":
			_, _ = w.Write([]byte(`{"value":{"sessionId":"shared/id","capabilities":{"automationName":"XCUITest"}}}`))
		case r.Method == http.MethodDelete && r.RequestURI == "/session/shared%2Fid":
			_, _ = w.Write([]byte(`{"value":null}`))
		case r.Method == http.MethodGet && r.RequestURI == "/session/shared%2Fid/context":
			_, _ = w.Write([]byte(`{"value":"NATIVE_APP"}`))
		case r.Method == http.MethodPost && r.RequestURI == "/session/shared%2Fid/elements":
			_, _ = w.Write([]byte(`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"element/id"}]}`))
		case r.Method == http.MethodGet && r.RequestURI == "/session/shared%2Fid/window/rect":
			_, _ = w.Write([]byte(`{"value":{"x":0,"y":0,"width":100,"height":100}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/session/shared%2Fid/element/element%2Fid/rect":
			_, _ = w.Write([]byte(`{"value":{"x":10,"y":10,"width":20,"height":20}}`))
		default:
			http.NotFound(w, r)
		}
	})
}
