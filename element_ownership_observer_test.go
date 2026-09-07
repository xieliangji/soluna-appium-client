package appium_test

import (
	"context"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestElementFindSnapshotsSessionBeforeObserverReassignment(t *testing.T) {
	for _, operation := range []struct {
		name      string
		operation string
		find      func(*appium.Session) ([]*appium.Element, error)
	}{
		{name: "Find", operation: "find_element", find: func(session *appium.Session) ([]*appium.Element, error) {
			element, err := session.Find(context.Background(), appium.XPath("//button"))
			if element == nil {
				return nil, err
			}
			return []*appium.Element{element}, err
		}},
		{name: "FindElements", operation: "find_elements", find: func(session *appium.Session) ([]*appium.Element, error) {
			return session.FindElements(context.Background(), appium.XPath("//button"))
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			var handle appium.Session
			observer := &reassignSessionObserver{
				target: operation.operation,
				handle: &handle,
			}
			recorder := contracttest.NewRecorder(observerOwnershipHandler())
			server := contracttest.NewServer(recorder)
			t.Cleanup(server.Close)
			client, err := server.NewClient(appium.ClientOptions{Observer: observer})
			if err != nil {
				t.Fatal(err)
			}
			first, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{}))
			if err != nil {
				t.Fatal(err)
			}
			second, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{}))
			if err != nil {
				t.Fatal(err)
			}
			handle = *first
			observer.replacement = second
			recorder.Reset()

			elements, err := operation.find(&handle)
			if err != nil || len(elements) != 1 {
				t.Fatalf("find returned %d elements with error %v", len(elements), err)
			}
			element := elements[0]
			if !observer.reassigned {
				t.Fatal("observer did not reassign the Session handle")
			}
			if !element.BelongsTo(first) || element.BelongsTo(second) || element.BelongsTo(&handle) {
				t.Fatal("element ownership changed after observer reassignment")
			}
			wantRoutes := []string{
				"/session/session-1/context",
				"/session/session-1/elements",
				"/session/session-1/window/rect",
				"/session/session-1/element/wheel%2Fid/rect",
			}
			findRequests := recorder.Requests()
			if len(findRequests) != len(wantRoutes) {
				t.Fatalf("Find sent %d requests, want %d", len(findRequests), len(wantRoutes))
			}
			for index, want := range wantRoutes {
				if findRequests[index].RequestURI != want {
					t.Errorf("Find request %d used %q, want %q", index, findRequests[index].RequestURI, want)
				}
			}

			recorder.Reset()
			text, err := element.Text(context.Background())
			if err != nil || text != "first" {
				t.Fatalf("element command used the wrong Session: %q / %v", text, err)
			}
			requests := recorder.Requests()
			if len(requests) != 1 || requests[0].RequestURI != "/session/session-1/element/wheel%2Fid/text" {
				t.Fatalf("element command used unexpected route: %+v", requests)
			}
		})
	}
}

type reassignSessionObserver struct {
	target      string
	handle      *appium.Session
	replacement *appium.Session
	reassigned  bool
}

func (o *reassignSessionObserver) OnCommandStarted(appium.CommandStartedEvent) {}

func (o *reassignSessionObserver) OnCommandFinished(event appium.CommandFinishedEvent) {
	if !o.reassigned && event.Operation == o.target {
		*o.handle = *o.replacement
		o.reassigned = true
	}
}

func observerOwnershipHandler() http.Handler {
	creates := 0
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.RequestURI == "/session":
			if creates == 0 {
				creates++
				_, _ = w.Write([]byte(`{"value":{"sessionId":"session-1","capabilities":{"automationName":"XCUITest"}}}`))
				return
			}
			creates++
			_, _ = w.Write([]byte(`{"value":{"sessionId":"session-2","capabilities":{"automationName":"XCUITest"}}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/session/session-1/context":
			_, _ = w.Write([]byte(`{"value":"NATIVE_APP"}`))
		case r.Method == http.MethodPost && r.RequestURI == "/session/session-1/elements":
			_, _ = w.Write([]byte(`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"wheel/id"}]}`))
		case r.Method == http.MethodGet && r.RequestURI == "/session/session-1/window/rect":
			_, _ = w.Write([]byte(`{"value":{"x":0,"y":0,"width":100,"height":100}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/session/session-1/element/wheel%2Fid/rect":
			_, _ = w.Write([]byte(`{"value":{"x":10,"y":10,"width":20,"height":20}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/session/session-1/element/wheel%2Fid/text":
			_, _ = w.Write([]byte(`{"value":"first"}`))
		default:
			http.NotFound(w, r)
		}
	})
}
