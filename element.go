package appium

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	findElementOperation             = "find_element"
	findElementsOperation            = "find_elements"
	findElementFromElementOperation  = "find_element_from_element"
	findElementsFromElementOperation = "find_elements_from_element"

	getElementRectOperation      = "get_element_rect"
	getElementTextOperation      = "get_element_text"
	getElementAttributeOperation = "get_element_attribute"
	tapElementOperation          = "tap_element"
	clearElementOperation        = "clear_element"
	sendKeysOperation            = "send_keys"
)

// Element 表示绑定到某个 Session 的远端 WebDriver 元素引用。
//
// Element 只保存远端元素 ID，不缓存 Locator、文本、属性或坐标。
// 元素失效后客户端不会自动重新定位。
type Element struct {
	session *Session
	id      string
}

// Find 查找当前 Context viewport 中第一个匹配 Locator 的元素。
//
// 方法先读取当前 Context。NATIVE_APP 使用 Window Rect；已识别的 Web Context
// 使用 CSS layout viewport，并按当前滚动偏移平移文档相对 Element Rect。
// 底层会获取结构树中全部匹配元素，并按远端顺序返回第一个存在正面积交集的结果。
// 未识别的 Context 不执行候选查找或几何 fallback。
//
// Locator Strategy 会按照调用方提供的协议值原样发送，
// 客户端不会执行别名转换或自动规范化。
func (s *Session) Find(
	ctx context.Context,
	locator Locator,
) (*Element, error) {
	if _, err := s.commandClient(findElementOperation); err != nil {
		return nil, err
	}
	if err := validateLocator(findElementOperation, locator); err != nil {
		return nil, err
	}

	kind, err := s.contextKindForGeometry(ctx, findElementOperation)
	if err != nil {
		return nil, err
	}

	candidates, err := s.findElementCandidates(
		ctx,
		findElementOperation,
		locator,
	)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, elementNotFoundInContextViewportError(
			findElementOperation,
			kind,
		)
	}

	geometry, err := s.geometryForContext(ctx, kind, findElementOperation)
	if err != nil {
		return nil, err
	}

	for _, element := range candidates {
		rect, err := geometry.elementRect(ctx, element)
		if err != nil {
			return nil, err
		}

		if _, ok := intersectRects(
			rect,
			geometry.viewport,
		); ok {
			return element, nil
		}
	}

	return nil, elementNotFoundInContextViewportError(
		findElementOperation,
		kind,
	)
}

// FindElements 查找当前 Context viewport 中全部匹配 Locator 的元素。
//
// NATIVE_APP 使用 Window Rect；已识别的 Web Context 使用 CSS layout viewport，
// 并按当前滚动偏移平移文档相对 Element Rect。结果保持远端顺序，只包含存在
// 正面积交集的元素。未识别的 Context 不执行候选查找或几何 fallback。
//
// Locator Strategy 会按照调用方提供的协议值原样发送，
// 客户端不会执行别名转换或自动规范化。
//
// 没有有效元素时返回非 nil 空 slice 和 nil error。
func (s *Session) FindElements(
	ctx context.Context,
	locator Locator,
) ([]*Element, error) {
	if _, err := s.commandClient(findElementsOperation); err != nil {
		return nil, err
	}
	if err := validateLocator(findElementsOperation, locator); err != nil {
		return nil, err
	}

	kind, err := s.contextKindForGeometry(ctx, findElementsOperation)
	if err != nil {
		return nil, err
	}

	candidates, err := s.findElementCandidates(
		ctx,
		findElementsOperation,
		locator,
	)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return candidates, nil
	}

	geometry, err := s.geometryForContext(ctx, kind, findElementsOperation)
	if err != nil {
		return nil, err
	}

	elements := make(
		[]*Element,
		0,
		len(candidates),
	)

	for _, element := range candidates {
		rect, err := geometry.elementRect(ctx, element)
		if err != nil {
			return nil, err
		}

		if _, ok := intersectRects(
			rect,
			geometry.viewport,
		); ok {
			elements = append(
				elements,
				element,
			)
		}
	}

	return elements, nil
}

