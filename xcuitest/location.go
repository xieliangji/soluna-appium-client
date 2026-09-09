package xcuitest

import (
	"context"
	"encoding/json"
	"errors"
	"math"

	appium "github.com/xieliangji/soluna-appium-client"
)

const (
	iosGetSimulatedLocationOperation   = "ios_get_simulated_location"
	iosSetSimulatedLocationOperation   = "ios_set_simulated_location"
	iosResetSimulatedLocationOperation = "ios_reset_simulated_location"
)

// SimulatedLocation 表示 XCTest 模拟位置的经纬度，单位为度。
// Latitude 必须为有限的 [-90, 90] 数值，Longitude 必须为有限的 [-180, 180] 数值。
// 零值表示有效的 (0, 0) 坐标，不表示未设置；本类型不包含海拔或真实 GPS 读数。
type SimulatedLocation struct {
	Latitude  float64
	Longitude float64
}

type simulatedLocationRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// IOSGetSimulatedLocation 读取远端报告的 XCTest 模拟位置快照。
//
// 未设置或已重置时返回 nil 和 nil error；失败时返回 nil 和 error。
// 每次读取远端，不缓存或回退到真实 GPS。需要远端确认的 XCUITest Session；
// nil、未初始化或仅供清理的 Session 返回参数错误。
// 上游要求 XCUITest Driver 4.18+、Xcode 14.3+ 和 iOS 16.4+；实际 Driver/WDA
// 与 Host 组合见兼容性文档，SDK 不探测或推断版本支持。
func IOSGetSimulatedLocation(ctx context.Context, session *appium.Session) (*SimulatedLocation, error) {
	if err := requireXCUITestSession(session, iosGetSimulatedLocationOperation); err != nil {
		return nil, err
	}

	var location *SimulatedLocation
	err := session.ExecuteScriptWithOperationAndDecode(
		ctx,
		iosGetSimulatedLocationOperation,
		"mobile: getSimulatedLocation",
		nil,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, err := decodeSimulatedLocation(ctx, value)
			if err != nil {
				return err
			}
			location = decoded
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return location, nil
}

// IOSSetSimulatedLocation 请求设置 XCTest 模拟位置。
//
// location 必须满足 SimulatedLocation 的数值范围，(0, 0) 会显式发送。
// Session 和版本条件与 IOSGetSimulatedLocation 相同。成功只表示本次远端
// 命令成功，不保证应用已经观察到位置变化；SDK 不自动读取确认、重试或恢复。
// 模拟位置可能持续到设备重启，调用方应在测试结束后显式调用
// IOSResetSimulatedLocation；Session.Close 不负责清除位置。
func IOSSetSimulatedLocation(ctx context.Context, session *appium.Session, location SimulatedLocation) error {
	if err := requireXCUITestSession(session, iosSetSimulatedLocationOperation); err != nil {
		return err
	}
	if !validSimulatedLocation(location) {
		return &appium.Error{
			Code:      appium.CodeInvalidArgument,
			Operation: iosSetSimulatedLocationOperation,
			Message:   "simulated location requires finite latitude in [-90, 90] and longitude in [-180, 180]",
			Delivery:  appium.DeliveryNotSent,
		}
	}

	return session.ExecuteScriptWithOperationAndDecode(
		ctx,
		iosSetSimulatedLocationOperation,
		"mobile: setSimulatedLocation",
		[]any{simulatedLocationRequest{Latitude: location.Latitude, Longitude: location.Longitude}},
		decodeSimulatedLocationNull,
	)
}

// IOSResetSimulatedLocation 请求清除此前设置的 XCTest 模拟位置。
//
// Session 和版本条件与 IOSGetSimulatedLocation 相同。只发送一次请求，不自动
// 确认后续位置或重试；成功不保证应用立即取得真实 GPS 读数，也不重置定位权限。
func IOSResetSimulatedLocation(ctx context.Context, session *appium.Session) error {
	if err := requireXCUITestSession(session, iosResetSimulatedLocationOperation); err != nil {
		return err
	}
	return session.ExecuteScriptWithOperationAndDecode(
		ctx,
		iosResetSimulatedLocationOperation,
		"mobile: resetSimulatedLocation",
		nil,
		decodeSimulatedLocationNull,
	)
}

func validSimulatedLocation(location SimulatedLocation) bool {
	return !math.IsNaN(location.Latitude) && !math.IsInf(location.Latitude, 0) &&
		location.Latitude >= -90 && location.Latitude <= 90 &&
		!math.IsNaN(location.Longitude) && !math.IsInf(location.Longitude, 0) &&
		location.Longitude >= -180 && location.Longitude <= 180
}

func decodeSimulatedLocation(ctx context.Context, value json.RawMessage) (*SimulatedLocation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// 精确匹配字段名，避免 struct 解码接受 Latitude 等大小写变体。
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(value, &payload); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	latitude, hasLatitude := payload["latitude"]
	longitude, hasLongitude := payload["longitude"]
	if !hasLatitude || !hasLongitude {
		return nil, errors.New("simulated location response must contain latitude and longitude")
	}
	latitudeNull, longitudeNull := isJSONNull(latitude), isJSONNull(longitude)
	if latitudeNull && longitudeNull {
		return nil, nil
	}
	if latitudeNull || longitudeNull {
		return nil, errors.New("simulated location coordinates must both be numbers or both be null")
	}
	var location SimulatedLocation
	if err := json.Unmarshal(latitude, &location.Latitude); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(longitude, &location.Longitude); err != nil {
		return nil, err
	}
	if !validSimulatedLocation(location) {
		return nil, errors.New("simulated location response coordinates are outside the valid range")
	}
	return &location, nil
}

func decodeSimulatedLocationNull(ctx context.Context, value json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !isJSONNull(value) {
		return errors.New("simulated location command response value must be null")
	}
	return nil
}
