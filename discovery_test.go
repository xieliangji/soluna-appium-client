package soluna_appium_client_test

import (
	"context"
	"net/http"
	"reflect"
	"sort"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestDP041CommandsProtocolAndCatalogModel(t *testing.T) {
	const response = `{"value":{
"rest":{
  "base":{
    "/session/:sessionId/base":{
      "GET":{"command":"getBase","deprecated":false,"info":"base info","params":[{"name":"required","required":true,"schema":{"type":"string"}},{"name":"optional","required":false}],"unknown":{"nested":[{"value":1}]}}
    }
  },
  "driver":{
    "/session/:sessionId/driver":{"POST":{}}
  },
  "plugins":{
    "demo-plugin":{
      "/session/:sessionId/plugin":{"PATCH":{"command":"pluginCommand"}}
    }
  },
  "sectionExtra":{"enabled":true}
},
"bidi":{
  "base":{"session":{"new":{"command":"session.new","params":[]}}},
  "driver":{"custom":{"command":{"deprecated":true}}},
  "plugins":{"demo-plugin":{"network":{"subscribe":{"info":"plugin bidi"}}}}
},
"topExtra":{"items":[1,{"ok":true}]}
}}`

	recorder := contracttest.NewRecorder(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.RequestURI == "/session":
			_, _ = writer.Write([]byte(`{"value":{"sessionId":"session/id","capabilities":{"automationName":"XCUITest"}}}`))
		case request.Method == http.MethodGet && request.RequestURI == "/session/session%2Fid/appium/commands":
			_, _ = writer.Write([]byte(response))
		default:
			http.NotFound(writer, request)
		}
	}))
	server := contracttest.NewServer(recorder)
	defer server.Close()

	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"platformName": "iOS", "appium:automationName": "XCUITest",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	catalog, err := session.Commands(context.Background())
	if err != nil {
		t.Fatalf("read commands: %v", err)
	}
	if catalog.Rest == nil || catalog.BiDi == nil {
		t.Fatalf("expected both command sections: %#v", catalog)
	}
	if !reflect.DeepEqual(catalog.Rest.Extra, map[string]any{"sectionExtra": map[string]any{"enabled": true}}) {
		t.Fatalf("unexpected rest extra: %#v", catalog.Rest.Extra)
	}
	if !reflect.DeepEqual(catalog.Extra, map[string]any{"topExtra": map[string]any{"items": []any{float64(1), map[string]any{"ok": true}}}}) {
		t.Fatalf("unexpected catalog extra: %#v", catalog.Extra)
	}

	base := findDP041HTTPCommand(t, catalog.Rest.Base.Entries, "GET", "/session/:sessionId/base")
	if base.Source != (appium.CatalogSource{Kind: appium.CatalogSourceBase}) {
		t.Fatalf("unexpected base source: %#v", base.Source)
	}
	if base.Command == nil || *base.Command != "getBase" || base.Deprecated == nil || *base.Deprecated {
		t.Fatalf("unexpected base metadata: %#v", base.CatalogMetadata)
	}
	if base.Info == nil || *base.Info != "base info" || len(base.Params) != 2 {
		t.Fatalf("unexpected base metadata details: %#v", base.CatalogMetadata)
	}
	if base.Params[0].Name != "required" || !base.Params[0].Required ||
		!reflect.DeepEqual(base.Params[0].Extra, map[string]any{"schema": map[string]any{"type": "string"}}) {
		t.Fatalf("unexpected required parameter: %#v", base.Params[0])
	}
	if base.Params[1].Name != "optional" || base.Params[1].Required || base.Params[1].Extra != nil {
		t.Fatalf("unexpected optional parameter: %#v", base.Params[1])
	}
	if !reflect.DeepEqual(base.Extra, map[string]any{"unknown": map[string]any{"nested": []any{map[string]any{"value": float64(1)}}}}) {
		t.Fatalf("unexpected command extra: %#v", base.Extra)
	}

	plugin := findDP041HTTPCommand(t, catalog.Rest.Plugins["demo-plugin"].Entries, "PATCH", "/session/:sessionId/plugin")
	if plugin.Source != (appium.CatalogSource{Kind: appium.CatalogSourcePlugin, PluginName: "demo-plugin"}) {
		t.Fatalf("unexpected plugin source: %#v", plugin.Source)
	}
	bidi := findDP041BiDiCommand(t, catalog.BiDi.Plugins["demo-plugin"].Entries, "network", "subscribe")
	if bidi.Source != (appium.CatalogSource{Kind: appium.CatalogSourcePlugin, PluginName: "demo-plugin"}) {
		t.Fatalf("unexpected BiDi plugin source: %#v", bidi.Source)
	}
	baseBiDi := findDP041BiDiCommand(t, catalog.BiDi.Base.Entries, "session", "new")
	if baseBiDi.Params == nil || len(baseBiDi.Params) != 0 {
		t.Fatalf("expected explicit empty params slice: %#v", baseBiDi.Params)
	}

	if !catalog.SupportsHTTP("GET", "/session/:sessionId/base") ||
		!catalog.SupportsHTTP("PATCH", "/session/:sessionId/plugin") ||
		!catalog.SupportsBiDi("network", "subscribe") {
		t.Fatal("expected catalog Supports helpers to find entries")
	}
	if catalog.SupportsHTTP("get", "/session/:sessionId/base") ||
		catalog.SupportsHTTP("GET", "/session/:sessionId") ||
		catalog.SupportsHTTP("GET", " /session/:sessionId/base") ||
		catalog.SupportsBiDi("Network", "subscribe") ||
		catalog.SupportsBiDi("network", "subscribe.extra") ||
		catalog.SupportsHTTP("", "/session/:sessionId/base") {
		t.Fatal("Supports helpers must use exact non-empty identities")
	}

	base.Extra["unknown"].(map[string]any)["nested"].([]any)[0].(map[string]any)["value"] = float64(99)
	catalog.Extra["topExtra"].(map[string]any)["items"].([]any)[1].(map[string]any)["ok"] = false
	second, err := session.Commands(context.Background())
	if err != nil {
		t.Fatalf("read commands again: %v", err)
	}
	secondBase := findDP041HTTPCommand(t, second.Rest.Base.Entries, "GET", "/session/:sessionId/base")
	if !reflect.DeepEqual(secondBase.Extra, map[string]any{"unknown": map[string]any{"nested": []any{map[string]any{"value": float64(1)}}}}) {
		t.Fatalf("second command snapshot shares nested extra data: %#v", secondBase.Extra)
	}
	if !reflect.DeepEqual(second.Extra, map[string]any{"topExtra": map[string]any{"items": []any{float64(1), map[string]any{"ok": true}}}}) {
		t.Fatalf("second catalog snapshot shares nested extra data: %#v", second.Extra)
	}

	requests := recorder.Requests()
	if len(requests) != 3 {
		t.Fatalf("unexpected request count: %d", len(requests))
	}
	for _, request := range requests[1:] {
		if err := contracttest.MatchMethod(request, http.MethodGet); err != nil {
			t.Fatal(err)
		}
		if err := contracttest.MatchRequestURI(request, "/session/session%2Fid/appium/commands"); err != nil {
			t.Fatal(err)
		}
		if err := contracttest.MatchBody(request, nil); err != nil {
			t.Fatal(err)
		}
		if values := request.Header.Values("Content-Type"); len(values) != 0 {
			t.Fatalf("unexpected Content-Type on discovery GET: %v", values)
		}
	}
}

