package appium_test

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

const expectedWebViewportScript = "return {scrollX: window.scrollX, scrollY: window.scrollY, width: window.innerWidth, height: window.innerHeight}"

func TestWebContextFindElementsUsesScrolledCSSViewport(t *testing.T) {
	for _, contextName := range []string{"WEBVIEW", "WEBVIEW_应用", "CHROMIUM"} {
		t.Run(contextName, func(t *testing.T) {
			session, recorder := newContextTestSession(
				t,
				appium.ClientOptions{},
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					switch request.RequestURI {
					case "/session/session%2Fid/context":
						writeJSONValue(t, writer, contextName)

					case "/session/session%2Fid/elements":
						_, _ = writer.Write([]byte(
							`{"value":[` +
								`{"element-6066-11e4-a52e-4f735466cecf":"first"},` +
								`{"element-6066-11e4-a52e-4f735466cecf":"off"},` +
								`{"element-6066-11e4-a52e-4f735466cecf":"partial"},` +
								`{"element-6066-11e4-a52e-4f735466cecf":"zero"}` +
								`]}`,
						))

					case "/session/session%2Fid/execute/sync":
						_, _ = writer.Write([]byte(
							`{"value":{"scrollX":100.25,"scrollY":200.5,"width":100.5,"height":80.25,"ignored":true}}`,
						))

					case "/session/session%2Fid/element/first/rect":
						_, _ = writer.Write([]byte(
							`{"value":{"x":100.5,"y":200.75,"width":10.25,"height":10.5}}`,
						))

					case "/session/session%2Fid/element/off/rect":
						_, _ = writer.Write([]byte(
							`{"value":{"x":50,"y":200.5,"width":20,"height":10}}`,
						))

					case "/session/session%2Fid/element/partial/rect":
						_, _ = writer.Write([]byte(
							`{"value":{"x":195.75,"y":275.5,"width":20.5,"height":10.5}}`,
						))

					case "/session/session%2Fid/element/zero/rect":
						_, _ = writer.Write([]byte(
							`{"value":{"x":120,"y":220,"width":0,"height":10}}`,
						))

					default:
						http.NotFound(writer, request)
					}
				}),
			)

			elements, err := session.FindElements(
				context.Background(),
				appium.ID("item"),
			)
			if err != nil {
				t.Fatalf("find web elements: %v", err)
			}
			if len(elements) != 2 ||
				elements[0].ID() != "first" ||
				elements[1].ID() != "partial" {
				t.Fatalf("unexpected filtered elements: %#v", elementIDs(elements))
			}

			requests := recorder.Requests()
			expectedURIs := []string{
				"/session/session%2Fid/context",
				"/session/session%2Fid/elements",
				"/session/session%2Fid/execute/sync",
				"/session/session%2Fid/element/first/rect",
				"/session/session%2Fid/element/off/rect",
				"/session/session%2Fid/element/partial/rect",
				"/session/session%2Fid/element/zero/rect",
			}
			assertRequestURIs(t, requests, expectedURIs)
			if err := contracttest.MatchJSONBody(
				requests[2],
				map[string]any{
					"script": expectedWebViewportScript,
					"args":   []any{},
				},
			); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWebContextFindUsesNegativeScrollOffsetsWithoutClamping(t *testing.T) {
	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.RequestURI {
			case "/session/session%2Fid/context":
				_, _ = writer.Write([]byte(`{"value":"WEBVIEW_app"}`))
			case "/session/session%2Fid/elements":
				_, _ = writer.Write([]byte(
					`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"target"}]}`,
				))
			case "/session/session%2Fid/execute/sync":
				_, _ = writer.Write([]byte(
					`{"value":{"scrollX":-100,"scrollY":-50,"width":100,"height":100}}`,
				))
			case "/session/session%2Fid/element/target/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":-50,"y":-25,"width":10,"height":10}}`,
				))
			default:
				http.NotFound(writer, request)
			}
		}),
	)

	element, err := session.Find(context.Background(), appium.ID("target"))
	if err != nil {
		t.Fatalf("find with negative scroll offsets: %v", err)
	}
	if element.ID() != "target" {
		t.Fatalf("element ID = %q, want target", element.ID())
	}
	assertRequestURIs(t, recorder.Requests(), []string{
		"/session/session%2Fid/context",
		"/session/session%2Fid/elements",
		"/session/session%2Fid/execute/sync",
		"/session/session%2Fid/element/target/rect",
	})
}

func TestWebContextEmptyFindResultsDoNotProbeViewport(t *testing.T) {
	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.RequestURI {
			case "/session/session%2Fid/context":
				_, _ = writer.Write([]byte(`{"value":"WEBVIEW_app"}`))
			case "/session/session%2Fid/elements":
				_, _ = writer.Write([]byte(`{"value":[]}`))
			default:
				http.Error(writer, "unexpected viewport probe", http.StatusInternalServerError)
			}
		}),
	)

	elements, err := session.FindElements(context.Background(), appium.ID("missing"))
	if err != nil {
		t.Fatalf("find missing web elements: %v", err)
	}
	if elements == nil || len(elements) != 0 {
		t.Fatalf("empty web result = %#v, want non-nil empty slice", elements)
	}
	assertRequestURIs(t, recorder.Requests(), []string{
		"/session/session%2Fid/context",
		"/session/session%2Fid/elements",
	})

	recorder.Reset()
	element, err := session.Find(context.Background(), appium.ID("missing"))
	if element != nil {
		t.Fatalf("missing web find returned element: %v", element)
	}
	assertCommandError(t, err, appium.CodeElementNotFound, appium.DeliveryAcknowledged, "find_element")
	assertRequestURIs(t, recorder.Requests(), []string{
		"/session/session%2Fid/context",
		"/session/session%2Fid/elements",
	})
}

func TestWebContextElementFindUsesParentScopeAndStopsAtFirstIntersection(t *testing.T) {
	var neverRectCalls atomic.Int32
	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.RequestURI {
			case "/session/session%2Fid/context":
				_, _ = writer.Write([]byte(`{"value":"WEBVIEW_app"}`))
			case "/session/session%2Fid/elements":
				_, _ = writer.Write([]byte(
					`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"parent"}]}`,
				))
			case "/session/session%2Fid/element/parent/elements":
				_, _ = writer.Write([]byte(
					`{"value":[` +
						`{"element-6066-11e4-a52e-4f735466cecf":"off"},` +
						`{"element-6066-11e4-a52e-4f735466cecf":"visible"},` +
						`{"element-6066-11e4-a52e-4f735466cecf":"never"}` +
						`]}`,
				))
			case "/session/session%2Fid/execute/sync":
				_, _ = writer.Write([]byte(
					`{"value":{"scrollX":100,"scrollY":200,"width":100,"height":100}}`,
				))
			case "/session/session%2Fid/element/parent/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":110,"y":210,"width":40,"height":40}}`,
				))
			case "/session/session%2Fid/element/off/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":50,"y":210,"width":20,"height":20}}`,
				))
			case "/session/session%2Fid/element/visible/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":190,"y":220,"width":20,"height":20}}`,
				))
			case "/session/session%2Fid/element/never/rect":
				neverRectCalls.Add(1)
				_, _ = writer.Write([]byte(
					`{"value":{"x":110,"y":210,"width":20,"height":20}}`,
				))
			default:
				http.NotFound(writer, request)
			}
		}),
	)

	parent, err := session.Find(context.Background(), appium.ID("parent"))
	if err != nil {
		t.Fatalf("find parent: %v", err)
	}
	recorder.Reset()

	child, err := parent.Find(context.Background(), appium.ID("child"))
	if err != nil {
		t.Fatalf("find child: %v", err)
	}
	if child.ID() != "visible" {
		t.Fatalf("child ID = %q, want visible", child.ID())
	}
	if neverRectCalls.Load() != 0 {
		t.Fatalf("find queried rect after first intersection: %d", neverRectCalls.Load())
	}
	assertRequestURIs(t, recorder.Requests(), []string{
		"/session/session%2Fid/context",
		"/session/session%2Fid/element/parent/elements",
		"/session/session%2Fid/execute/sync",
		"/session/session%2Fid/element/off/rect",
		"/session/session%2Fid/element/visible/rect",
	})
}

