package xcuitest_test

import (
	"context"
	"errors"
	"math"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
	"github.com/xieliangji/soluna-appium-client/xcuitest"
)

type simulatedLocationCommand struct {
	name      string
	operation string
	script    string
	args      []any
	run       func(context.Context, *appium.Session) (*xcuitest.SimulatedLocation, error)
}

func simulatedLocationCommands() []simulatedLocationCommand {
	return []simulatedLocationCommand{
		{"get", "ios_get_simulated_location", "mobile: getSimulatedLocation", []any{}, xcuitest.IOSGetSimulatedLocation},
		{"set", "ios_set_simulated_location", "mobile: setSimulatedLocation", []any{map[string]any{"latitude": 12.5, "longitude": -45.25}},
			func(ctx context.Context, session *appium.Session) (*xcuitest.SimulatedLocation, error) {
				return nil, xcuitest.IOSSetSimulatedLocation(ctx, session, xcuitest.SimulatedLocation{Latitude: 12.5, Longitude: -45.25})
			}},
		{"reset", "ios_reset_simulated_location", "mobile: resetSimulatedLocation", []any{},
			func(ctx context.Context, session *appium.Session) (*xcuitest.SimulatedLocation, error) {
				return nil, xcuitest.IOSResetSimulatedLocation(ctx, session)
			}},
	}
}

func TestIOSSimulatedLocationProtocol(t *testing.T) {
	for _, command := range simulatedLocationCommands() {
		t.Run(command.name, func(t *testing.T) {
			observer := &operationObserver{}
			session, recorder := newSimulatedLocationSession(t, "XCUITest", appium.ClientOptions{Observer: observer},
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if command.name == "get" {
						_, _ = w.Write([]byte(`{"value":{"latitude":12.5,"longitude":-45.25,"altitude":0}}`))
						return
					}
					_, _ = w.Write([]byte("{\"value\": \n null \t}"))
				}))
			copy := *session
			observer.reset()
			location, err := command.run(context.Background(), &copy)
			if err != nil {
				t.Fatalf("execute simulated location command: %v", err)
			}
			if command.name == "get" && !reflect.DeepEqual(location, &xcuitest.SimulatedLocation{Latitude: 12.5, Longitude: -45.25}) {
				t.Fatalf("unexpected location: %+v", location)
			}
			assertSimulatedLocationRequest(t, recorder, command.script, command.args)
			assertSimulatedLocationObserved(t, observer, command.operation, "", appium.DeliveryAcknowledged, http.StatusOK)
		})
	}
}

func TestIOSGetSimulatedLocationReadsFreshSnapshots(t *testing.T) {
	snapshots := []struct {
		value string
		want  *xcuitest.SimulatedLocation
	}{
		{`{"latitude":null,"longitude":null,"altitude":null}`, nil},
		{`{"latitude":0,"longitude":0}`, &xcuitest.SimulatedLocation{}},
		{`{"latitude":12.5,"longitude":-45.25,"altitude":0,"extra":{"source":"synthetic"}}`, &xcuitest.SimulatedLocation{Latitude: 12.5, Longitude: -45.25}},
		{`{"latitude":90,"longitude":180}`, &xcuitest.SimulatedLocation{Latitude: 90, Longitude: 180}},
		{`{"latitude":-90,"longitude":-180}`, &xcuitest.SimulatedLocation{Latitude: -90, Longitude: -180}},
		{`{"latitude":1.25e1,"longitude":-4.525e1}`, &xcuitest.SimulatedLocation{Latitude: 12.5, Longitude: -45.25}},
		{`{"latitude": null ,"longitude": null }`, nil},
	}
	var index atomic.Int32
	session, recorder := newSimulatedLocationSession(t, "XCUITest", appium.ClientOptions{},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			i := int(index.Add(1)) - 1
			if i >= len(snapshots) {
				t.Error("unexpected additional request")
				http.Error(w, "unexpected request", http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{"value":` + snapshots[i].value + `}`))
		}))
	for _, snapshot := range snapshots {
		recorder.Reset()
		got, err := xcuitest.IOSGetSimulatedLocation(context.Background(), session)
		if err != nil || !reflect.DeepEqual(got, snapshot.want) {
			t.Fatalf("location: got %+v / %v, want %+v", got, err, snapshot.want)
		}
		assertSimulatedLocationRequest(t, recorder, "mobile: getSimulatedLocation", []any{})
		if got != nil {
			got.Latitude = 999 // A caller-owned result must not affect later reads.
		}
	}
}

func TestIOSSetSimulatedLocationSendsZeroAndBoundaryCoordinates(t *testing.T) {
	session, recorder := newSimulatedLocationSession(t, "XCUITest", appium.ClientOptions{},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"value":null}`))
		}))
	for _, location := range []xcuitest.SimulatedLocation{
		{},
		{Latitude: -90, Longitude: -180},
		{Latitude: 90, Longitude: 180},
		{Latitude: math.Nextafter(90, 0), Longitude: math.Nextafter(-180, 0)},
		{Latitude: 12.5, Longitude: -45.25},
	} {
		recorder.Reset()
		if err := xcuitest.IOSSetSimulatedLocation(context.Background(), session, location); err != nil {
			t.Fatalf("set valid location: %v", err)
		}
		assertSimulatedLocationRequest(t, recorder, "mobile: setSimulatedLocation", []any{map[string]any{
			"latitude": location.Latitude, "longitude": location.Longitude,
		}})
	}
}

