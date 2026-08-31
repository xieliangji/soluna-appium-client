package soluna_appium_client_test

import (
	"errors"
	"testing"

	appium "github.com/xieliangji/soluna-appium-client"
)

func TestIsErrorCodeFindsCodesAcrossJoinedErrors(t *testing.T) {
	primary := &appium.Error{
		Code:     appium.CodeCanceled,
		Delivery: appium.DeliveryUnknown,
	}
	diagnostic := &appium.Error{
		Code:     appium.CodeElementNotFound,
		Delivery: appium.DeliveryAcknowledged,
	}
	err := errors.Join(primary, diagnostic)

	if !appium.IsErrorCode(err, appium.CodeCanceled) {
		t.Fatalf("IsErrorCode() did not find primary code: %v", err)
	}
	if !appium.IsErrorCode(err, appium.CodeElementNotFound) {
		t.Fatalf("IsErrorCode() did not find diagnostic code: %v", err)
	}
	if got := appium.DeliveryOf(err); got != appium.DeliveryUnknown {
		t.Fatalf("DeliveryOf() = %q, want primary delivery %q", got, appium.DeliveryUnknown)
	}
}