// findElementCandidates 获取结构树中全部匹配 Locator 的元素候选。
//
// 本方法只负责 WebDriver 元素查找和引用解码，
// 不判断元素是否位于当前 Window 内。
func (s *Session) findElementCandidates(
	ctx context.Context,
	operation string,
	locator Locator,
) ([]*Element, error) {
	client, err := s.commandClient(operation)
	if err != nil {
		return nil, err
	}

	if err := validateLocator(operation, locator); err != nil {
		return nil, err
	}

	command, err := wire.NewCommand(
		operation,
		http.MethodPost,
		"session",
		s.id,
		"elements",
	)
	if err != nil {
		return nil, commandDefinitionError(
			operation,
			"find element candidates command definition is invalid",
			err,
		)
	}

	request := struct {
		Using string `json:"using"`
		Value string `json:"value"`
	}{
		Using: string(locator.Strategy),
		Value: locator.Value,
	}

	var elementIDs []string

	err = client.executeCommand(
		ctx,
		command,
		request,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			ids, decodeErr := decodeElementReferences(
				ctx,
				value,
			)
			if decodeErr != nil {
				return decodeErr
			}

			elementIDs = ids
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	elements := make(
		[]*Element,
		len(elementIDs),
	)

	for index, elementID := range elementIDs {
		elements[index] = &Element{
			session: s,
			id:      elementID,
		}
	}

	return elements, nil
}

// elementNotFoundInContextViewportError 表示没有匹配元素处于当前几何范围内。
//
// 到达该结果前，候选查询已经获得远端响应，
// 因此 Delivery 为 Acknowledged。
func elementNotFoundInContextViewportError(
	operation string,
	kind applicationContextKind,
) error {
	message := "no matching element intersects current window"
	if kind == applicationContextWeb {
		message = "no matching element intersects current web viewport"
	}

	return &Error{
		Code:      CodeElementNotFound,
		Operation: operation,
		Message:   message,
		Delivery:  DeliveryAcknowledged,
	}
}

// ID 返回远端 WebDriver Element ID。
func (e *Element) ID() string {
	if e == nil {
		return ""
	}
	return e.id
}

// Find 在当前元素的后代中查找第一个位于当前 Context viewport 内的匹配元素。
//
// 底层使用 W3C Find Elements From Element 获取全部结构匹配候选，
// 再按照远端返回顺序检查候选 Rect。NATIVE_APP 使用 Window Rect；已识别的
// Web Context 使用按滚动偏移平移后的 CSS viewport Rect。
//
// Locator Strategy 会按照调用方提供的协议值原样发送，
// 客户端不会执行别名转换或自动规范化。
func (e *Element) Find(
	ctx context.Context,
	locator Locator,
) (*Element, error) {
	session, _, err := e.commandContext(findElementFromElementOperation)
	if err != nil {
		return nil, err
	}
	if err := validateLocator(findElementFromElementOperation, locator); err != nil {
		return nil, err
	}

	kind, err := session.contextKindForGeometry(
		ctx,
		findElementFromElementOperation,
	)
	if err != nil {
		return nil, err
	}

	candidates, err := e.findElementCandidates(
		ctx,
		findElementFromElementOperation,
		locator,
	)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, elementNotFoundInContextViewportError(
			findElementFromElementOperation,
			kind,
		)
	}

	geometry, err := session.geometryForContext(
		ctx,
		kind,
		findElementFromElementOperation,
	)
	if err != nil {
		return nil, err
	}

	for _, candidate := range candidates {
		rect, err := geometry.elementRect(ctx, candidate)
		if err != nil {
			return nil, err
		}

		if _, ok := intersectRects(
			rect,
			geometry.viewport,
		); ok {
			return candidate, nil
		}
	}

	return nil, elementNotFoundInContextViewportError(
		findElementFromElementOperation,
		kind,
	)
}