func TestIOSSetSimulatedLocationRejectsInvalidCoordinatesWithoutRequests(t *testing.T) {
	observer := &operationObserver{}
	session, recorder := newSimulatedLocationSession(t, "XCUITest", appium.ClientOptions{Observer: observer}, http.NotFoundHandler())
	observer.reset()
	for _, test := range []struct {
		name     string
		location xcuitest.SimulatedLocation
	}{
		{"latitude below minimum", xcuitest.SimulatedLocation{Latitude: math.Nextafter(-90, math.Inf(-1))}},
		{"latitude above maximum", xcuitest.SimulatedLocation{Latitude: math.Nextafter(90, math.Inf(1))}},
		{"latitude NaN", xcuitest.SimulatedLocation{Latitude: math.NaN()}},
		{"latitude positive infinity", xcuitest.SimulatedLocation{Latitude: math.Inf(1)}},
		{"latitude negative infinity", xcuitest.SimulatedLocation{Latitude: math.Inf(-1)}},
		{"longitude below minimum", xcuitest.SimulatedLocation{Longitude: math.Nextafter(-180, math.Inf(-1))}},
		{"longitude above maximum", xcuitest.SimulatedLocation{Longitude: math.Nextafter(180, math.Inf(1))}},
		{"longitude NaN", xcuitest.SimulatedLocation{Longitude: math.NaN()}},
		{"longitude positive infinity", xcuitest.SimulatedLocation{Longitude: math.Inf(1)}},
		{"longitude negative infinity", xcuitest.SimulatedLocation{Longitude: math.Inf(-1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := xcuitest.IOSSetSimulatedLocation(context.Background(), session, test.location)
			assertSimulatedLocationError(t, err, "ios_set_simulated_location", appium.CodeInvalidArgument, appium.DeliveryNotSent, 0)
			if len(recorder.Requests()) != 0 || len(observer.started) != 0 || len(observer.finished) != 0 {
				t.Fatal("invalid coordinates entered the remote command chain")
			}
		})
	}
}

func TestIOSSimulatedLocationRequiresConfirmedXCUITestSession(t *testing.T) {
	for _, driver := range []string{"UiAutomator2", "xcuitest", "XCUITest ", "CustomDriver"} {
		t.Run(driver, func(t *testing.T) {
			// The request asks for XCUITest; only the response may authorize platform commands.
			session, recorder := newSimulatedLocationSession(t, driver, appium.ClientOptions{}, http.NotFoundHandler())
			for _, command := range simulatedLocationCommands() {
				t.Run(command.name, func(t *testing.T) {
					location, err := command.run(context.Background(), session)
					assertSimulatedLocationError(t, err, command.operation, appium.CodeUnsupported, appium.DeliveryNotSent, 0)
					if location != nil || len(recorder.Requests()) != 0 {
						t.Fatal("driver mismatch returned a location or sent a request")
					}
				})
			}
		})
	}
}

func TestIOSSimulatedLocationRejectsInvalidSessionAndContextWithoutRequests(t *testing.T) {
	session, recorder := newSimulatedLocationSession(t, "XCUITest", appium.ClientOptions{}, http.NotFoundHandler())
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Unix(0, 0))
	t.Cleanup(expire)
	for _, command := range simulatedLocationCommands() {
		t.Run(command.name, func(t *testing.T) {
			for _, test := range []struct {
				name    string
				ctx     context.Context
				session *appium.Session
				code    appium.ErrorCode
				cause   error
			}{
				{"nil session", context.Background(), nil, appium.CodeInvalidArgument, nil},
				{"zero session", context.Background(), &appium.Session{}, appium.CodeInvalidArgument, nil},
				{"nil context", nil, session, appium.CodeInvalidArgument, nil},
				{"canceled context", canceled, session, appium.CodeCanceled, context.Canceled},
				{"expired context", expired, session, appium.CodeDeadlineExceeded, context.DeadlineExceeded},
			} {
				t.Run(test.name, func(t *testing.T) {
					location, err := command.run(test.ctx, test.session)
					assertSimulatedLocationError(t, err, command.operation, test.code, appium.DeliveryNotSent, 0)
					if test.cause != nil && !errors.Is(err, test.cause) {
						t.Fatalf("context cause was lost: %v", err)
					}
					if location != nil || len(recorder.Requests()) != 0 {
						t.Fatal("local failure returned a location or sent a request")
					}
				})
			}
		})
	}
	copy := *session
	if err := session.Close(context.Background()); err != nil {
		t.Fatalf("close session: %v", err)
	}
	// Close must not implicitly reset simulated location.
	if len(recorder.Requests()) != 1 || recorder.Requests()[0].Method != http.MethodDelete {
		t.Fatal("session close performed extra commands")
	}
	recorder.Reset()
	for _, command := range simulatedLocationCommands() {
		t.Run("closed "+command.name, func(t *testing.T) {
			location, err := command.run(context.Background(), &copy)
			assertSimulatedLocationError(t, err, command.operation, appium.CodeSessionLost, appium.DeliveryNotSent, 0)
			if location != nil || len(recorder.Requests()) != 0 {
				t.Fatal("closed session returned a location or sent a request")
			}
		})
	}
}

