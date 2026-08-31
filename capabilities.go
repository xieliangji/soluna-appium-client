package appium

// Capabilities 表示一组 W3C WebDriver/Appium Capability。
//
// Capability 名称和值保持开放，以支持 Appium Driver、Plugin 以及后续扩展。
// 客户端不会维护 Capability 白名单，也不会自动修改 Capability 名称。
type Capabilities map[string]any

// W3CCapabilities 表示创建 WebDriver Session 时使用的 W3C Capabilities 结构。
//
// AlwaysMatch 中的 Capability 必须全部满足。
// FirstMatch 中的每一项表示一个可选 Capability 组合，由远端选择首个可满足的组合。
type W3CCapabilities struct {
	AlwaysMatch Capabilities   `json:"alwaysMatch,omitempty"`
	FirstMatch  []Capabilities `json:"firstMatch,omitempty"`
}

// MatchCapabilities 创建只包含 alwaysMatch 的 W3C Capabilities。
//
// 移动端 Appium Session 通常只需要明确的一组 alwaysMatch Capability。
// 需要 firstMatch 时应直接构造 W3CCapabilities。
func MatchCapabilities(capabilities Capabilities) W3CCapabilities {
	return W3CCapabilities{
		AlwaysMatch: capabilities,
	}
}