// FindElements 在当前元素后代中查找全部位于当前 Context viewport 内的匹配元素。
//
// 底层使用 W3C Find Elements From Element 获取全部结构匹配候选，
// 并保持远端返回顺序。NATIVE_APP 使用 Window Rect；已识别的 Web Context
// 使用按滚动偏移平移后的 CSS viewport Rect。
//
// 没有有效元素时返回非 nil 空 slice 和 nil error。
func (e *Element) FindElements(
	ctx context.Context,
	locator Locator,
) ([]*Element, error) {
	session, _, err := e.commandContext(findElementsFromElementOperation)
	if err != nil {
		return nil, err
	}
	if err := validateLocator(findElementsFromElementOperation, locator); err != nil {
		return nil, err
	}

	kind, err := session.contextKindForGeometry(
		ctx,
		findElementsFromElementOperation,
	)
	if err != nil {
		return nil, err
	}

	candidates, err := e.findElementCandidates(
		ctx,
		findElementsFromElementOperation,
		locator,
	)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return candidates, nil
	}

	geometry, err := session.geometryForContext(
		ctx,
		kind,
		findElementsFromElementOperation,
	)
	if err != nil {
		return nil, err
	}

	elements := make(
		[]*Element,
		0,
		len(candidates),
	)

	for _, candidate := range candidates {
		rect, err := geometry.elementRect(ctx, candidate)
		if err != nil {
			return nil, err
		}

		if _, ok := intersectRects(
			rect,
			geometry.viewport,
		); ok {
			elements = append(
				elements,
				candidate,
			)
		}
	}

	return elements, nil
}

// findElementCandidates 获取当前元素后代中的全部结构匹配候选。
//
// 本方法只负责 W3C Find Elements From Element 和元素引用解码，
// 不判断候选是否位于当前 Window 内。
func (e *Element) findElementCandidates(
	ctx context.Context,
	operation string,
	locator Locator,
) ([]*Element, error) {
	session, client, err := e.commandContext(operation)
	if err != nil {
		return nil, err
	}

	if err := validateLocator(operation, locator); err != nil {
		return nil, err
	}

	command, err := wire.NewCommand(
		operation,
		http.MethodPost,
		"session",
		session.id,
		"element",
		e.id,
		"elements",
	)
	if err != nil {
		return nil, commandDefinitionError(
			operation,
			"find elements from element command definition is invalid",
			err,
		)
	}

	request := struct {
		Using string `json:"using"`
		Value string `json:"value"`
	}{
		Using: string(locator.Strategy),
		Value: locator.Value,
	}

	var elementIDs []string

	err = client.executeCommand(
		ctx,
		command,
		request,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			ids, decodeErr := decodeElementReferences(
				ctx,
				value,
			)
			if decodeErr != nil {
				return decodeErr
			}

			elementIDs = ids
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	elements := make(
		[]*Element,
		len(elementIDs),
	)

	for index, elementID := range elementIDs {
		elements[index] = &Element{
			session: session,
			id:      elementID,
		}
	}

	return elements, nil
}

// Rect 获取元素当前的 WebDriver Rect。
func (e *Element) Rect(ctx context.Context) (Rect, error) {
	return e.rectWithTransform(ctx, nil)
}

// rectWithTransform 在 Element Rect 命令的响应解码阶段应用可选坐标转换。
func (e *Element) rectWithTransform(
	ctx context.Context,
	transform func(Rect) (Rect, error),
) (Rect, error) {
	session, client, err := e.commandContext(
		getElementRectOperation,
	)
	if err != nil {
		return Rect{}, err
	}

	command, err := wire.NewCommand(
		getElementRectOperation,
		http.MethodGet,
		"session",
		session.id,
		"element",
		e.id,
		"rect",
	)
	if err != nil {
		return Rect{}, commandDefinitionError(
			getElementRectOperation,
			"get element rect command definition is invalid",
			err,
		)
	}

	var rect Rect

	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			decoded, decodeErr := decodeRect(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			if transform != nil {
				decoded, decodeErr = transform(decoded)
				if decodeErr != nil {
					return decodeErr
				}
			}

			rect = decoded
			return nil
		},
	)
	if err != nil {
		return Rect{}, err
	}

	return rect, nil
}

// Text 获取元素当前文本。
func (e *Element) Text(ctx context.Context) (string, error) {
	session, client, err := e.commandContext(
		getElementTextOperation,
	)
	if err != nil {
		return "", err
	}

	command, err := wire.NewCommand(
		getElementTextOperation,
		http.MethodGet,
		"session",
		session.id,
		"element",
		e.id,
		"text",
	)
	if err != nil {
		return "", commandDefinitionError(
			getElementTextOperation,
			"get element text command definition is invalid",
			err,
		)
	}

	var text string

	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(
			ctx context.Context,
			value json.RawMessage,
		) error {
			decoded, decodeErr := codec.DecodeJSONString(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}

			text = decoded
			return nil
		},
	)
	if err != nil {
		return "", err
	}

	return text, nil
}

