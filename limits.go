package appium

const (
	defaultMaxResponseBytes           int64 = 16 << 20  // 普通命令响应默认上限：16 MiB
	defaultMaxPageSourceResponseBytes int64 = 96 << 20  // Page Source 响应默认上限：96 MiB
	defaultMaxScreenshotResponseBytes int64 = 64 << 20  // Session/Element Screenshot 响应默认上限：64 MiB
	defaultMaxRecordingResponseBytes  int64 = 256 << 20 // 录屏响应默认上限：256 MiB
	defaultMaxLogResponseBytes        int64 = 32 << 20  // Pull Logs 响应默认上限：32 MiB
	defaultMaxRemoteErrorBytes        int64 = 64 << 10  // 远端错误数据默认上限：64 KiB
)

// Limits 定义客户端处理远端数据时使用的资源边界。
//
// 所有字段都以字节为单位。
// 字段为零时使用客户端定义的默认值，负数属于无效配置。
type Limits struct {
	// MaxResponseBytes 表示普通 WebDriver 命令允许读取的最大响应大小。
	MaxResponseBytes int64

	// MaxPageSourceResponseBytes 表示 Page Source 命令允许读取的最大响应大小。
	MaxPageSourceResponseBytes int64

	// MaxScreenshotResponseBytes 表示 Session 或 Element Screenshot 命令允许
	// 读取的最大响应大小，同时限制解码后的截图数据大小。
	MaxScreenshotResponseBytes int64

	// MaxRecordingResponseBytes 表示停止录屏时允许读取的最大响应大小。
	MaxRecordingResponseBytes int64

	// MaxLogResponseBytes 表示 Session Pull Logs 命令允许读取的最大响应大小。
	MaxLogResponseBytes int64

	// MaxRemoteErrorBytes 表示公开 Error 中允许保留的最大远端错误数据大小。
	MaxRemoteErrorBytes int64
}

// DefaultLimits 返回客户端默认使用的资源边界。
func DefaultLimits() Limits {
	return Limits{
		MaxResponseBytes:           defaultMaxResponseBytes,
		MaxPageSourceResponseBytes: defaultMaxPageSourceResponseBytes,
		MaxScreenshotResponseBytes: defaultMaxScreenshotResponseBytes,
		MaxRecordingResponseBytes:  defaultMaxRecordingResponseBytes,
		MaxLogResponseBytes:        defaultMaxLogResponseBytes,
		MaxRemoteErrorBytes:        defaultMaxRemoteErrorBytes,
	}
}
