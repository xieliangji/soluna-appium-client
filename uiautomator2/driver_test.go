package uiautomator2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

// UIA-001 只有内部门禁；Session 均由公共 CreateSession 入口创建。
func TestRequireUiAutomator2SessionUsesExactRemoteAutomationName(t *testing.T) {
	for _, test := range []struct {
		name      string
		requested string
		remote    string
		wantCode  appium.ErrorCode
	}{
		{"matching driver", "UiAutomator2", "UiAutomator2", ""},
		{"remote overrides request", "XCUITest", "UiAutomator2", ""},
		{"request cannot override remote", "UiAutomator2", "XCUITest", appium.CodeUnsupported},
		{"other Android driver", "UiAutomator2", "Espresso", appium.CodeUnsupported},
		{"unknown driver", "UiAutomator2", "CustomDriver", appium.CodeUnsupported},
		{"lowercase", "UiAutomator2", "uiautomator2", appium.CodeUnsupported},
		{"uppercase", "UiAutomator2", "UIAUTOMATOR2", appium.CodeUnsupported},
		{"leading whitespace", "UiAutomator2", " UiAutomator2", appium.CodeUnsupported},
		{"trailing whitespace", "UiAutomator2", "UiAutomator2\t", appium.CodeUnsupported},
		{"whitespace only", "UiAutomator2", " ", appium.CodeUnsupported},
	} {
		t.Run(test.name, func(t *testing.T) {
			requested := appium.Capabilities{
				"platformName":          "Android",
				"appium:automationName": test.requested,
			}
			session, recorder, observer := newDriverGateSession(t, requested, test.remote)
			// 调用方可修改请求和返回快照，但不能改变远端确认的 Driver。
			requested["appium:automationName"] = "ChangedDriver"
			snapshot := session.Capabilities()
			snapshot["automationName"] = "ChangedDriver"
			copiedSession := *session
			for _, current := range []*appium.Session{session, &copiedSession} {
				const operation = "android_driver_gate_test"
				err := requireUiAutomator2Session(current, operation)
				if test.wantCode == "" {
					if err != nil {
						t.Fatalf("accept remote UiAutomator2 session: %v", err)
					}
				} else {
					assertDriverGateError(t, err, operation, test.wantCode)
				}
			}
			if len(recorder.Requests()) != 0 || observer.events.Load() != 0 {
				t.Fatal("driver gate entered the remote command chain")
			}
		})
	}
}

func TestRequireUiAutomator2SessionRejectsUninitializedSessions(t *testing.T) {
	for _, test := range []struct {
		name    string
		session *appium.Session
	}{
		{"nil", nil},
		{"zero value", &appium.Session{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			const operation = "android_uninitialized_gate_test"
			err := requireUiAutomator2Session(test.session, operation)
			assertDriverGateError(t, err, operation, appium.CodeInvalidArgument)
		})
	}
}

func TestRequireUiAutomator2SessionRejectsCleanupOnlySession(t *testing.T) {
	observer := &driverGateObserver{}
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.RequestURI == "/session":
			// 请求要求 UiAutomator2，但响应缺少 automationName，自动清理也失败。
			_, _ = w.Write([]byte(`{"value":{"sessionId":"cleanup/session","capabilities":{"platformName":"Android"}}}`))
		case r.Method == http.MethodDelete && r.RequestURI == "/session/cleanup%2Fsession":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"value":{"error":"unknown error","message":"cleanup failed"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
	client, err := server.NewClient(appium.ClientOptions{Observer: observer})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(appium.Capabilities{
		"appium:automationName": "UiAutomator2",
	}))
	if session == nil || !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
		t.Fatalf("expected cleanup-only session and invalid response, got %v / %v", session, err)
	}
	if session.ID() != "cleanup/session" || session.AutomationName() != "" || len(recorder.Requests()) != 2 {
		t.Fatal("expected failed initialization followed by automatic cleanup")
	}
	recorder.Reset()
	observer.events.Store(0)

	const operation = "android_cleanup_gate_test"
	err = requireUiAutomator2Session(session, operation)
	assertDriverGateError(t, err, operation, appium.CodeInvalidArgument)
	if len(recorder.Requests()) != 0 || observer.events.Load() != 0 {
		t.Fatal("cleanup-only session entered the remote command chain")
	}
}

func TestRequireUiAutomator2SessionLeavesClosedStateToRootCommandChain(t *testing.T) {
	session, recorder, observer := newDriverGateSession(t, appium.Capabilities{
		"appium:automationName": "UiAutomator2",
	}, "UiAutomator2")
	copiedSession := *session
	if err := session.Close(context.Background()); err != nil {
		t.Fatalf("close session: %v", err)
	}
	recorder.Reset()
	observer.events.Store(0)

	const operation = "android_closed_gate_test"
	for _, current := range []*appium.Session{session, &copiedSession} {
		if err := requireUiAutomator2Session(current, operation); err != nil {
			t.Fatalf("driver gate must leave close validation to root: %v", err)
		}
		_, err := current.ExecuteScriptWithOperation(context.Background(), operation, "return null", nil)
		assertDriverGateError(t, err, operation, appium.CodeSessionLost)
	}
	if len(recorder.Requests()) != 0 || observer.events.Load() != 0 {
		t.Fatal("closed session entered the remote command chain")
	}
}

func assertDriverGateError(t *testing.T, err error, operation string, code appium.ErrorCode) {
	t.Helper()
	var commandErr *appium.Error
	if !errors.As(err, &commandErr) || commandErr == nil {
		t.Fatalf("expected structured driver gate error, got %v", err)
	}
	if commandErr.Operation != operation || commandErr.Code != code || commandErr.Delivery != appium.DeliveryNotSent || commandErr.StatusCode != 0 {
		t.Fatalf("unexpected error: %+v; want %s / %s / %s / 0", commandErr, operation, code, appium.DeliveryNotSent)
	}
}

func newDriverGateSession(t *testing.T, requested appium.Capabilities, remoteDriver string) (*appium.Session, *contracttest.Recorder, *driverGateObserver) {
	t.Helper()
	observer := &driverGateObserver{}
	recorder := contracttest.NewRecorder(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.RequestURI == "/session":
			response := struct {
				Value struct {
					SessionID    string              `json:"sessionId"`
					Capabilities appium.Capabilities `json:"capabilities"`
				} `json:"value"`
			}{}
			response.Value.SessionID = "gate/session"
			response.Value.Capabilities = appium.Capabilities{
				"platformName":          "Android",
				"automationName":        remoteDriver,
				"appium:automationName": "UiAutomator2",
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("encode session response: %v", err)
			}
		case r.Method == http.MethodDelete && r.RequestURI == "/session/gate%2Fsession":
			_, _ = w.Write([]byte(`{"value":null}`))
		default:
			http.NotFound(w, r)
		}
	}))
	server := contracttest.NewServer(recorder)
	t.Cleanup(server.Close)
	client, err := server.NewClient(appium.ClientOptions{Observer: observer})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	session, err := client.CreateSession(context.Background(), appium.MatchCapabilities(requested))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	recorder.Reset()
	observer.events.Store(0)
	return session, recorder, observer
}

type driverGateObserver struct {
	events atomic.Int32
}

func (o *driverGateObserver) OnCommandStarted(appium.CommandStartedEvent) {
	o.events.Add(1)
}

func (o *driverGateObserver) OnCommandFinished(appium.CommandFinishedEvent) {
	o.events.Add(1)
}