func TestDP041ExtensionsProtocolAndNoCache(t *testing.T) {
	responses := []string{
		`{"value":{"rest":{"driver":{"mobile:one":{"command":"one"}},"plugins":{}}}}`,
		`{"value":{"rest":{"driver":{"mobile:two":{}}}}}`,
	}
	getCount := 0
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.RequestURI == "/session":
			_, _ = writer.Write([]byte(`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`))
		case request.Method == http.MethodGet && request.RequestURI == "/session/session/appium/extensions":
			body := responses[getCount]
			getCount++
			_, _ = writer.Write([]byte(body))
		default:
			http.NotFound(writer, request)
		}
	}))
	server := contracttest.NewServer(recorder)
	defer server.Close()
	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"platformName": "iOS", "appium:automationName": "XCUITest",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	first, err := session.Extensions(context.Background())
	if err != nil {
		t.Fatalf("read first extensions: %v", err)
	}
	if first.Rest == nil || first.Rest.Plugins == nil || len(first.Rest.Plugins) != 0 {
		t.Fatalf("expected explicit empty plugin map: %#v", first)
	}
	if !first.SupportsExecuteMethod("mobile:one") || first.SupportsExecuteMethod("mobile:two") {
		t.Fatalf("unexpected first extension catalog: %#v", first)
	}

	second, err := session.Extensions(context.Background())
	if err != nil {
		t.Fatalf("read second extensions: %v", err)
	}
	if second.Rest == nil || second.Rest.Plugins != nil {
		t.Fatalf("expected missing plugin map to remain nil: %#v", second)
	}
	if second.SupportsExecuteMethod("mobile:one") || !second.SupportsExecuteMethod("mobile:two") {
		t.Fatalf("unexpected second extension catalog: %#v", second)
	}

	requests := recorder.Requests()
	if len(requests) != 3 {
		t.Fatalf("unexpected request count: %d", len(requests))
	}
	for _, request := range requests[1:] {
		if err := contracttest.MatchMethod(request, http.MethodGet); err != nil {
			t.Fatal(err)
		}
		if err := contracttest.MatchRequestURI(request, "/session/session/appium/extensions"); err != nil {
			t.Fatal(err)
		}
		if err := contracttest.MatchBody(request, nil); err != nil {
			t.Fatal(err)
		}
		if values := request.Header.Values("Content-Type"); len(values) != 0 {
			t.Fatalf("unexpected Content-Type on discovery GET: %v", values)
		}
	}
}

