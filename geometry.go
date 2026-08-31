package soluna_appium_client

// Point 表示 WebDriver viewport 坐标系中的一个点。
//
// 该类型用于点击、长按、滑动等基于 W3C Actions 的指针操作。
// 它不表示截图像素坐标。
type Point struct {
	X int
	Y int
}

// Rect 表示 WebDriver 返回的矩形区域。
//
// X 和 Y 表示矩形左上角坐标，Width 和 Height 表示矩形尺寸。
// 该类型描述的是 WebDriver 坐标语义，不保证与截图像素坐标一一对应。
type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// PixelRect 表示 Driver 报告的整数像素 viewport 几何。
//
// X 和 Y 是 Driver 命令定义的像素平面中的左上角偏移，Width 和 Height
// 是正的像素尺寸。该类型不绑定任何具体 Screenshot，也不表示 WebDriver
// 坐标或截图缓冲区索引。
type PixelRect struct {
	X      int
	Y      int
	Width  int
	Height int
}
