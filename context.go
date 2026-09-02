package appium

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	getContextsOperation       = "get_contexts"
	getCurrentContextOperation = "get_current_context"
	switchContextOperation     = "switch_context"
	getWebViewportOperation    = "get_web_viewport"

	webViewportScript = "return {scrollX: window.scrollX, scrollY: window.scrollY, width: window.innerWidth, height: window.innerHeight}"
)

type applicationContextKind uint8

const (
	applicationContextUnknown applicationContextKind = iota
	applicationContextNative
	applicationContextWeb
)

type webViewport struct {
	scrollX float64
	scrollY float64
	rect    Rect
}

type elementGeometry struct {
	kind     applicationContextKind
	viewport Rect
	web      webViewport
}

// Contexts 获取当前 Session 可用的远端 Context 名称快照。
//
// 返回值保留远端顺序、重复项和空字符串。空数组返回非 nil 空 slice。
// 客户端不会缓存、规范化或根据结果自动切换 Context。
func (s *Session) Contexts(ctx context.Context) ([]string, error) {
	client, err := s.commandClient(getContextsOperation)
	if err != nil {
		return nil, err
	}

	command, err := wire.NewCommand(
		getContextsOperation,
		http.MethodGet,
		"session",
		s.id,
		"contexts",
	)
	if err != nil {
		return nil, commandDefinitionError(
			getContextsOperation,
			"get contexts command definition is invalid",
			err,
		)
	}

	var contexts []string
	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decodeContexts(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			contexts = decoded
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return contexts, nil
}

// CurrentContext 获取当前 Session 的远端 Context 名称快照。
//
// 返回值按远端原样保留，包括空字符串。客户端不会缓存或推断 Context 类型。
func (s *Session) CurrentContext(ctx context.Context) (string, error) {
	client, err := s.commandClient(getCurrentContextOperation)
	if err != nil {
		return "", err
	}

	command, err := wire.NewCommand(
		getCurrentContextOperation,
		http.MethodGet,
		"session",
		s.id,
		"context",
	)
	if err != nil {
		return "", commandDefinitionError(
			getCurrentContextOperation,
			"get current context command definition is invalid",
			err,
		)
	}

	var current string
	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := codec.DecodeJSONString(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			current = decoded
			return nil
		},
	)
	if err != nil {
		return "", err
	}

	return current, nil
}

