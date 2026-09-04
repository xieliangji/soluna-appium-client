package appium

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
)

const (
	getActiveAppIDOperation = "get_active_app_id"

	xcuiTestActiveAppInfoScript      = "mobile: activeAppInfo"
	uiAutomator2CurrentPackageScript = "mobile: getCurrentPackage"
	xcuiTestAutomationName           = "XCUITest"
	uiAutomator2AutomationName       = "UiAutomator2"
)

// ActiveAppID 读取当前 Driver 报告的前台应用标识快照。
//
// XCUITest Session 返回当前 iOS 应用的 bundle ID；UiAutomator2 Session
// 返回当前 Android 应用的 package。UiAutomator2 明确报告没有
// focused package 时返回空字符串和 nil error。每次调用都会读取远端，
// 客户端不从 Capabilities 推断、不缓存结果，也不枚举进程或已安装应用。
func (s *Session) ActiveAppID(ctx context.Context) (string, error) {
	client, err := s.commandClient(getActiveAppIDOperation)
	if err != nil {
		return "", err
	}

	var (
		script  string
		decoder func(context.Context, json.RawMessage) (string, error)
	)

	switch s.automationName {
	case xcuiTestAutomationName:
		script = xcuiTestActiveAppInfoScript
		decoder = decodeXCUITestActiveAppID

	case uiAutomator2AutomationName:
		script = uiAutomator2CurrentPackageScript
		decoder = decodeUiAutomator2ActiveAppID

	case "":
		return "", &Error{
			Code:      CodeInvalidArgument,
			Operation: getActiveAppIDOperation,
			Message:   "session is not usable for active app ID",
			Delivery:  DeliveryNotSent,
		}

	default:
		return "", &Error{
			Code:      CodeUnsupported,
			Operation: getActiveAppIDOperation,
			Message: fmt.Sprintf(
				"active app ID is unsupported for automationName %q",
				s.automationName,
			),
			Delivery: DeliveryNotSent,
		}
	}

	var appID string
	err = executeScriptCommand(
		ctx,
		client,
		getActiveAppIDOperation,
		s.id,
		script,
		nil,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decoder(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}

			appID = decoded
			return nil
		},
	)
	if err != nil {
		return "", err
	}

	return appID, nil
}

// decodeXCUITestActiveAppID 从 activeAppInfo 中严格读取 bundleId。
func decodeXCUITestActiveAppID(
	ctx context.Context,
	value json.RawMessage,
) (string, error) {
	if ctx == nil {
		return "", errors.New("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(value, &payload); err != nil {
		return "", fmt.Errorf("decode XCUITest active app info: %w", err)
	}

	bundleIDValue, ok := payload["bundleId"]
	if !ok {
		return "", errors.New(
			"XCUITest active app info does not contain bundleId",
		)
	}

	bundleID, err := codec.DecodeJSONString(ctx, bundleIDValue)
	if err != nil {
		return "", fmt.Errorf(
			"decode XCUITest active app bundleId: %w",
			err,
		)
	}
	if bundleID == "" {
		return "", errors.New(
			"XCUITest active app bundleId must not be empty",
		)
	}

	return bundleID, nil
}

// decodeUiAutomator2ActiveAppID 严格读取 getCurrentPackage 返回的 package。
func decodeUiAutomator2ActiveAppID(
	ctx context.Context,
	value json.RawMessage,
) (string, error) {
	if ctx == nil {
		return "", errors.New("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// Android 在当前没有 focused package 时明确返回 null。
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return "", nil
	}

	packageName, err := codec.DecodeJSONString(ctx, value)
	if err != nil {
		return "", fmt.Errorf(
			"decode UiAutomator2 current package: %w",
			err,
		)
	}
	if packageName == "" {
		return "", errors.New(
			"UiAutomator2 current package must not be empty",
		)
	}

	return packageName, nil
}
