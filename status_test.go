package appium_test

import (
	"context"
	"net/http"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestStatus(t *testing.T) {
	recorder := contracttest.NewRecorder(
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.Header().Set(
					"Content-Type",
					"application/json",
				)

				_, _ = writer.Write(
					[]byte(
						`{"value":{"ready":true,"message":"ready","build":{"version":"3.0.0"}}}`,
					),
				)
			},
		),
	)

	server := contracttest.NewServer(recorder)
	defer server.Close()

	client, err := server.NewClient(
		appium.ClientOptions{},
	)
	if err != nil {
		t.Fatalf(
			"create client: %v",
			err,
		)
	}

	status, err := client.Status(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"get status: %v",
			err,
		)
	}

	expected := appium.ServerStatus{
		Ready:   true,
		Message: "ready",
		Version: "3.0.0",
	}

	if status != expected {
		t.Fatalf(
			"unexpected status: expected %+v, got %+v",
			expected,
			status,
		)
	}

	requests := recorder.Requests()
	if len(requests) != 1 {
		t.Fatalf(
			"unexpected request count: expected 1, got %d",
			len(requests),
		)
	}

	request := requests[0]

	if err := contracttest.MatchMethod(
		request,
		http.MethodGet,
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchRequestURI(
		request,
		"/status",
	); err != nil {
		t.Fatal(err)
	}

	if err := contracttest.MatchBody(
		request,
		nil,
	); err != nil {
		t.Fatal(err)
	}

	if values := request.Header.Values(
		"Content-Type",
	); len(values) != 0 {
		t.Fatalf(
			"unexpected Content-Type header: %v",
			values,
		)
	}
}

func TestStatusRejectsInvalidValue(t *testing.T) {
	server := contracttest.NewServer(
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.Header().Set(
					"Content-Type",
					"application/json",
				)

				_, _ = writer.Write(
					[]byte(
						`{"value":{"ready":true,"message":"ready","build":{}}}`,
					),
				)
			},
		),
	)
	defer server.Close()

	client, err := server.NewClient(
		appium.ClientOptions{},
	)
	if err != nil {
		t.Fatalf(
			"create client: %v",
			err,
		)
	}

	_, err = client.Status(
		context.Background(),
	)
	if err == nil {
		t.Fatal(
			"expected invalid response error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeResponseInvalid,
	) {
		t.Fatalf(
			"unexpected error code: %v",
			err,
		)
	}

	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryAcknowledged {
		t.Fatalf(
			"unexpected delivery state: expected %q, got %q",
			appium.DeliveryAcknowledged,
			delivery,
		)
	}
}
