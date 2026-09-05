package appium

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	getDeviceTimeOperation = "get_device_time"
	deviceTimeLayout       = "2006-01-02T15:04:05-07:00"
	deviceTimeLength       = len("2006-01-02T15:04:05+07:00")
)

// DeviceTime 读取当前 Driver 报告的设备时间快照。
//
// 返回值保留远端报告的数字 UTC 偏移，精度为秒。成功响应
// 必须精确符合 YYYY-MM-DDTHH:mm:ss±HH:MM；客户端不猜测其他格式、
// 不缓存或校正时钟，也不会在远端失败时返回 Host 当前时间。
//
// XCUITest Driver 在 Simulator 上可能以 Appium Host 时钟作为设备时间；
// 该行为属于远端 Driver，无法从本命令的响应中区分。
func (s *Session) DeviceTime(ctx context.Context) (time.Time, error) {
	client, err := s.commandClient(getDeviceTimeOperation)
	if err != nil {
		return time.Time{}, err
	}

	command, err := wire.NewCommand(
		getDeviceTimeOperation,
		http.MethodGet,
		"session",
		s.id,
		"appium",
		"device",
		"system_time",
	)
	if err != nil {
		return time.Time{}, commandDefinitionError(
			getDeviceTimeOperation,
			"get device time command definition is invalid",
			err,
		)
	}

	var deviceTime time.Time
	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decodeDeviceTime(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			deviceTime = decoded
			return nil
		},
	)
	if err != nil {
		return time.Time{}, err
	}

	return deviceTime, nil
}

// decodeDeviceTime 严格解码 Appium Device Time 命令的默认时间格式。
func decodeDeviceTime(
	ctx context.Context,
	value json.RawMessage,
) (time.Time, error) {
	if ctx == nil {
		return time.Time{}, errors.New("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return time.Time{}, err
	}

	encoded, err := codec.DecodeJSONString(ctx, value)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"decode device time response value: %w",
			err,
		)
	}
	if !hasDeviceTimeShape(encoded) {
		return time.Time{}, errors.New(
			"device time response value must match YYYY-MM-DDTHH:mm:ss±HH:MM",
		)
	}

	decoded, err := time.ParseInLocation(
		deviceTimeLayout,
		encoded,
		time.UTC,
	)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"parse device time response value: %w",
			err,
		)
	}

	// ParseInLocation intentionally avoids time.Local. Round-tripping additionally
	// rejects accepted-but-noncanonical forms such as a negative zero offset.
	if decoded.Format(deviceTimeLayout) != encoded {
		return time.Time{}, errors.New(
			"device time response value is not canonical",
		)
	}

	if err := ctx.Err(); err != nil {
		return time.Time{}, err
	}

	return decoded, nil
}

// hasDeviceTimeShape 校验 Driver 默认格式的精确宽度、分隔符和 UTC 偏移。
func hasDeviceTimeShape(value string) bool {
	if len(value) != deviceTimeLength {
		return false
	}

	for index := 0; index < len(value); index++ {
		switch index {
		case 4, 7:
			if value[index] != '-' {
				return false
			}

		case 10:
			if value[index] != 'T' {
				return false
			}

		case 13, 16, 22:
			if value[index] != ':' {
				return false
			}

		case 19:
			if value[index] != '+' && value[index] != '-' {
				return false
			}

		default:
			if value[index] < '0' || value[index] > '9' {
				return false
			}
		}
	}

	offsetHour := int(value[20]-'0')*10 + int(value[21]-'0')
	offsetMinute := int(value[23]-'0')*10 + int(value[24]-'0')

	return offsetHour < 24 && offsetMinute < 60
}
