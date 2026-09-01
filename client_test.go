package appium_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
)

func TestNewClientAcceptsZeroOptions(t *testing.T) {
	client, err := appium.NewClient(
		"http://127.0.0.1:4723",
		appium.ClientOptions{},
	)
	if err != nil {
		t.Fatalf(
			"create client with zero options: %v",
			err,
		)
	}

	if client == nil {
		t.Fatal(
			"expected non-nil client",
		)
	}
}

func TestNewClientDoesNotMutateProvidedHTTPClient(t *testing.T) {
	httpClient := &http.Client{}

	options := appium.ClientOptions{
		HTTPClient: httpClient,
	}

	client, err := appium.NewClient(
		"http://127.0.0.1:4723",
		options,
	)
	if err != nil {
		t.Fatalf(
			"create client: %v",
			err,
		)
	}

	if client == nil {
		t.Fatal(
			"expected non-nil client",
		)
	}

	// Transport 会为内部 HTTP Client 设置 Redirect 策略，
	// 但不能修改调用方传入的原始 HTTP Client。
	if httpClient.CheckRedirect != nil {
		t.Fatal(
			"provided HTTP client was mutated",
		)
	}

	if httpClient.Timeout != 0 {
		t.Fatalf(
			"provided HTTP client timeout was mutated: %v",
			httpClient.Timeout,
		)
	}

	// NewClient 内部会补全默认配置，
	// 但 ClientOptions 是调用方值，不能被反向修改。
	if options.CommandTimeout != 0 {
		t.Fatalf(
			"caller command timeout was mutated: %v",
			options.CommandTimeout,
		)
	}

	if options.ReadyProbeTimeout != 0 {
		t.Fatalf(
			"caller ready probe timeout was mutated: %v",
			options.ReadyProbeTimeout,
		)
	}

	if options.SessionCleanupTimeout != 0 {
		t.Fatalf(
			"caller session cleanup timeout was mutated: %v",
			options.SessionCleanupTimeout,
		)
	}

	if options.Limits != (appium.Limits{}) {
		t.Fatalf(
			"caller limits were mutated: %+v",
			options.Limits,
		)
	}
}

func TestNewClientRejectsInvalidServerURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "empty",
			url:  "",
		},
		{
			name: "missing scheme",
			url:  "127.0.0.1:4723",
		},
		{
			name: "unsupported scheme",
			url:  "ftp://127.0.0.1:4723",
		},
		{
			name: "missing host",
			url:  "http://",
		},
		{
			name: "user info",
			url:  "http://user:password@127.0.0.1:4723",
		},
		{
			name: "query",
			url:  "http://127.0.0.1:4723?token=value",
		},
		{
			name: "fragment",
			url:  "http://127.0.0.1:4723#fragment",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				client, err := appium.NewClient(
					test.url,
					appium.ClientOptions{},
				)

				if err == nil {
					t.Fatal(
						"expected invalid server URL error",
					)
				}

				if client != nil {
					t.Fatal(
						"invalid configuration must not return a client",
					)
				}

				assertInvalidClientConfig(
					t,
					err,
				)
			},
		)
	}
}

func TestNewClientRejectsHTTPClientTimeout(t *testing.T) {
	client, err := appium.NewClient(
		"http://127.0.0.1:4723",
		appium.ClientOptions{
			HTTPClient: &http.Client{
				Timeout: time.Second,
			},
		},
	)

	if err == nil {
		t.Fatal(
			"expected HTTP client timeout configuration error",
		)
	}

	if client != nil {
		t.Fatal(
			"invalid HTTP client configuration must not return a client",
		)
	}

	assertInvalidClientConfig(
		t,
		err,
	)
}

func TestNewClientRejectsNegativeConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		options appium.ClientOptions
	}{
		{
			name: "command timeout",
			options: appium.ClientOptions{
				CommandTimeout: -time.Nanosecond,
			},
		},
		{
			name: "ready probe timeout",
			options: appium.ClientOptions{
				ReadyProbeTimeout: -time.Nanosecond,
			},
		},
		{
			name: "session cleanup timeout",
			options: appium.ClientOptions{
				SessionCleanupTimeout: -time.Nanosecond,
			},
		},
		{
			name: "response limit",
			options: appium.ClientOptions{
				Limits: appium.Limits{
					MaxResponseBytes: -1,
				},
			},
		},
		{
			name: "page source response limit",
			options: appium.ClientOptions{
				Limits: appium.Limits{
					MaxPageSourceResponseBytes: -1,
				},
			},
		},
		{
			name: "recording response limit",
			options: appium.ClientOptions{
				Limits: appium.Limits{
					MaxRecordingResponseBytes: -1,
				},
			},
		},
		{
			name: "log response limit",
			options: appium.ClientOptions{
				Limits: appium.Limits{
					MaxLogResponseBytes: -1,
				},
			},
		},
		{
			name: "remote error limit",
			options: appium.ClientOptions{
				Limits: appium.Limits{
					MaxRemoteErrorBytes: -1,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				client, err := appium.NewClient(
					"http://127.0.0.1:4723",
					test.options,
				)

				if err == nil {
					t.Fatal(
						"expected invalid client configuration error",
					)
				}

				if client != nil {
					t.Fatal(
						"invalid configuration must not return a client",
					)
				}

				assertInvalidClientConfig(
					t,
					err,
				)
			},
		)
	}
}

// assertInvalidClientConfig 校验 NewClient 的本地配置错误契约。
func assertInvalidClientConfig(
	t *testing.T,
	err error,
) {
	t.Helper()

	if !appium.IsErrorCode(
		err,
		appium.CodeInvalidConfig,
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

	var clientErr *appium.Error

	if !errors.As(
		err,
		&clientErr,
	) {
		t.Fatalf(
			"expected *appium.Error, got %T",
			err,
		)
	}

	if clientErr.Operation != "new_client" {
		t.Fatalf(
			"unexpected operation: %q",
			clientErr.Operation,
		)
	}

	if clientErr.StatusCode != 0 {
		t.Fatalf(
			"local configuration error must not have HTTP status: %d",
			clientErr.StatusCode,
		)
	}
}