// Attribute 获取元素指定属性。
//
// exists 为 false 表示远端明确返回 null，即该属性不存在。
func (e *Element) Attribute(
	ctx context.Context,
	name string,
) (value string, exists bool, err error) {
	session, client, err := e.commandContext(
		getElementAttributeOperation,
	)
	if err != nil {
		return "", false, err
	}

	if name == "" {
		return "", false, &Error{
			Code:      CodeInvalidArgument,
			Operation: getElementAttributeOperation,
			Message:   "attribute name is empty",
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		getElementAttributeOperation,
		http.MethodGet,
		"session",
		session.id,
		"element",
		e.id,
		"attribute",
		name,
	)
	if err != nil {
		return "", false, commandDefinitionError(
			getElementAttributeOperation,
			"get element attribute command definition is invalid",
			err,
		)
	}

	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(
			ctx context.Context,
			raw json.RawMessage,
		) error {
			decoded, found, decodeErr := decodeOptionalString(
				ctx,
				raw,
			)
			if decodeErr != nil {
				return decodeErr
			}

			value = decoded
			exists = found
			return nil
		},
	)
	if err != nil {
		return "", false, err
	}

	return value, exists, nil
}

// Tap 点击元素当前位于 Context viewport 内的几何交集区域中心。
//
// NATIVE_APP 使用 Window Rect；已识别的 Web Context 使用 CSS layout viewport，
// 并按当前滚动偏移平移文档相对 Element Rect。方法不使用 WebDriver Element Click
// 的 Driver 侧点击位置计算，也不为未知 Context 执行 fallback。
//
// Tap 不保证目标位置没有被其他元素遮挡，也不表示元素视觉上可见或可交互。
func (e *Element) Tap(
	ctx context.Context,
) error {
	return e.TapInWindowIntersection(
		ctx,
		0.5,
		0.5,
	)
}

// TapInWindowIntersection 在元素与当前 Context viewport 的交集内按比例点击。
//
// xRatio 和 yRatio 分别表示交集区域水平方向和垂直方向的位置，
// 有效范围均为 [0, 1]。
// 0 表示最靠近起始边界的可用整数坐标，1 表示最靠近结束边界的可用整数坐标。
//
// NATIVE_APP 的 viewport 是 Window Rect；已识别 Web Context 的 viewport 是
// CSS layout viewport。客户端只发送严格位于正面积交集内部的整数坐标。
//
// 如果两者没有正面积交集，或者交集内不存在可表示的整数 viewport 坐标，
// 本方法不会发送 Tap。
func (e *Element) TapInWindowIntersection(
	ctx context.Context,
	xRatio float64,
	yRatio float64,
) error {
	session, _, err := e.commandContext(
		tapElementOperation,
	)
	if err != nil {
		return err
	}

	if !validTapRatio(xRatio) ||
		!validTapRatio(yRatio) {
		return elementTapArgumentError(
			"tap ratios must be finite values in [0, 1]",
		)
	}

	kind, err := session.contextKindForGeometry(ctx, tapElementOperation)
	if err != nil {
		return err
	}

	// viewport 通常比元素位置稳定，因此先获取 viewport，
	// 再获取更容易随 UI 变化的 Element Rect，
	// 尽量缩短 Element Rect 快照与实际 Tap 之间的时间。
	geometry, err := session.geometryForContext(ctx, kind, tapElementOperation)
	if err != nil {
		return err
	}

	elementRect, err := geometry.elementRect(ctx, e)
	if err != nil {
		return err
	}

	intersection, ok := intersectRects(
		elementRect,
		geometry.viewport,
	)
	if !ok {
		return elementTapArgumentError(
			"element does not intersect current window",
		)
	}

	point, ok := pointInRectByRatio(
		intersection,
		xRatio,
		yRatio,
	)
	if !ok {
		return elementTapArgumentError(
			"element and window intersection does not contain an integer viewport point",
		)
	}

	return session.Tap(
		ctx,
		point,
	)
}

// Clear 清除元素当前输入内容。
func (e *Element) Clear(ctx context.Context) error {
	session, client, err := e.commandContext(
		clearElementOperation,
	)
	if err != nil {
		return err
	}

	command, err := wire.NewCommand(
		clearElementOperation,
		http.MethodPost,
		"session",
		session.id,
		"element",
		e.id,
		"clear",
	)
	if err != nil {
		return commandDefinitionError(
			clearElementOperation,
			"clear element command definition is invalid",
			err,
		)
	}

	return client.executeCommand(
		ctx,
		command,
		struct{}{},
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		decodeNullResponse,
	)
}