func TestWebContextElementFindElementsPreservesOrderAndFiltersAllCandidates(t *testing.T) {
	recorder, parent := newWebContextElementScopeParent(
		t,
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.RequestURI {
			case "/session/session%2Fid/element/parent/elements":
				_, _ = writer.Write([]byte(
					`{"value":[` +
						`{"element-6066-11e4-a52e-4f735466cecf":"off"},` +
						`{"element-6066-11e4-a52e-4f735466cecf":"third"},` +
						`{"element-6066-11e4-a52e-4f735466cecf":"first"},` +
						`{"element-6066-11e4-a52e-4f735466cecf":"edge"}` +
						`]}`,
				))
			case "/session/session%2Fid/element/off/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":50,"y":210,"width":20,"height":20}}`,
				))
			case "/session/session%2Fid/element/third/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":190,"y":220,"width":20,"height":20}}`,
				))
			case "/session/session%2Fid/element/first/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":120,"y":230,"width":20,"height":20}}`,
				))
			case "/session/session%2Fid/element/edge/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":200,"y":220,"width":20,"height":20}}`,
				))
			default:
				http.NotFound(writer, request)
			}
		}),
	)

	elements, err := parent.FindElements(
		context.Background(),
		appium.ID("child"),
	)
	if err != nil {
		t.Fatalf("find web descendants: %v", err)
	}
	expectedIDs := []string{"third", "first"}
	if got := elementIDs(elements); !equalStrings(got, expectedIDs) {
		t.Fatalf("element IDs = %#v, want %#v", got, expectedIDs)
	}

	requests := recorder.Requests()
	assertRequestURIs(t, requests, []string{
		"/session/session%2Fid/context",
		"/session/session%2Fid/element/parent/elements",
		"/session/session%2Fid/execute/sync",
		"/session/session%2Fid/element/off/rect",
		"/session/session%2Fid/element/third/rect",
		"/session/session%2Fid/element/first/rect",
		"/session/session%2Fid/element/edge/rect",
	})
	if err := contracttest.MatchJSONBody(
		requests[1],
		map[string]any{"using": "id", "value": "child"},
	); err != nil {
		t.Fatal(err)
	}
}

