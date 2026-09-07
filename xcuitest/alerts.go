package xcuitest

import (
	"context"
	"encoding/json"
	"errors"
	"unicode/utf8"

	appium "github.com/xieliangji/soluna-appium-client"
)

const (
	iosAcceptAlertWithLabelOperation  = "ios_accept_alert_with_label"
	iosDismissAlertWithLabelOperation = "ios_dismiss_alert_with_label"
	iosAlertScript                    = "mobile: alert"
)

type alertLabelRequest struct {
	Action      string `json:"action"`
	ButtonLabel string `json:"buttonLabel"`
}

// IOSAcceptAlertWithLabel 请求 XCUITest 通过指定按钮 label 接受当前 Alert。
//
// label 必须非空且为有效 UTF-8，其余内容原样发送，不裁剪空白或改变大小写。
// nil、未初始化或仅供清理的 Session 在本地拒绝；Driver 必须为远端确认的
// XCUITest。按钮匹配与点击由 WDA 决定，成功不保证按钮对应应用的肯定操作。
// 客户端只发送一次请求，不探测、等待、重试或回退到默认按钮。
// 不指定 label 时应使用根包 Session.AcceptAlert。
func IOSAcceptAlertWithLabel(ctx context.Context, session *appium.Session, label string) error {
	return executeAlertWithLabel(ctx, session, iosAcceptAlertWithLabelOperation, "accept", label)
}

// IOSDismissAlertWithLabel 请求 XCUITest 通过指定按钮 label 关闭当前 Alert。
//
// label 必须非空且为有效 UTF-8，其余内容原样发送，不裁剪空白或改变大小写。
// nil、未初始化或仅供清理的 Session 在本地拒绝；Driver 必须为远端确认的
// XCUITest。按钮匹配与点击由 WDA 决定，成功不保证按钮对应应用的取消操作。
// 客户端只发送一次请求，不探测、等待、重试或回退到默认按钮。
// 不指定 label 时应使用根包 Session.DismissAlert。
func IOSDismissAlertWithLabel(ctx context.Context, session *appium.Session, label string) error {
	return executeAlertWithLabel(ctx, session, iosDismissAlertWithLabelOperation, "dismiss", label)
}

func executeAlertWithLabel(
	ctx context.Context,
	session *appium.Session,
	operation string,
	action string,
	label string,
) error {
	if err := requireXCUITestSession(session, operation); err != nil {
		return err
	}
	if label == "" || !utf8.ValidString(label) {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: operation,
			Message:   "alert button label must be non-empty valid UTF-8",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	return session.ExecuteScriptWithOperationAndDecode(
		ctx,
		operation,
		iosAlertScript,
		[]any{alertLabelRequest{Action: action, ButtonLabel: label}},
		decodeAlertLabelNull,
	)
}

func decodeAlertLabelNull(ctx context.Context, value json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !isJSONNull(value) {
		return errors.New("alert label response value must be null")
	}
	return nil
}