// SendKeys 向元素发送文本输入。
//
// value 会原样作为 W3C Element Send Keys 的 text 参数发送。
func (e *Element) SendKeys(
	ctx context.Context,
	value string,
) error {
	session, client, err := e.commandContext(
		sendKeysOperation,
	)
	if err != nil {
		return err
	}

	command, err := wire.NewCommand(
		sendKeysOperation,
		http.MethodPost,
		"session",
		session.id,
		"element",
		e.id,
		"value",
	)
	if err != nil {
		return commandDefinitionError(
			sendKeysOperation,
			"send keys command definition is invalid",
			err,
		)
	}

	request := struct {
		Text string `json:"text"`
	}{
		Text: value,
	}

	return client.executeCommand(
		ctx,
		command,
		request,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		decodeNullResponse,
	)
}

// validTapRatio 判断点击比例是否为有限的 [0, 1] 数值。
func validTapRatio(value float64) bool {
	return !math.IsNaN(value) &&
		!math.IsInf(value, 0) &&
		value >= 0 &&
		value <= 1
}

// intersectRects 返回两个 Rect 的正面积交集。
func intersectRects(
	first Rect,
	second Rect,
) (Rect, bool) {
	left := math.Max(
		first.X,
		second.X,
	)
	top := math.Max(
		first.Y,
		second.Y,
	)
	right := math.Min(
		first.X+first.Width,
		second.X+second.Width,
	)
	bottom := math.Min(
		first.Y+first.Height,
		second.Y+second.Height,
	)

	// 使用否定形式同时拒绝 NaN 和零面积交集。
	if !(right > left) ||
		!(bottom > top) {
		return Rect{}, false
	}

	return Rect{
		X:      left,
		Y:      top,
		Width:  right - left,
		Height: bottom - top,
	}, true
}

// pointInRectByRatio 在 Rect 内部选择一个确定的整数坐标。
//
// Rect 使用连续坐标，而 W3C Actions 当前使用整数 Point。
// 因此先求严格位于 Rect 半开区间 [start, end) 内的整数坐标范围，
// 再把比例位置映射并限制到该范围。
func pointInRectByRatio(
	rect Rect,
	xRatio float64,
	yRatio float64,
) (Point, bool) {
	minX := math.Ceil(rect.X)
	maxX := math.Ceil(rect.X+rect.Width) - 1

	minY := math.Ceil(rect.Y)
	maxY := math.Ceil(rect.Y+rect.Height) - 1

	if minX > maxX ||
		minY > maxY {
		return Point{}, false
	}

	// Point 使用 int，拒绝无法安全表示的异常远端坐标。
	if minX <= float64(math.MinInt) ||
		maxX >= float64(math.MaxInt) ||
		minY <= float64(math.MinInt) ||
		maxY >= float64(math.MaxInt) {
		return Point{}, false
	}

	x := math.Round(
		rect.X + rect.Width*xRatio,
	)
	y := math.Round(
		rect.Y + rect.Height*yRatio,
	)

	x = math.Max(
		minX,
		math.Min(maxX, x),
	)
	y = math.Max(
		minY,
		math.Min(maxY, y),
	)

	return Point{
		X: int(x),
		Y: int(y),
	}, true
}

// elementTapArgumentError 创建元素几何点击参数或前置条件错误。
func elementTapArgumentError(
	message string,
) error {
	return &Error{
		Code:      CodeInvalidArgument,
		Operation: tapElementOperation,
		Message:   message,
		Delivery:  DeliveryNotSent,
	}
}

// validateLocator 校验 Element 查找的本地 Locator 参数。
func validateLocator(
	operation string,
	locator Locator,
) error {
	if locator.Strategy != "" {
		return nil
	}

	return &Error{
		Code:      CodeInvalidArgument,
		Operation: operation,
		Message:   "locator strategy is empty",
		Delivery:  DeliveryNotSent,
	}
}