func TestWebContextElementFindElementsDropsPartialResultOnLaterRectFailure(t *testing.T) {
	var neverRectCalls atomic.Int32
	recorder, parent := newWebContextElementScopeParent(
		t,
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.RequestURI {
			case "/session/session%2Fid/element/parent/elements":
				_, _ = writer.Write([]byte(
					`{"value":[` +
						`{"element-6066-11e4-a52e-4f735466cecf":"good"},` +
						`{"element-6066-11e4-a52e-4f735466cecf":"invalid"},` +
						`{"element-6066-11e4-a52e-4f735466cecf":"never"}` +
						`]}`,
				))
			case "/session/session%2Fid/element/good/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":120,"y":220,"width":20,"height":20}}`,
				))
			case "/session/session%2Fid/element/invalid/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":130,"y":230,"width":20}}`,
				))
			case "/session/session%2Fid/element/never/rect":
				neverRectCalls.Add(1)
				_, _ = writer.Write([]byte(
					`{"value":{"x":140,"y":240,"width":20,"height":20}}`,
				))
			default:
				http.NotFound(writer, request)
			}
		}),
	)

	elements, err := parent.FindElements(
		context.Background(),
		appium.ID("child"),
	)
	if elements != nil {
		t.Fatalf("failed lookup returned partial elements: %#v", elementIDs(elements))
	}
	assertCommandError(
		t,
		err,
		appium.CodeResponseInvalid,
		appium.DeliveryAcknowledged,
		"get_element_rect",
	)
	if neverRectCalls.Load() != 0 {
		t.Fatalf("lookup continued after invalid rect: %d", neverRectCalls.Load())
	}
	assertRequestURIs(t, recorder.Requests(), []string{
		"/session/session%2Fid/context",
		"/session/session%2Fid/element/parent/elements",
		"/session/session%2Fid/execute/sync",
		"/session/session%2Fid/element/good/rect",
		"/session/session%2Fid/element/invalid/rect",
	})
}

