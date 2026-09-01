package appium_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/contracttest"
)

func TestResponseTooLargeIsAcknowledged(t *testing.T) {
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
						`{"value":{"ready":true,"message":"this response deliberately exceeds the configured response limit","build":{"version":"3.0.0"}}}`,
					),
				)
			},
		),
	)
	defer server.Close()

	client, err := server.NewClient(
		appium.ClientOptions{
			Limits: appium.Limits{
				MaxResponseBytes: 32,
			},
		},
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
			"expected response too large error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeResponseTooLarge,
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

	if clientErr.StatusCode != http.StatusOK {
		t.Fatalf(
			"unexpected HTTP status: expected %d, got %d",
			http.StatusOK,
			clientErr.StatusCode,
		)
	}
}

func TestTransportFailureAfterRequestWriteHasUnknownDelivery(t *testing.T) {
	hijackResult := make(
		chan error,
		1,
	)

	server := contracttest.NewServer(
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				controller := http.NewResponseController(
					writer,
				)

				connection, _, err := controller.Hijack()
				if err != nil {
					hijackResult <- err
					return
				}

				hijackResult <- nil

				// 请求已经到达 Server，但在发送任何 HTTP Response
				// 之前直接断开连接。
				_ = connection.Close()
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

	if hijackErr := <-hijackResult; hijackErr != nil {
		t.Fatalf(
			"hijack connection: %v",
			hijackErr,
		)
	}

	if err == nil {
		t.Fatal(
			"expected transport failure",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeTransportFailed,
	) {
		t.Fatalf(
			"unexpected error code: %v",
			err,
		)
	}

	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryUnknown {
		t.Fatalf(
			"unexpected delivery state: expected %q, got %q",
			appium.DeliveryUnknown,
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

	if clientErr.StatusCode != 0 {
		t.Fatalf(
			"HTTP status must be unknown before response headers: got %d",
			clientErr.StatusCode,
		)
	}
}

func TestCanceledContextBeforeDelivery(t *testing.T) {
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

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err = client.Status(ctx)
	if err == nil {
		t.Fatal(
			"expected canceled context error",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeCanceled,
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

	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf(
			"canceled command must not be delivered: got %d requests",
			len(requests),
		)
	}
}

func TestReadyProbeTimeoutAfterRequestWriteHasUnknownDelivery(t *testing.T) {
	requestStarted := make(
		chan struct{},
		1,
	)

	server := contracttest.NewServer(
		http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				requestStarted <- struct{}{}

				// 不发送 Response Header。
				// 等待客户端自身的 ReadyProbeTimeout 取消请求。
				<-request.Context().Done()
			},
		),
	)
	defer server.Close()

	client, err := server.NewClient(
		appium.ClientOptions{
			ReadyProbeTimeout: 100 * time.Millisecond,
		},
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
			"expected ready probe timeout",
		)
	}

	select {
	case <-requestStarted:
	default:
		t.Fatal(
			"ready probe timed out before reaching server",
		)
	}

	if !appium.IsErrorCode(
		err,
		appium.CodeDeadlineExceeded,
	) {
		t.Fatalf(
			"unexpected error code: %v",
			err,
		)
	}

	if delivery := appium.DeliveryOf(err); delivery != appium.DeliveryUnknown {
		t.Fatalf(
			"unexpected delivery state: expected %q, got %q",
			appium.DeliveryUnknown,
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

	if clientErr.StatusCode != 0 {
		t.Fatalf(
			"HTTP status must be unknown before response headers: got %d",
			clientErr.StatusCode,
		)
	}
}