// commandContext 校验 Element 及其所属 Session 是否可用于远端命令。
func (e *Element) commandContext(
	operation string,
) (*Session, *Client, error) {
	if e == nil ||
		e.session == nil ||
		e.id == "" {
		return nil, nil, &Error{
			Code:      CodeInvalidArgument,
			Operation: operation,
			Message:   "element is not initialized",
			Delivery:  DeliveryNotSent,
		}
	}

	client, err := e.session.commandClient(operation)
	if err != nil {
		return nil, nil, err
	}

	return e.session, client, nil
}

// decodeElementReference 严格解码 W3C Element Reference。
//
// 客户端只接受 W3C element reference key，
// 不使用旧 JSON Wire Protocol 的 ELEMENT 别名。
func decodeElementReference(
	ctx context.Context,
	value json.RawMessage,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var payload struct {
		ElementID json.RawMessage `json:"element-6066-11e4-a52e-4f735466cecf"`
	}

	if err := json.Unmarshal(value, &payload); err != nil {
		return "", fmt.Errorf(
			"decode element reference: %w",
			err,
		)
	}

	if len(payload.ElementID) == 0 {
		return "", errors.New(
			"element response does not contain W3C element reference",
		)
	}

	elementID, err := codec.DecodeJSONString(
		ctx,
		payload.ElementID,
	)
	if err != nil {
		return "", fmt.Errorf(
			"decode element ID: %w",
			err,
		)
	}

	if elementID == "" {
		return "", errors.New(
			"element response contains empty element ID",
		)
	}

	return elementID, nil
}

// decodeElementReferences 严格解码 W3C Element Reference 数组。
//
// 返回值必须是 JSON array。
// 数组中的每个元素都必须使用 W3C element reference key，
// 不接受旧 JSON Wire Protocol 的 ELEMENT 别名。
func decodeElementReferences(
	ctx context.Context,
	value json.RawMessage,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	trimmed := bytes.TrimSpace(value)

	if len(trimmed) == 0 ||
		trimmed[0] != '[' {
		return nil, errors.New(
			"elements response must be a JSON array",
		)
	}

	var references []json.RawMessage

	if err := json.Unmarshal(
		trimmed,
		&references,
	); err != nil {
		return nil, fmt.Errorf(
			"decode element references: %w",
			err,
		)
	}

	elementIDs := make(
		[]string,
		len(references),
	)

	for index, reference := range references {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		elementID, err := decodeElementReference(
			ctx,
			reference,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"decode element reference at index %d: %w",
				index,
				err,
			)
		}

		elementIDs[index] = elementID
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return elementIDs, nil
}

// decodeRect 严格解码元素 Rect。
func decodeRect(
	ctx context.Context,
	value json.RawMessage,
) (Rect, error) {
	if err := ctx.Err(); err != nil {
		return Rect{}, err
	}

	var payload struct {
		X      *float64 `json:"x"`
		Y      *float64 `json:"y"`
		Width  *float64 `json:"width"`
		Height *float64 `json:"height"`
	}

	if err := json.Unmarshal(value, &payload); err != nil {
		return Rect{}, fmt.Errorf(
			"decode WebDriver rect: %w",
			err,
		)
	}

	if payload.X == nil ||
		payload.Y == nil ||
		payload.Width == nil ||
		payload.Height == nil {
		return Rect{}, errors.New(
			"webdriver rect response is incomplete",
		)
	}

	if *payload.Width < 0 || *payload.Height < 0 {
		return Rect{}, errors.New(
			"webdriver rect contains negative size",
		)
	}

	if err := ctx.Err(); err != nil {
		return Rect{}, err
	}

	return Rect{
		X:      *payload.X,
		Y:      *payload.Y,
		Width:  *payload.Width,
		Height: *payload.Height,
	}, nil
}

// decodeOptionalString 解码 string 或 null 类型的响应值。
func decodeOptionalString(
	ctx context.Context,
	value json.RawMessage,
) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}

	if bytes.Equal(
		bytes.TrimSpace(value),
		[]byte("null"),
	) {
		return "", false, nil
	}

	decoded, err := codec.DecodeJSONString(ctx, value)
	if err != nil {
		return "", false, err
	}

	return decoded, true, nil
}

// commandDefinitionError 创建内部命令定义错误。
func commandDefinitionError(
	operation string,
	message string,
	cause error,
) error {
	return &Error{
		Code:      CodeInvalidConfig,
		Operation: operation,
		Message:   message,
		Delivery:  DeliveryNotSent,
		Cause:     cause,
	}
}