func TestWebContextElementTapUsesViewportRelativeCSSPoint(t *testing.T) {
	var rectCalls atomic.Int32
	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.RequestURI {
			case "/session/session%2Fid/context":
				_, _ = writer.Write([]byte(`{"value":"WEBVIEW"}`))
			case "/session/session%2Fid/elements":
				_, _ = writer.Write([]byte(
					`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"target"}]}`,
				))
			case "/session/session%2Fid/execute/sync":
				_, _ = writer.Write([]byte(
					`{"value":{"scrollX":100.25,"scrollY":200.5,"width":100,"height":80}}`,
				))
			case "/session/session%2Fid/element/target/rect":
				if rectCalls.Add(1) == 1 {
					_, _ = writer.Write([]byte(
						`{"value":{"x":110,"y":210,"width":20,"height":20}}`,
					))
					return
				}
				_, _ = writer.Write([]byte(
					`{"value":{"x":190.5,"y":270.25,"width":30,"height":20}}`,
				))
			case "/session/session%2Fid/actions":
				_, _ = writer.Write([]byte(`{"value":null}`))
			default:
				http.NotFound(writer, request)
			}
		}),
	)

	element, err := session.Find(context.Background(), appium.ID("target"))
	if err != nil {
		t.Fatalf("find target: %v", err)
	}
	recorder.Reset()

	if err := element.Tap(context.Background()); err != nil {
		t.Fatalf("tap web element: %v", err)
	}
	requests := recorder.Requests()
	assertRequestURIs(t, requests, []string{
		"/session/session%2Fid/context",
		"/session/session%2Fid/execute/sync",
		"/session/session%2Fid/element/target/rect",
		"/session/session%2Fid/actions",
	})
	if err := contracttest.MatchJSONBody(
		requests[1],
		map[string]any{
			"script": expectedWebViewportScript,
			"args":   []any{},
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := contracttest.MatchJSONBody(
		requests[3],
		expectedTapActionsBody(95, 75),
	); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownContextNamesDoNotFallback(t *testing.T) {
	for _, contextName := range []string{"", "WEBVIEW_", "webview", "NATIVE", "CUSTOM"} {
		t.Run(contextName, func(t *testing.T) {
			session, recorder := newContextTestSession(
				t,
				appium.ClientOptions{},
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					if request.RequestURI != "/session/session%2Fid/context" {
						http.Error(writer, "unexpected fallback request", http.StatusInternalServerError)
						return
					}
					writeJSONValue(t, writer, contextName)
				}),
			)

			element, err := session.Find(context.Background(), appium.ID("target"))
			if element != nil {
				t.Fatalf("unknown context returned element: %v", element)
			}
			assertUnsupportedGeometryError(t, err, "find_element")
			assertRequestURIs(t, recorder.Requests(), []string{
				"/session/session%2Fid/context",
			})
		})
	}
}