func TestDP041OptionalSectionsAndRequiredChildren(t *testing.T) {
	tests := []struct {
		name       string
		commands   string
		extensions string
		validCmd   bool
		validExt   bool
		restNil    bool
		bidiNil    bool
		extRestNil bool
	}{
		{name: "empty catalogs", commands: `{}`, extensions: `{}`, validCmd: true, validExt: true, restNil: true, bidiNil: true, extRestNil: true},
		{name: "empty command sections", commands: `{"rest":{"base":{},"driver":{}},"bidi":{"base":{},"driver":{}}}`, extensions: `{"rest":{"driver":{}}}`, validCmd: true, validExt: true},
		{name: "missing rest child", commands: `{"rest":{}}`, extensions: `{"rest":{}}`, validCmd: false, validExt: false},
		{name: "missing bidi child", commands: `{"bidi":{}}`, extensions: `{}`, validCmd: false, validExt: true, extRestNil: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session, recorder := newDP041DiscoveryTestSession(t, `{"value":`+test.commands+`}`, `{"value":`+test.extensions+`}`)
			commands, err := session.Commands(context.Background())
			if test.validCmd {
				if err != nil {
					t.Fatalf("read commands: %v", err)
				}
				if (commands.Rest == nil) != test.restNil || (commands.BiDi == nil) != test.bidiNil {
					t.Fatalf("unexpected command section presence: %#v", commands)
				}
			} else if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) || appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
				t.Fatalf("expected invalid command response, got %v", err)
			}

			extensions, err := session.Extensions(context.Background())
			if test.validExt {
				if err != nil {
					t.Fatalf("read extensions: %v", err)
				}
				if (extensions.Rest == nil) != test.extRestNil {
					t.Fatalf("unexpected extension section presence: %#v", extensions)
				}
			} else if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) || appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
				t.Fatalf("expected invalid extension response, got %v", err)
			}
			if len(recorder.Requests()) != 3 {
				t.Fatalf("unexpected request count: %d", len(recorder.Requests()))
			}
		})
	}
}