// SwitchContext 请求将当前 Session 切换到指定的远端 Context。
//
// name 必须是有效 UTF-8，并按调用方提供的确切字符串发送；空字符串和未知名称
// 交由远端判断。成功只表示本次切换命令成功，客户端不会缓存或推测后续状态。
func (s *Session) SwitchContext(
	ctx context.Context,
	name string,
) error {
	client, err := s.commandClient(switchContextOperation)
	if err != nil {
		return err
	}

	if !utf8.ValidString(name) {
		return &Error{
			Code:      CodeInvalidArgument,
			Operation: switchContextOperation,
			Message:   "context name must be valid UTF-8",
			Delivery:  DeliveryNotSent,
		}
	}

	command, err := wire.NewCommand(
		switchContextOperation,
		http.MethodPost,
		"session",
		s.id,
		"context",
	)
	if err != nil {
		return commandDefinitionError(
			switchContextOperation,
			"switch context command definition is invalid",
			err,
		)
	}

	request := struct {
		Name string `json:"name"`
	}{
		Name: name,
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

// contextKindForGeometry 读取一次当前 Context，并为组合几何操作选择策略。
func (s *Session) contextKindForGeometry(
	ctx context.Context,
	operation string,
) (applicationContextKind, error) {
	name, err := s.CurrentContext(ctx)
	if err != nil {
		return applicationContextUnknown, err
	}

	kind := classifyApplicationContext(name)
	if kind == applicationContextUnknown {
		return applicationContextUnknown, &Error{
			Code:      CodeUnsupported,
			Operation: operation,
			Message:   "current context has no supported geometry strategy",
			Delivery:  DeliveryNotSent,
		}
	}

	return kind, nil
}

// classifyApplicationContext 按设计限定的精确名称规则分类 Context。
func classifyApplicationContext(name string) applicationContextKind {
	switch {
	case name == "NATIVE_APP":
		return applicationContextNative
	case name == "WEBVIEW",
		name == "CHROMIUM",
		strings.HasPrefix(name, "WEBVIEW_") && len(name) > len("WEBVIEW_"):
		return applicationContextWeb
	default:
		return applicationContextUnknown
	}
}

// geometryForContext 取得一次组合 Element 操作使用的 viewport 几何快照。
func (s *Session) geometryForContext(
	ctx context.Context,
	kind applicationContextKind,
	operation string,
) (elementGeometry, error) {
	switch kind {
	case applicationContextNative:
		rect, err := s.WindowRect(ctx)
		if err != nil {
			return elementGeometry{}, err
		}
		return elementGeometry{
			kind:     kind,
			viewport: rect,
		}, nil

	case applicationContextWeb:
		viewport, err := s.getWebViewport(ctx)
		if err != nil {
			return elementGeometry{}, err
		}
		return elementGeometry{
			kind:     kind,
			viewport: viewport.rect,
			web:      viewport,
		}, nil

	default:
		return elementGeometry{}, &Error{
			Code:      CodeUnsupported,
			Operation: operation,
			Message:   "application context geometry is unsupported",
			Delivery:  DeliveryNotSent,
		}
	}
}

// elementRect 读取 Element Rect，并按当前几何策略转换为 viewport 坐标。
func (g elementGeometry) elementRect(
	ctx context.Context,
	element *Element,
) (Rect, error) {
	if g.kind == applicationContextWeb {
		return element.rectWithTransform(
			ctx,
			func(rect Rect) (Rect, error) {
				return translateWebElementRect(rect, g.web)
			},
		)
	}

	return element.Rect(ctx)
}

// getWebViewport 通过固定 Execute Script 读取浏览器 CSS layout viewport。
func (s *Session) getWebViewport(ctx context.Context) (webViewport, error) {
	client, err := s.commandClient(getWebViewportOperation)
	if err != nil {
		return webViewport{}, err
	}

	var viewport webViewport
	err = executeScriptCommand(
		ctx,
		client,
		getWebViewportOperation,
		s.id,
		webViewportScript,
		nil,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decodeWebViewport(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			viewport = decoded
			return nil
		},
	)
	if err != nil {
		return webViewport{}, err
	}

	return viewport, nil
}

// decodeContexts 严格解码 Context 名称数组。
func decodeContexts(
	ctx context.Context,
	value json.RawMessage,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var rawContexts []json.RawMessage
	if err := json.Unmarshal(value, &rawContexts); err != nil {
		return nil, fmt.Errorf("decode contexts response: %w", err)
	}
	if rawContexts == nil {
		return nil, errors.New("contexts response must be a JSON array")
	}

	contexts := make([]string, len(rawContexts))
	for index, raw := range rawContexts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		name, err := codec.DecodeJSONString(ctx, raw)
		if err != nil {
			return nil, fmt.Errorf(
				"decode context at index %d: %w",
				index,
				err,
			)
		}
		contexts[index] = name
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return contexts, nil
}

// decodeWebViewport 严格解码固定脚本返回的 CSS layout viewport 快照。
func decodeWebViewport(
	ctx context.Context,
	value json.RawMessage,
) (webViewport, error) {
	if err := ctx.Err(); err != nil {
		return webViewport{}, err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(value, &fields); err != nil {
		return webViewport{}, fmt.Errorf(
			"decode web viewport response: %w",
			err,
		)
	}
	if fields == nil {
		return webViewport{}, errors.New(
			"web viewport response must be a JSON object",
		)
	}

	scrollX, err := decodeWebViewportNumber(ctx, fields, "scrollX")
	if err != nil {
		return webViewport{}, err
	}
	scrollY, err := decodeWebViewportNumber(ctx, fields, "scrollY")
	if err != nil {
		return webViewport{}, err
	}
	width, err := decodeWebViewportNumber(ctx, fields, "width")
	if err != nil {
		return webViewport{}, err
	}
	height, err := decodeWebViewportNumber(ctx, fields, "height")
	if err != nil {
		return webViewport{}, err
	}

	if !(width > 0) || !(height > 0) {
		return webViewport{}, errors.New(
			"web viewport must have positive size",
		)
	}
	if !finiteFloat(scrollX+width) || !finiteFloat(scrollY+height) {
		return webViewport{}, errors.New(
			"web viewport contains a non-finite endpoint",
		)
	}

	if err := ctx.Err(); err != nil {
		return webViewport{}, err
	}

	return webViewport{
		scrollX: scrollX,
		scrollY: scrollY,
		rect: Rect{
			Width:  width,
			Height: height,
		},
	}, nil
}

// decodeWebViewportNumber 解码固定 viewport 响应中的一个必需有限数值。
func decodeWebViewportNumber(
	ctx context.Context,
	fields map[string]json.RawMessage,
	name string,
) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	raw, exists := fields[name]
	if !exists || len(bytes.TrimSpace(raw)) == 0 {
		return 0, fmt.Errorf("web viewport response does not contain %s", name)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, fmt.Errorf("web viewport %s must be a JSON number", name)
	}

	var number float64
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, fmt.Errorf("decode web viewport %s: %w", name, err)
	}
	if !finiteFloat(number) {
		return 0, fmt.Errorf("web viewport %s must be finite", name)
	}

	return number, nil
}

// translateWebElementRect 将文档坐标 Element Rect 平移到 CSS viewport 坐标。
func translateWebElementRect(
	rect Rect,
	viewport webViewport,
) (Rect, error) {
	translated := Rect{
		X:      rect.X - viewport.scrollX,
		Y:      rect.Y - viewport.scrollY,
		Width:  rect.Width,
		Height: rect.Height,
	}

	if !finiteFloat(translated.X) ||
		!finiteFloat(translated.Y) ||
		!finiteFloat(translated.Width) ||
		!finiteFloat(translated.Height) ||
		!finiteFloat(translated.X+translated.Width) ||
		!finiteFloat(translated.Y+translated.Height) {
		return Rect{}, errors.New(
			"translated web element rect contains a non-finite value",
		)
	}

	return translated, nil
}

func finiteFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