func TestUnknownContextRejectsAllContextSensitiveElementOperations(t *testing.T) {
	var current atomic.Value
	current.Store("NATIVE_APP")
	observer := &commandObserverRecorder{}

	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{Observer: observer},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.RequestURI {
			case "/session/session%2Fid/context":
				writeJSONValue(t, writer, current.Load().(string))
			case "/session/session%2Fid/elements":
				_, _ = writer.Write([]byte(
					`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"parent"}]}`,
				))
			case "/session/session%2Fid/window/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":0,"y":0,"width":100,"height":100}}`,
				))
			case "/session/session%2Fid/element/parent/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":10,"y":10,"width":20,"height":20}}`,
				))
			default:
				http.Error(writer, "unexpected fallback request", http.StatusInternalServerError)
			}
		}),
	)

	parent, err := session.Find(context.Background(), appium.ID("parent"))
	if err != nil {
		t.Fatalf("find parent in native context: %v", err)
	}
	current.Store("WEBVIEW_")

	tests := []struct {
		name      string
		operation string
		invoke    func() error
	}{
		{
			name:      "session find",
			operation: "find_element",
			invoke: func() error {
				_, err := session.Find(context.Background(), appium.ID("child"))
				return err
			},
		},
		{
			name:      "session find elements",
			operation: "find_elements",
			invoke: func() error {
				_, err := session.FindElements(context.Background(), appium.ID("child"))
				return err
			},
		},
		{
			name:      "element find",
			operation: "find_element_from_element",
			invoke: func() error {
				_, err := parent.Find(context.Background(), appium.ID("child"))
				return err
			},
		},
		{
			name:      "element find elements",
			operation: "find_elements_from_element",
			invoke: func() error {
				_, err := parent.FindElements(context.Background(), appium.ID("child"))
				return err
			},
		},
		{
			name:      "element tap",
			operation: "tap_element",
			invoke: func() error {
				return parent.Tap(context.Background())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder.Reset()
			observer.reset()
			err := test.invoke()
			assertUnsupportedGeometryError(t, err, test.operation)
			assertRequestURIs(t, recorder.Requests(), []string{
				"/session/session%2Fid/context",
			})

			started, finished := observer.snapshot()
			if len(started) != 1 || len(finished) != 1 {
				t.Fatalf(
					"observer events: started=%d finished=%d, want one successful context probe",
					len(started),
					len(finished),
				)
			}
			if started[0].Operation != "get_current_context" ||
				finished[0].Operation != "get_current_context" ||
				finished[0].ErrorCode != "" {
				t.Fatalf("unexpected context observer events: %#v %#v", started, finished)
			}
		})
	}
}