func TestDP041RejectsMalformedKnownFields(t *testing.T) {
	cases := []struct {
		name       string
		commands   string
		extensions string
	}{
		{name: "rest null", commands: `{"rest":null}`, extensions: `{}`},
		{name: "bidi null", commands: `{"bidi":null}`, extensions: `{}`},
		{name: "plugins null", commands: `{"rest":{"base":{},"driver":{},"plugins":null}}`, extensions: `{}`},
		{name: "plugin value null", commands: `{"rest":{"base":{},"driver":{},"plugins":{"p":null}}}`, extensions: `{}`},
		{name: "empty path", commands: `{"rest":{"base":{"":{"GET":{}}},"driver":{}}}`, extensions: `{}`},
		{name: "empty method", commands: `{"rest":{"base":{"/path":{"":{}}},"driver":{}}}`, extensions: `{}`},
		{name: "empty plugin name", commands: `{"rest":{"base":{},"driver":{},"plugins":{"":{}}}}`, extensions: `{}`},
		{name: "empty bidi domain", commands: `{"bidi":{"base":{"":{"name":{}}},"driver":{}}}`, extensions: `{}`},
		{name: "empty bidi name", commands: `{"bidi":{"base":{"domain":{"":{}}},"driver":{}}}`, extensions: `{}`},
		{name: "empty execute method", commands: `{}`, extensions: `{"rest":{"driver":{"":{}}}}`},
		{name: "metadata command null", commands: `{"rest":{"base":{"/path":{"GET":{"command":null}}},"driver":{}}}`, extensions: `{}`},
		{name: "metadata deprecated wrong type", commands: `{"rest":{"base":{"/path":{"GET":{"deprecated":"no"}}},"driver":{}}}`, extensions: `{}`},
		{name: "metadata params null", commands: `{"rest":{"base":{"/path":{"GET":{"params":null}}},"driver":{}}}`, extensions: `{}`},
		{name: "param missing name", commands: `{"rest":{"base":{"/path":{"GET":{"params":[{"required":true}]}}},"driver":{}}}`, extensions: `{}`},
		{name: "param empty name", commands: `{"rest":{"base":{"/path":{"GET":{"params":[{"name":"","required":true}]}}},"driver":{}}}`, extensions: `{}`},
		{name: "param missing required", commands: `{"rest":{"base":{"/path":{"GET":{"params":[{"name":"x"}]}}},"driver":{}}}`, extensions: `{}`},
		{name: "param required wrong type", commands: `{"rest":{"base":{"/path":{"GET":{"params":[{"name":"x","required":1}]}}},"driver":{}}}`, extensions: `{}`},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			session, _ := newDP041DiscoveryTestSession(t, `{"value":`+test.commands+`}`, `{"value":`+test.extensions+`}`)
			if test.commands != `{}` {
				_, err := session.Commands(context.Background())
				if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) || appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
					t.Fatalf("expected invalid command response, got %v", err)
				}
			}
			if test.extensions != `{}` {
				_, err := session.Extensions(context.Background())
				if err == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) || appium.DeliveryOf(err) != appium.DeliveryAcknowledged {
					t.Fatalf("expected invalid extension response, got %v", err)
				}
			}
		})
	}
}

func newDP041DiscoveryTestSession(t *testing.T, commandsResponse, extensionsResponse string) (*appium.Session, *contracttest.Recorder) {
	t.Helper()
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.RequestURI == "/session":
			_, _ = writer.Write([]byte(`{"value":{"sessionId":"session","capabilities":{"automationName":"XCUITest"}}}`))
		case request.Method == http.MethodGet && request.RequestURI == "/session/session/appium/commands":
			_, _ = writer.Write([]byte(commandsResponse))
		case request.Method == http.MethodGet && request.RequestURI == "/session/session/appium/extensions":
			_, _ = writer.Write([]byte(extensionsResponse))
		default:
			http.NotFound(writer, request)
		}
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
	client, err := server.NewClient(appium.ClientOptions{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"platformName": "iOS", "appium:automationName": "XCUITest",
	}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return session, recorder
}

func findDP041HTTPCommand(t *testing.T, entries []appium.HTTPCommand, method, path string) appium.HTTPCommand {
	t.Helper()
	for _, entry := range entries {
		if entry.Method == method && entry.Path == path {
			return entry
		}
	}
	identities := make([]string, 0, len(entries))
	for _, entry := range entries {
		identities = append(identities, entry.Method+" "+entry.Path)
	}
	sort.Strings(identities)
	t.Fatalf("catalog entry %s %s not found; entries=%v", method, path, identities)
	return appium.HTTPCommand{}
}

func findDP041BiDiCommand(t *testing.T, entries []appium.BiDiCommand, domain, name string) appium.BiDiCommand {
	t.Helper()
	for _, entry := range entries {
		if entry.Domain == domain && entry.Name == name {
			return entry
		}
	}
	t.Fatalf("BiDi catalog entry %s/%s not found", domain, name)
	return appium.BiDiCommand{}
}
