package appium

// LocatorStrategy 表示 WebDriver/Appium 元素定位策略。
type LocatorStrategy string

const (
	StrategyID                 LocatorStrategy = "id"                    // 根据资源 ID 定位元素
	StrategyXPath              LocatorStrategy = "xpath"                 // 根据 XPath 表达式定位元素
	StrategyClassName          LocatorStrategy = "class name"            // 根据元素类名定位元素
	StrategyAccessibilityID    LocatorStrategy = "accessibility id"      // 根据 Accessibility ID 定位元素
	StrategyIOSPredicate       LocatorStrategy = "-ios predicate string" // 根据 iOS Predicate 表达式定位元素
	StrategyIOSClassChain      LocatorStrategy = "-ios class chain"      // 根据 iOS Class Chain 表达式定位元素
	StrategyAndroidUIAutomator LocatorStrategy = "-android uiautomator"  // 根据 Android UIAutomator 表达式定位元素
	StrategyImage              LocatorStrategy = "-image"                // 根据图片内容定位元素
)

// Locator 表示一次元素定位所需的策略和值。
//
// Strategy 必须使用 Appium 当前支持的实际协议值。
// 客户端不会对旧名称、别名或非标准写法进行自动转换。
type Locator struct {
	Strategy LocatorStrategy
	Value    string
}

// ID 创建资源 ID 定位器。
func ID(value string) Locator {
	return Locator{
		Strategy: StrategyID,
		Value:    value,
	}
}

// XPath 创建 XPath 定位器。
func XPath(value string) Locator {
	return Locator{
		Strategy: StrategyXPath,
		Value:    value,
	}
}

// ClassName 创建类名定位器。
func ClassName(value string) Locator {
	return Locator{
		Strategy: StrategyClassName,
		Value:    value,
	}
}

// AccessibilityID 创建 Accessibility ID 定位器。
func AccessibilityID(value string) Locator {
	return Locator{
		Strategy: StrategyAccessibilityID,
		Value:    value,
	}
}

// IOSPredicate 创建 iOS Predicate 定位器。
func IOSPredicate(value string) Locator {
	return Locator{
		Strategy: StrategyIOSPredicate,
		Value:    value,
	}
}

// IOSClassChain 创建 iOS Class Chain 定位器。
func IOSClassChain(value string) Locator {
	return Locator{
		Strategy: StrategyIOSClassChain,
		Value:    value,
	}
}

// AndroidUIAutomator 创建 Android UIAutomator 定位器。
func AndroidUIAutomator(value string) Locator {
	return Locator{
		Strategy: StrategyAndroidUIAutomator,
		Value:    value,
	}
}

// Image 创建图片定位器。
func Image(value string) Locator {
	return Locator{
		Strategy: StrategyImage,
		Value:    value,
	}
}