func TestWebViewportRejectsInvalidValuesBeforeRectProbe(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"null", `null`},
		{"missing field", `{"scrollX":0,"scrollY":0,"width":100}`},
		{"case alias", `{"ScrollX":0,"scrollY":0,"width":100,"height":100}`},
		{"null field", `{"scrollX":null,"scrollY":0,"width":100,"height":100}`},
		{"string field", `{"scrollX":0,"scrollY":0,"width":"100","height":100}`},
		{"zero width", `{"scrollX":0,"scrollY":0,"width":0,"height":100}`},
		{"negative height", `{"scrollX":0,"scrollY":0,"width":100,"height":-1}`},
		{"number overflow", `{"scrollX":1e309,"scrollY":0,"width":100,"height":100}`},
		{"endpoint overflow", `{"scrollX":1.7976931348623157e308,"scrollY":0,"width":1.7976931348623157e308,"height":100}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session, recorder := newContextTestSession(
				t,
				appium.ClientOptions{},
				http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					switch request.RequestURI {
					case "/session/session%2Fid/context":
						_, _ = writer.Write([]byte(`{"value":"WEBVIEW_app"}`))
					case "/session/session%2Fid/elements":
						_, _ = writer.Write([]byte(
							`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"target"}]}`,
						))
					case "/session/session%2Fid/execute/sync":
						_, _ = writer.Write([]byte(`{"value":` + test.value + `}`))
					default:
						http.Error(writer, "unexpected rect or fallback request", http.StatusInternalServerError)
					}
				}),
			)

			elements, err := session.FindElements(context.Background(), appium.ID("target"))
			if elements != nil {
				t.Fatalf("invalid viewport returned partial elements: %#v", elements)
			}
			assertCommandError(t, err, appium.CodeResponseInvalid, appium.DeliveryAcknowledged, "get_web_viewport")
			assertRequestURIs(t, recorder.Requests(), []string{
				"/session/session%2Fid/context",
				"/session/session%2Fid/elements",
				"/session/session%2Fid/execute/sync",
			})
		})
	}
}

func TestWebElementTapStopsOnInvalidViewportBeforeRectAndActions(t *testing.T) {
	var current atomic.Value
	current.Store("NATIVE_APP")
	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.RequestURI {
			case "/session/session%2Fid/context":
				writeJSONValue(t, writer, current.Load().(string))
			case "/session/session%2Fid/elements":
				_, _ = writer.Write([]byte(
					`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"target"}]}`,
				))
			case "/session/session%2Fid/window/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":0,"y":0,"width":100,"height":100}}`,
				))
			case "/session/session%2Fid/element/target/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":10,"y":10,"width":20,"height":20}}`,
				))
			case "/session/session%2Fid/execute/sync":
				_, _ = writer.Write([]byte(
					`{"value":{"scrollX":0,"scrollY":0,"width":0,"height":100}}`,
				))
			default:
				http.Error(writer, "unexpected rect or actions request", http.StatusInternalServerError)
			}
		}),
	)

	element, err := session.Find(context.Background(), appium.ID("target"))
	if err != nil {
		t.Fatalf("find target in native context: %v", err)
	}
	current.Store("WEBVIEW_app")
	recorder.Reset()

	err = element.Tap(context.Background())
	assertCommandError(t, err, appium.CodeResponseInvalid, appium.DeliveryAcknowledged, "get_web_viewport")
	assertRequestURIs(t, recorder.Requests(), []string{
		"/session/session%2Fid/context",
		"/session/session%2Fid/execute/sync",
	})
}

func TestWebFindElementsRejectsTranslatedRectOverflowWithoutPartialResult(t *testing.T) {
	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.RequestURI {
			case "/session/session%2Fid/context":
				_, _ = writer.Write([]byte(`{"value":"WEBVIEW_app"}`))
			case "/session/session%2Fid/elements":
				_, _ = writer.Write([]byte(
					`{"value":[` +
						`{"element-6066-11e4-a52e-4f735466cecf":"good"},` +
						`{"element-6066-11e4-a52e-4f735466cecf":"overflow"}` +
						`]}`,
				))
			case "/session/session%2Fid/execute/sync":
				_, _ = writer.Write([]byte(
					`{"value":{"scrollX":-1.7976931348623157e308,"scrollY":0,"width":100,"height":100}}`,
				))
			case "/session/session%2Fid/element/good/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":-1.7976931348623157e308,"y":10,"width":20,"height":20}}`,
				))
			case "/session/session%2Fid/element/overflow/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":1.7976931348623157e308,"y":10,"width":20,"height":20}}`,
				))
			default:
				http.NotFound(writer, request)
			}
		}),
	)

	elements, err := session.FindElements(context.Background(), appium.ID("target"))
	if elements != nil {
		t.Fatalf("overflow returned partial elements: %#v", elements)
	}
	assertCommandError(t, err, appium.CodeResponseInvalid, appium.DeliveryAcknowledged, "get_element_rect")
	assertRequestURIs(t, recorder.Requests(), []string{
		"/session/session%2Fid/context",
		"/session/session%2Fid/elements",
		"/session/session%2Fid/execute/sync",
		"/session/session%2Fid/element/good/rect",
		"/session/session%2Fid/element/overflow/rect",
	})
}

func TestWebFindPropagatesStaleRectWithoutRetry(t *testing.T) {
	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.RequestURI {
			case "/session/session%2Fid/context":
				_, _ = writer.Write([]byte(`{"value":"WEBVIEW_app"}`))
			case "/session/session%2Fid/elements":
				_, _ = writer.Write([]byte(
					`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"stale"}]}`,
				))
			case "/session/session%2Fid/execute/sync":
				_, _ = writer.Write([]byte(
					`{"value":{"scrollX":0,"scrollY":0,"width":100,"height":100}}`,
				))
			case "/session/session%2Fid/element/stale/rect":
				writer.WriteHeader(http.StatusNotFound)
				_, _ = writer.Write([]byte(
					`{"value":{"error":"stale element reference","message":"stale"}}`,
				))
			default:
				http.NotFound(writer, request)
			}
		}),
	)

	element, err := session.Find(context.Background(), appium.ID("target"))
	if element != nil {
		t.Fatalf("stale lookup returned element: %v", element)
	}
	assertCommandError(t, err, appium.CodeElementStale, appium.DeliveryAcknowledged, "get_element_rect")
	assertRequestURIs(t, recorder.Requests(), []string{
		"/session/session%2Fid/context",
		"/session/session%2Fid/elements",
		"/session/session%2Fid/execute/sync",
		"/session/session%2Fid/element/stale/rect",
	})
}

func TestElementRectDoesNotProbeContext(t *testing.T) {
	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.RequestURI {
			case "/session/session%2Fid/context":
				_, _ = writer.Write([]byte(`{"value":"NATIVE_APP"}`))
			case "/session/session%2Fid/elements":
				_, _ = writer.Write([]byte(
					`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"target"}]}`,
				))
			case "/session/session%2Fid/window/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":0,"y":0,"width":100,"height":100}}`,
				))
			case "/session/session%2Fid/element/target/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":10.5,"y":20.25,"width":30,"height":40}}`,
				))
			default:
				http.NotFound(writer, request)
			}
		}),
	)

	element, err := session.Find(context.Background(), appium.ID("target"))
	if err != nil {
		t.Fatalf("find target: %v", err)
	}
	recorder.Reset()

	rect, err := element.Rect(context.Background())
	if err != nil {
		t.Fatalf("get element rect: %v", err)
	}
	if rect != (appium.Rect{X: 10.5, Y: 20.25, Width: 30, Height: 40}) {
		t.Fatalf("rect = %+v", rect)
	}
	assertRequestURIs(t, recorder.Requests(), []string{
		"/session/session%2Fid/element/target/rect",
	})
}

func newWebContextElementScopeParent(
	t *testing.T,
	next http.Handler,
) (*contracttest.Recorder, *appium.Element) {
	t.Helper()

	session, recorder := newContextTestSession(
		t,
		appium.ClientOptions{},
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.RequestURI {
			case "/session/session%2Fid/context":
				_, _ = writer.Write([]byte(`{"value":"WEBVIEW_app"}`))
			case "/session/session%2Fid/elements":
				_, _ = writer.Write([]byte(
					`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"parent"}]}`,
				))
			case "/session/session%2Fid/execute/sync":
				_, _ = writer.Write([]byte(
					`{"value":{"scrollX":100,"scrollY":200,"width":100,"height":100}}`,
				))
			case "/session/session%2Fid/element/parent/rect":
				_, _ = writer.Write([]byte(
					`{"value":{"x":110,"y":210,"width":40,"height":40}}`,
				))
			default:
				if next == nil {
					http.NotFound(writer, request)
					return
				}
				next.ServeHTTP(writer, request)
			}
		}),
	)

	parent, err := session.Find(context.Background(), appium.ID("parent"))
	if err != nil {
		t.Fatalf("find web parent: %v", err)
	}
	recorder.Reset()
	return recorder, parent
}

func assertUnsupportedGeometryError(t *testing.T, err error, operation string) {
	t.Helper()
	assertCommandError(t, err, appium.CodeUnsupported, appium.DeliveryNotSent, operation)
}

func assertCommandError(
	t *testing.T,
	err error,
	code appium.ErrorCode,
	delivery appium.DeliveryState,
	operation string,
) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error", code)
	}
	if !appium.IsErrorCode(err, code) {
		t.Fatalf("error = %v, want code %q", err, code)
	}
	if got := appium.DeliveryOf(err); got != delivery {
		t.Fatalf("delivery = %q, want %q", got, delivery)
	}
	var clientErr *appium.Error
	if !errors.As(err, &clientErr) {
		t.Fatalf("error type = %T, want *appium.Error", err)
	}
	if clientErr.Operation != operation {
		t.Fatalf("operation = %q, want %q", clientErr.Operation, operation)
	}
}

func assertRequestURIs(
	t *testing.T,
	requests []contracttest.RecordedRequest,
	expected []string,
) {
	t.Helper()
	if len(requests) != len(expected) {
		t.Fatalf("request count = %d, want %d: %#v", len(requests), len(expected), requests)
	}
	for index, uri := range expected {
		if requests[index].RequestURI != uri {
			t.Fatalf(
				"request %d URI = %q, want %q",
				index,
				requests[index].RequestURI,
				uri,
			)
		}
	}
}

func elementIDs(elements []*appium.Element) []string {
	ids := make([]string, len(elements))
	for index, element := range elements {
		ids[index] = element.ID()
	}
	return ids
}