func TestIOSGetSimulatedLocationRejectsMalformedSnapshotsInCommandChain(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
	}{
		{"top-level null", `null`},
		{"boolean", `false`},
		{"array", `[]`},
		{"number", `0`},
		{"string", `"location"`},
		{"empty object", `{}`},
		{"missing latitude", `{"longitude":0}`},
		{"missing longitude", `{"latitude":0}`},
		{"missing longitude with null latitude", `{"latitude":null}`},
		{"wrong case latitude", `{"Latitude":0,"longitude":0}`},
		{"wrong case longitude", `{"latitude":0,"Longitude":0}`},
		{"null latitude only", `{"latitude":null,"longitude":0}`},
		{"null longitude only", `{"latitude":0,"longitude":null}`},
		{"string latitude", `{"latitude":"12.5","longitude":0}`},
		{"string longitude", `{"latitude":0,"longitude":"12.5"}`},
		{"boolean latitude", `{"latitude":true,"longitude":0}`},
		{"object longitude", `{"latitude":0,"longitude":{}}`},
		{"array latitude", `{"latitude":[],"longitude":0}`},
		{"latitude below minimum", `{"latitude":-90.0000001,"longitude":0}`},
		{"latitude above maximum", `{"latitude":90.0000001,"longitude":0}`},
		{"longitude below minimum", `{"latitude":0,"longitude":-180.0000001}`},
		{"longitude above maximum", `{"latitude":0,"longitude":180.0000001}`},
		{"latitude overflow", `{"latitude":1e400,"longitude":0}`},
		{"longitude overflow", `{"latitude":0,"longitude":-1e400}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			observer := &operationObserver{}
			session, recorder := newSimulatedLocationSession(t, "XCUITest", appium.ClientOptions{Observer: observer},
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = w.Write([]byte(`{"value":` + test.value + `}`))
				}))
			observer.reset()
			location, err := xcuitest.IOSGetSimulatedLocation(context.Background(), session)
			assertSimulatedLocationError(t, err, "ios_get_simulated_location", appium.CodeResponseInvalid, appium.DeliveryAcknowledged, http.StatusOK)
			if location != nil {
				t.Fatalf("invalid response returned partial location: %+v", location)
			}
			assertSimulatedLocationRequest(t, recorder, "mobile: getSimulatedLocation", []any{})
			assertSimulatedLocationObserved(t, observer, "ios_get_simulated_location", appium.CodeResponseInvalid, appium.DeliveryAcknowledged, http.StatusOK)
		})
	}
}

func TestIOSSimulatedLocationErrorsStayInCommandChain(t *testing.T) {
	for _, command := range simulatedLocationCommands() {
		t.Run(command.name, func(t *testing.T) {
			for _, test := range []struct {
				name       string
				body       string
				status     int
				code       appium.ErrorCode
				remoteCode string
			}{
				{"boolean", `{"value":true}`, 200, appium.CodeResponseInvalid, ""},
				{"number", `{"value":0}`, 200, appium.CodeResponseInvalid, ""},
				{"string", `{"value":"null"}`, 200, appium.CodeResponseInvalid, ""},
				{"object", `{"value":{}}`, 200, appium.CodeResponseInvalid, ""},
				{"array", `{"value":[]}`, 200, appium.CodeResponseInvalid, ""},
				{"missing value", `{}`, 200, appium.CodeResponseInvalid, ""},
				{"malformed envelope", `{"value":`, 200, appium.CodeResponseInvalid, ""},
				{"unsupported", `{"value":{"error":"unknown command","message":"unavailable"}}`, 404, appium.CodeUnsupported, "unknown command"},
				{"unsupported operation", `{"value":{"error":"unsupported operation","message":"unavailable"}}`, 500, appium.CodeUnsupported, "unsupported operation"},
				{"invalid argument", `{"value":{"error":"invalid argument","message":"invalid parameters"}}`, 400, appium.CodeInvalidArgument, "invalid argument"},
				{"session lost", `{"value":{"error":"invalid session id","message":"missing"}}`, 404, appium.CodeSessionLost, "invalid session id"},
				{"WDA failure", `{"value":{"error":"unknown error","message":"location simulation failed"}}`, 500, appium.CodeCommandFailed, "unknown error"},
			} {
				t.Run(test.name, func(t *testing.T) {
					observer := &operationObserver{}
					session, recorder := newSimulatedLocationSession(t, "XCUITest", appium.ClientOptions{Observer: observer},
						http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(test.status)
							_, _ = w.Write([]byte(test.body))
						}))
					observer.reset()
					location, err := command.run(context.Background(), session)
					commandErr := assertSimulatedLocationError(t, err, command.operation, test.code, appium.DeliveryAcknowledged, test.status)
					if location != nil || commandErr.RemoteCode != test.remoteCode {
						t.Fatalf("unexpected result: %+v / %+v", location, commandErr)
					}
					assertSimulatedLocationRequest(t, recorder, command.script, command.args)
					assertSimulatedLocationObserved(t, observer, command.operation, commandErr.Code, commandErr.Delivery, commandErr.StatusCode)
				})
			}
		})
	}
}

func TestIOSSimulatedLocationHonorsResponseLimit(t *testing.T) {
	for _, command := range simulatedLocationCommands() {
		t.Run(command.name, func(t *testing.T) {
			observer := &operationObserver{}
			session, recorder := newSimulatedLocationSession(t, "XCUITest", appium.ClientOptions{Observer: observer, Limits: appium.Limits{MaxResponseBytes: 512}},
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = w.Write([]byte(`{"value":null,"padding":"` + strings.Repeat("x", 600) + `"}`))
				}))
			observer.reset()
			location, err := command.run(context.Background(), session)
			assertSimulatedLocationError(t, err, command.operation, appium.CodeResponseTooLarge, appium.DeliveryAcknowledged, http.StatusOK)
			if location != nil {
				t.Fatal("oversized response returned a location")
			}
			assertSimulatedLocationRequest(t, recorder, command.script, command.args)
			assertSimulatedLocationObserved(t, observer, command.operation, appium.CodeResponseTooLarge, appium.DeliveryAcknowledged, http.StatusOK)
		})
	}
}

func TestIOSSimulatedLocationDoesNotReplayAfterCancellation(t *testing.T) {
	for _, command := range simulatedLocationCommands() {
		t.Run(command.name, func(t *testing.T) {
			entered := make(chan struct{}, 1)
			observer := &operationObserver{}
			session, recorder := newSimulatedLocationSession(t, "XCUITest", appium.ClientOptions{Observer: observer},
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					select {
					case entered <- struct{}{}:
					default:
					}
					<-r.Context().Done()
				}))
			observer.reset()
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			result := make(chan error, 1)
			go func() {
				location, err := command.run(ctx, session)
				if location != nil {
					t.Error("canceled command returned a location")
				}
				result <- err
			}()
			<-entered
			cancel()
			err := <-result
			assertSimulatedLocationError(t, err, command.operation, appium.CodeCanceled, appium.DeliveryUnknown, 0)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation cause was lost: %v", err)
			}
			assertSimulatedLocationRequest(t, recorder, command.script, command.args)
			assertSimulatedLocationObserved(t, observer, command.operation, appium.CodeCanceled, appium.DeliveryUnknown, 0)
		})
	}
}

func assertSimulatedLocationError(t *testing.T, err error, operation string, code appium.ErrorCode, delivery appium.DeliveryState, status int) *appium.Error {
	t.Helper()
	var commandErr *appium.Error
	if !errors.As(err, &commandErr) || commandErr == nil {
		t.Fatalf("expected structured simulated location error, got %v", err)
	}
	if commandErr.Operation != operation || commandErr.Code != code || commandErr.Delivery != delivery || commandErr.StatusCode != status {
		t.Fatalf("unexpected error: %+v; want %s / %s / %s / %d", commandErr, operation, code, delivery, status)
	}
	return commandErr
}

func assertSimulatedLocationObserved(t *testing.T, observer *operationObserver, operation string, code appium.ErrorCode, delivery appium.DeliveryState, status int) {
	t.Helper()
	if len(observer.started) != 1 || len(observer.finished) != 1 {
		t.Fatalf("unexpected observer event counts: %+v / %+v", observer.started, observer.finished)
	}
	finished := observer.finished[0]
	if observer.started[0].Operation != operation || finished.Operation != operation ||
		finished.ErrorCode != code || finished.StatusCode != status || finished.Delivery != delivery {
		t.Fatalf("unexpected observer events: %+v / %+v", observer.started, observer.finished)
	}
}

func assertSimulatedLocationRequest(t *testing.T, recorder *contracttest.Recorder, script string, args []any) {
	t.Helper()
	requests := recorder.Requests()
	if len(requests) != 1 {
		t.Fatalf("expected one simulated location request, got %d", len(requests))
	}
	request := requests[0]
	for _, err := range []error{
		contracttest.MatchMethod(request, http.MethodPost),
		contracttest.MatchRequestURI(request, "/session/location%2Fsession/execute/sync"),
		contracttest.MatchHeader(request, "Content-Type", "application/json"),
		contracttest.MatchJSONBody(request, map[string]any{"script": script, "args": args}),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func newSimulatedLocationSession(t *testing.T, driver string, options appium.ClientOptions, command http.Handler) (*appium.Session, *contracttest.Recorder) {
	t.Helper()
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.RequestURI == "/session":
			_, _ = w.Write([]byte(`{"value":{"sessionId":"location/session","capabilities":{"automationName":"` + driver + `"}}}`))
		case r.Method == http.MethodDelete && r.RequestURI == "/session/location%2Fsession":
			_, _ = w.Write([]byte(`{"value":null}`))
		case r.Method == http.MethodPost && r.RequestURI == "/session/location%2Fsession/execute/sync":
			command.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
	client, err := server.NewClient(options)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{"appium:automationName": "XCUITest"}))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	recorder.Reset()
	return session, recorder
}
