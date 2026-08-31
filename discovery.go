package appium

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/xieliangji/soluna-appium-client/internal/codec"
	"github.com/xieliangji/soluna-appium-client/internal/wire"
)

const (
	listCommandsOperation   = "list_commands"
	listExtensionsOperation = "list_extensions"
)

// CatalogSourceKind 表示 Runtime Discovery 条目的来源层级。
type CatalogSourceKind string

const (
	// CatalogSourceBase 表示 Appium Base Driver 提供的条目。
	CatalogSourceBase CatalogSourceKind = "base"

	// CatalogSourceDriver 表示当前 Session Driver 提供的条目。
	CatalogSourceDriver CatalogSourceKind = "driver"

	// CatalogSourcePlugin 表示 Appium Plugin 提供的条目。
	CatalogSourcePlugin CatalogSourceKind = "plugin"
)

// CatalogSource 保存 Runtime Discovery 条目的来源 provenance。
//
// PluginName 仅在 Kind 为 CatalogSourcePlugin 时有值。
type CatalogSource struct {
	// Kind 表示条目的来源层级。
	Kind CatalogSourceKind
	// PluginName 表示 Plugin 来源的稳定名称。
	PluginName string
}

// CatalogParam 表示一个命令或 Execute Method 的参数元数据。
//
// Extra 保存远端返回的未知参数字段。
type CatalogParam struct {
	// Name 表示参数名称。
	Name string
	// Required 表示该参数是否必需。
	Required bool
	// Extra 保存未知参数字段。
	Extra map[string]any
}

// CatalogMetadata 保存 Runtime Discovery 条目的可选元数据。
//
// 指针字段为 nil 时表示远端响应中缺失对应字段；Params 为 nil 时表示
// params 字段缺失，非 nil 空 slice 表示远端明确返回了空数组。
type CatalogMetadata struct {
	// Command 表示远端可选的命令名元数据。
	Command *string
	// Deprecated 表示远端是否将条目标记为 deprecated。
	Deprecated *bool
	// Info 表示远端提供的说明文本。
	Info *string
	// Params 保存远端返回的参数元数据。
	Params []CatalogParam
	// Extra 保存未知元数据字段。
	Extra map[string]any
}

// HTTPCommand 表示一个 REST API 命令及其 execution identity。
type HTTPCommand struct {
	// CatalogMetadata 保存条目的可选元数据。
	CatalogMetadata
	// Source 保存条目的来源 provenance。
	Source CatalogSource
	// Path 表示 REST 路由路径 identity。
	Path string
	// Method 表示 REST HTTP method identity。
	Method string
}

// BiDiCommand 表示一个 WebDriver BiDi 命令及其 execution identity。
type BiDiCommand struct {
	// CatalogMetadata 保存条目的可选元数据。
	CatalogMetadata
	// Source 保存条目的来源 provenance。
	Source CatalogSource
	// Domain 表示 BiDi domain identity。
	Domain string
	// Name 表示 BiDi command name identity。
	Name string
}

// ExecuteMethod 表示一个 Appium Execute Method 及其 execution identity。
type ExecuteMethod struct {
	// CatalogMetadata 保存条目的可选元数据。
	CatalogMetadata
	// Source 保存条目的来源 provenance。
	Source CatalogSource
	// Name 表示 Execute Method identity。
	Name string
}

// HTTPCommandGroup 保存一组展开后的 REST 命令条目。
type HTTPCommandGroup struct {
	// Entries 保存展开后的 REST 命令条目。
	Entries []HTTPCommand
}

// BiDiCommandGroup 保存一组展开后的 BiDi 命令条目。
type BiDiCommandGroup struct {
	// Entries 保存展开后的 BiDi 命令条目。
	Entries []BiDiCommand
}

// ExecuteMethodGroup 保存一组展开后的 Execute Method 条目。
type ExecuteMethodGroup struct {
	// Entries 保存展开后的 Execute Method 条目。
	Entries []ExecuteMethod
}

// CommandCatalog 表示一次 Runtime Discovery Command Catalog 快照。
//
// Rest 或 BiDi 为 nil 时表示对应 section 在远端响应中缺失。
type CommandCatalog struct {
	// Rest 保存 REST section；nil 表示远端缺失该 section。
	Rest *RestCommandCatalog
	// BiDi 保存 BiDi section；nil 表示远端缺失该 section。
	BiDi *BiDiCommandCatalog
	// Extra 保存未知顶层字段。
	Extra map[string]any
}

// RestCommandCatalog 表示 REST 命令目录的来源分组。
//
// Plugins 为 nil 表示 plugins 字段缺失；非 nil 空 map 表示远端明确返回了
// 空 plugins object。
type RestCommandCatalog struct {
	// Base 保存 Appium Base Driver 的 REST 条目。
	Base HTTPCommandGroup
	// Driver 保存当前 Session Driver 的 REST 条目。
	Driver HTTPCommandGroup
	// Plugins 保存各 Plugin 的 REST 条目。
	Plugins map[string]HTTPCommandGroup
	// Extra 保存未知 section 字段。
	Extra map[string]any
}

// BiDiCommandCatalog 表示 BiDi 命令目录的来源分组。
//
// Plugins 为 nil 表示 plugins 字段缺失；非 nil 空 map 表示远端明确返回了
// 空 plugins object。
type BiDiCommandCatalog struct {
	// Base 保存 Appium Base Driver 的 BiDi 条目。
	Base BiDiCommandGroup
	// Driver 保存当前 Session Driver 的 BiDi 条目。
	Driver BiDiCommandGroup
	// Plugins 保存各 Plugin 的 BiDi 条目。
	Plugins map[string]BiDiCommandGroup
	// Extra 保存未知 section 字段。
	Extra map[string]any
}

// ExtensionCatalog 表示一次 Runtime Discovery Extension Catalog 快照。
//
// Rest 为 nil 时表示 rest section 在远端响应中缺失。
type ExtensionCatalog struct {
	// Rest 保存 REST Execute Method section；nil 表示远端缺失该 section。
	Rest *RestExtensionCatalog
	// Extra 保存未知顶层字段。
	Extra map[string]any
}

// RestExtensionCatalog 表示 Execute Method 目录的来源分组。
//
// Plugins 为 nil 表示 plugins 字段缺失；非 nil 空 map 表示远端明确返回了
// 空 plugins object。
type RestExtensionCatalog struct {
	// Driver 保存当前 Session Driver 的 Execute Method 条目。
	Driver ExecuteMethodGroup
	// Plugins 保存各 Plugin 的 Execute Method 条目。
	Plugins map[string]ExecuteMethodGroup
	// Extra 保存未知 section 字段。
	Extra map[string]any
}

// Commands 读取当前 Session 的 Appium Runtime Discovery Command Catalog。
//
// 每次调用都会重新请求远端，不建立本地缓存。返回目录及其嵌套数据只属于
// 本次快照；普通 SDK 命令不会隐式调用该方法。
func (s *Session) Commands(ctx context.Context) (CommandCatalog, error) {
	client, err := s.commandClient(listCommandsOperation)
	if err != nil {
		return CommandCatalog{}, err
	}

	command, err := wire.NewCommand(
		listCommandsOperation,
		http.MethodGet,
		"session",
		s.id,
		"appium",
		"commands",
	)
	if err != nil {
		return CommandCatalog{}, commandDefinitionError(
			listCommandsOperation,
			"list commands command definition is invalid",
			err,
		)
	}

	var catalog CommandCatalog
	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decodeCommandCatalog(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			catalog = decoded
			return nil
		},
	)
	if err != nil {
		return CommandCatalog{}, err
	}

	return catalog, nil
}

// Extensions 读取当前 Session 的 Appium Runtime Discovery Extension Catalog。
//
// 每次调用都会重新请求远端，不建立本地缓存。返回目录及其嵌套数据只属于
// 本次快照；目录结果不会作为其他命令的隐式能力门禁。
func (s *Session) Extensions(ctx context.Context) (ExtensionCatalog, error) {
	client, err := s.commandClient(listExtensionsOperation)
	if err != nil {
		return ExtensionCatalog{}, err
	}

	command, err := wire.NewCommand(
		listExtensionsOperation,
		http.MethodGet,
		"session",
		s.id,
		"appium",
		"extensions",
	)
	if err != nil {
		return ExtensionCatalog{}, commandDefinitionError(
			listExtensionsOperation,
			"list extensions command definition is invalid",
			err,
		)
	}

	var catalog ExtensionCatalog
	err = client.executeCommand(
		ctx,
		command,
		nil,
		client.commandTimeout,
		client.limits.MaxResponseBytes,
		func(ctx context.Context, value json.RawMessage) error {
			decoded, decodeErr := decodeExtensionCatalog(ctx, value)
			if decodeErr != nil {
				return decodeErr
			}
			catalog = decoded
			return nil
		},
	)
	if err != nil {
		return ExtensionCatalog{}, err
	}

	return catalog, nil
}

// SupportsHTTP 判断目录是否登记了指定的 REST method/path identity。
//
// 匹配区分大小写并要求逐字节相等；任一参数为空时返回 false。该方法只
// 查询本地快照，不代表远端随后一定能够成功执行命令。
func (c CommandCatalog) SupportsHTTP(method, path string) bool {
	if method == "" || path == "" || c.Rest == nil {
		return false
	}

	if supportsHTTPGroup(c.Rest.Base, method, path) ||
		supportsHTTPGroup(c.Rest.Driver, method, path) {
		return true
	}

	for _, group := range c.Rest.Plugins {
		if supportsHTTPGroup(group, method, path) {
			return true
		}
	}

	return false
}

// SupportsBiDi 判断目录是否登记了指定的 BiDi domain/name identity。
//
// 匹配区分大小写并要求逐字节相等；任一参数为空时返回 false。该方法只
// 查询本地快照，不代表远端随后一定能够成功执行命令。
func (c CommandCatalog) SupportsBiDi(domain, name string) bool {
	if domain == "" || name == "" || c.BiDi == nil {
		return false
	}

	if supportsBiDiGroup(c.BiDi.Base, domain, name) ||
		supportsBiDiGroup(c.BiDi.Driver, domain, name) {
		return true
	}

	for _, group := range c.BiDi.Plugins {
		if supportsBiDiGroup(group, domain, name) {
			return true
		}
	}

	return false
}

// SupportsExecuteMethod 判断目录是否登记了指定的 Execute Method name。
//
// 匹配区分大小写并要求逐字节相等；name 为空时返回 false。该方法只查询
// 本地快照，不代表远端随后一定能够成功执行命令。
func (c ExtensionCatalog) SupportsExecuteMethod(name string) bool {
	if name == "" || c.Rest == nil {
		return false
	}

	if supportsExecuteMethodGroup(c.Rest.Driver, name) {
		return true
	}

	for _, group := range c.Rest.Plugins {
		if supportsExecuteMethodGroup(group, name) {
			return true
		}
	}

	return false
}

func supportsHTTPGroup(group HTTPCommandGroup, method, path string) bool {
	for _, entry := range group.Entries {
		if entry.Method == method && entry.Path == path {
			return true
		}
	}
	return false
}

func supportsBiDiGroup(group BiDiCommandGroup, domain, name string) bool {
	for _, entry := range group.Entries {
		if entry.Domain == domain && entry.Name == name {
			return true
		}
	}
	return false
}

func supportsExecuteMethodGroup(group ExecuteMethodGroup, name string) bool {
	for _, entry := range group.Entries {
		if entry.Name == name {
			return true
		}
	}
	return false
}

func decodeCommandCatalog(
	ctx context.Context,
	value json.RawMessage,
) (CommandCatalog, error) {
	fields, err := decodeDiscoveryObject(ctx, value, "command catalog")
	if err != nil {
		return CommandCatalog{}, err
	}

	var catalog CommandCatalog
	if raw, ok := fields["rest"]; ok {
		rest, decodeErr := decodeRestCommandCatalog(ctx, raw)
		if decodeErr != nil {
			return CommandCatalog{}, decodeErr
		}
		catalog.Rest = &rest
	}
	if raw, ok := fields["bidi"]; ok {
		bidi, decodeErr := decodeBiDiCommandCatalog(ctx, raw)
		if decodeErr != nil {
			return CommandCatalog{}, decodeErr
		}
		catalog.BiDi = &bidi
	}

	catalog.Extra, err = decodeDiscoveryExtra(ctx, fields, isCommandCatalogField)
	if err != nil {
		return CommandCatalog{}, err
	}

	return catalog, nil
}

func decodeExtensionCatalog(
	ctx context.Context,
	value json.RawMessage,
) (ExtensionCatalog, error) {
	fields, err := decodeDiscoveryObject(ctx, value, "extension catalog")
	if err != nil {
		return ExtensionCatalog{}, err
	}

	var catalog ExtensionCatalog
	if raw, ok := fields["rest"]; ok {
		rest, decodeErr := decodeRestExtensionCatalog(ctx, raw)
		if decodeErr != nil {
			return ExtensionCatalog{}, decodeErr
		}
		catalog.Rest = &rest
	}

	catalog.Extra, err = decodeDiscoveryExtra(ctx, fields, isExtensionCatalogField)
	if err != nil {
		return ExtensionCatalog{}, err
	}

	return catalog, nil
}

func decodeRestCommandCatalog(
	ctx context.Context,
	raw json.RawMessage,
) (RestCommandCatalog, error) {
	fields, err := decodeDiscoveryObject(ctx, raw, "command catalog rest section")
	if err != nil {
		return RestCommandCatalog{}, err
	}

	baseRaw, ok := fields["base"]
	if !ok {
		return RestCommandCatalog{}, errors.New(
			"command catalog rest section does not contain base",
		)
	}
	driverRaw, ok := fields["driver"]
	if !ok {
		return RestCommandCatalog{}, errors.New(
			"command catalog rest section does not contain driver",
		)
	}

	base, err := decodeHTTPCommandGroup(
		ctx,
		baseRaw,
		CatalogSource{Kind: CatalogSourceBase},
		"command catalog rest.base",
	)
	if err != nil {
		return RestCommandCatalog{}, err
	}
	driver, err := decodeHTTPCommandGroup(
		ctx,
		driverRaw,
		CatalogSource{Kind: CatalogSourceDriver},
		"command catalog rest.driver",
	)
	if err != nil {
		return RestCommandCatalog{}, err
	}

	catalog := RestCommandCatalog{
		Base:   base,
		Driver: driver,
	}
	if pluginsRaw, ok := fields["plugins"]; ok {
		catalog.Plugins, err = decodeHTTPCommandPlugins(
			ctx,
			pluginsRaw,
			"command catalog rest.plugins",
		)
		if err != nil {
			return RestCommandCatalog{}, err
		}
	}

	catalog.Extra, err = decodeDiscoveryExtra(ctx, fields, isRestCommandCatalogField)
	if err != nil {
		return RestCommandCatalog{}, err
	}

	return catalog, nil
}

func decodeBiDiCommandCatalog(
	ctx context.Context,
	raw json.RawMessage,
) (BiDiCommandCatalog, error) {
	fields, err := decodeDiscoveryObject(ctx, raw, "command catalog bidi section")
	if err != nil {
		return BiDiCommandCatalog{}, err
	}

	baseRaw, ok := fields["base"]
	if !ok {
		return BiDiCommandCatalog{}, errors.New(
			"command catalog bidi section does not contain base",
		)
	}
	driverRaw, ok := fields["driver"]
	if !ok {
		return BiDiCommandCatalog{}, errors.New(
			"command catalog bidi section does not contain driver",
		)
	}

	base, err := decodeBiDiCommandGroup(
		ctx,
		baseRaw,
		CatalogSource{Kind: CatalogSourceBase},
		"command catalog bidi.base",
	)
	if err != nil {
		return BiDiCommandCatalog{}, err
	}
	driver, err := decodeBiDiCommandGroup(
		ctx,
		driverRaw,
		CatalogSource{Kind: CatalogSourceDriver},
		"command catalog bidi.driver",
	)
	if err != nil {
		return BiDiCommandCatalog{}, err
	}

	catalog := BiDiCommandCatalog{
		Base:   base,
		Driver: driver,
	}
	if pluginsRaw, ok := fields["plugins"]; ok {
		catalog.Plugins, err = decodeBiDiCommandPlugins(
			ctx,
			pluginsRaw,
			"command catalog bidi.plugins",
		)
		if err != nil {
			return BiDiCommandCatalog{}, err
		}
	}

	catalog.Extra, err = decodeDiscoveryExtra(ctx, fields, isBiDiCommandCatalogField)
	if err != nil {
		return BiDiCommandCatalog{}, err
	}

	return catalog, nil
}

func decodeRestExtensionCatalog(
	ctx context.Context,
	raw json.RawMessage,
) (RestExtensionCatalog, error) {
	fields, err := decodeDiscoveryObject(ctx, raw, "extension catalog rest section")
	if err != nil {
		return RestExtensionCatalog{}, err
	}

	driverRaw, ok := fields["driver"]
	if !ok {
		return RestExtensionCatalog{}, errors.New(
			"extension catalog rest section does not contain driver",
		)
	}

	driver, err := decodeExecuteMethodGroup(
		ctx,
		driverRaw,
		CatalogSource{Kind: CatalogSourceDriver},
		"extension catalog rest.driver",
	)
	if err != nil {
		return RestExtensionCatalog{}, err
	}

	catalog := RestExtensionCatalog{Driver: driver}
	if pluginsRaw, ok := fields["plugins"]; ok {
		catalog.Plugins, err = decodeExecuteMethodPlugins(
			ctx,
			pluginsRaw,
			"extension catalog rest.plugins",
		)
		if err != nil {
			return RestExtensionCatalog{}, err
		}
	}

	catalog.Extra, err = decodeDiscoveryExtra(ctx, fields, isRestExtensionCatalogField)
	if err != nil {
		return RestExtensionCatalog{}, err
	}

	return catalog, nil
}

func decodeHTTPCommandPlugins(
	ctx context.Context,
	raw json.RawMessage,
	label string,
) (map[string]HTTPCommandGroup, error) {
	fields, err := decodeDiscoveryObject(ctx, raw, label)
	if err != nil {
		return nil, err
	}

	plugins := make(map[string]HTTPCommandGroup, len(fields))
	for _, pluginName := range sortedDiscoveryKeys(fields) {
		if err := checkDiscoveryContext(ctx); err != nil {
			return nil, err
		}
		if pluginName == "" {
			return nil, fmt.Errorf("%s contains an empty plugin name", label)
		}

		group, decodeErr := decodeHTTPCommandGroup(
			ctx,
			fields[pluginName],
			CatalogSource{Kind: CatalogSourcePlugin, PluginName: pluginName},
			label+"["+pluginName+"]",
		)
		if decodeErr != nil {
			return nil, decodeErr
		}
		plugins[pluginName] = group
	}

	return plugins, nil
}

func decodeBiDiCommandPlugins(
	ctx context.Context,
	raw json.RawMessage,
	label string,
) (map[string]BiDiCommandGroup, error) {
	fields, err := decodeDiscoveryObject(ctx, raw, label)
	if err != nil {
		return nil, err
	}

	plugins := make(map[string]BiDiCommandGroup, len(fields))
	for _, pluginName := range sortedDiscoveryKeys(fields) {
		if err := checkDiscoveryContext(ctx); err != nil {
			return nil, err
		}
		if pluginName == "" {
			return nil, fmt.Errorf("%s contains an empty plugin name", label)
		}

		group, decodeErr := decodeBiDiCommandGroup(
			ctx,
			fields[pluginName],
			CatalogSource{Kind: CatalogSourcePlugin, PluginName: pluginName},
			label+"["+pluginName+"]",
		)
		if decodeErr != nil {
			return nil, decodeErr
		}
		plugins[pluginName] = group
	}

	return plugins, nil
}

func decodeExecuteMethodPlugins(
	ctx context.Context,
	raw json.RawMessage,
	label string,
) (map[string]ExecuteMethodGroup, error) {
	fields, err := decodeDiscoveryObject(ctx, raw, label)
	if err != nil {
		return nil, err
	}

	plugins := make(map[string]ExecuteMethodGroup, len(fields))
	for _, pluginName := range sortedDiscoveryKeys(fields) {
		if err := checkDiscoveryContext(ctx); err != nil {
			return nil, err
		}
		if pluginName == "" {
			return nil, fmt.Errorf("%s contains an empty plugin name", label)
		}

		group, decodeErr := decodeExecuteMethodGroup(
			ctx,
			fields[pluginName],
			CatalogSource{Kind: CatalogSourcePlugin, PluginName: pluginName},
			label+"["+pluginName+"]",
		)
		if decodeErr != nil {
			return nil, decodeErr
		}
		plugins[pluginName] = group
	}

	return plugins, nil
}

func decodeHTTPCommandGroup(
	ctx context.Context,
	raw json.RawMessage,
	source CatalogSource,
	label string,
) (HTTPCommandGroup, error) {
	paths, err := decodeDiscoveryObject(ctx, raw, label)
	if err != nil {
		return HTTPCommandGroup{}, err
	}

	entries := make([]HTTPCommand, 0)
	for _, path := range sortedDiscoveryKeys(paths) {
		if err := checkDiscoveryContext(ctx); err != nil {
			return HTTPCommandGroup{}, err
		}
		if path == "" {
			return HTTPCommandGroup{}, fmt.Errorf("%s contains an empty path", label)
		}

		methods, decodeErr := decodeDiscoveryObject(
			ctx,
			paths[path],
			label+"["+path+"]",
		)
		if decodeErr != nil {
			return HTTPCommandGroup{}, decodeErr
		}

		for _, method := range sortedDiscoveryKeys(methods) {
			if err := checkDiscoveryContext(ctx); err != nil {
				return HTTPCommandGroup{}, err
			}
			if method == "" {
				return HTTPCommandGroup{}, fmt.Errorf(
					"%s[%s] contains an empty method",
					label,
					path,
				)
			}

			metadata, metadataErr := decodeCatalogMetadata(
				ctx,
				methods[method],
				label+"["+path+"]["+method+"]",
			)
			if metadataErr != nil {
				return HTTPCommandGroup{}, metadataErr
			}

			entries = append(entries, HTTPCommand{
				CatalogMetadata: metadata,
				Source:          source,
				Path:            path,
				Method:          method,
			})
		}
	}

	return HTTPCommandGroup{Entries: entries}, nil
}

func decodeBiDiCommandGroup(
	ctx context.Context,
	raw json.RawMessage,
	source CatalogSource,
	label string,
) (BiDiCommandGroup, error) {
	domains, err := decodeDiscoveryObject(ctx, raw, label)
	if err != nil {
		return BiDiCommandGroup{}, err
	}

	entries := make([]BiDiCommand, 0)
	for _, domain := range sortedDiscoveryKeys(domains) {
		if err := checkDiscoveryContext(ctx); err != nil {
			return BiDiCommandGroup{}, err
		}
		if domain == "" {
			return BiDiCommandGroup{}, fmt.Errorf("%s contains an empty domain", label)
		}

		commands, decodeErr := decodeDiscoveryObject(
			ctx,
			domains[domain],
			label+"["+domain+"]",
		)
		if decodeErr != nil {
			return BiDiCommandGroup{}, decodeErr
		}

		for _, name := range sortedDiscoveryKeys(commands) {
			if err := checkDiscoveryContext(ctx); err != nil {
				return BiDiCommandGroup{}, err
			}
			if name == "" {
				return BiDiCommandGroup{}, fmt.Errorf(
					"%s[%s] contains an empty command name",
					label,
					domain,
				)
			}

			metadata, metadataErr := decodeCatalogMetadata(
				ctx,
				commands[name],
				label+"["+domain+"]["+name+"]",
			)
			if metadataErr != nil {
				return BiDiCommandGroup{}, metadataErr
			}

			entries = append(entries, BiDiCommand{
				CatalogMetadata: metadata,
				Source:          source,
				Domain:          domain,
				Name:            name,
			})
		}
	}

	return BiDiCommandGroup{Entries: entries}, nil
}

func decodeExecuteMethodGroup(
	ctx context.Context,
	raw json.RawMessage,
	source CatalogSource,
	label string,
) (ExecuteMethodGroup, error) {
	methods, err := decodeDiscoveryObject(ctx, raw, label)
	if err != nil {
		return ExecuteMethodGroup{}, err
	}

	entries := make([]ExecuteMethod, 0, len(methods))
	for _, name := range sortedDiscoveryKeys(methods) {
		if err := checkDiscoveryContext(ctx); err != nil {
			return ExecuteMethodGroup{}, err
		}
		if name == "" {
			return ExecuteMethodGroup{}, fmt.Errorf(
				"%s contains an empty method name",
				label,
			)
		}

		metadata, metadataErr := decodeCatalogMetadata(
			ctx,
			methods[name],
			label+"["+name+"]",
		)
		if metadataErr != nil {
			return ExecuteMethodGroup{}, metadataErr
		}

		entries = append(entries, ExecuteMethod{
			CatalogMetadata: metadata,
			Source:          source,
			Name:            name,
		})
	}

	return ExecuteMethodGroup{Entries: entries}, nil
}

func decodeCatalogMetadata(
	ctx context.Context,
	raw json.RawMessage,
	label string,
) (CatalogMetadata, error) {
	fields, err := decodeDiscoveryObject(ctx, raw, label)
	if err != nil {
		return CatalogMetadata{}, err
	}

	var metadata CatalogMetadata
	if commandRaw, ok := fields["command"]; ok {
		command, decodeErr := decodeDiscoveryString(
			ctx,
			commandRaw,
			label+".command",
		)
		if decodeErr != nil {
			return CatalogMetadata{}, decodeErr
		}
		metadata.Command = &command
	}
	if deprecatedRaw, ok := fields["deprecated"]; ok {
		deprecated, decodeErr := decodeDiscoveryBool(
			ctx,
			deprecatedRaw,
			label+".deprecated",
		)
		if decodeErr != nil {
			return CatalogMetadata{}, decodeErr
		}
		metadata.Deprecated = &deprecated
	}
	if infoRaw, ok := fields["info"]; ok {
		info, decodeErr := decodeDiscoveryString(
			ctx,
			infoRaw,
			label+".info",
		)
		if decodeErr != nil {
			return CatalogMetadata{}, decodeErr
		}
		metadata.Info = &info
	}
	if paramsRaw, ok := fields["params"]; ok {
		metadata.Params, err = decodeCatalogParams(
			ctx,
			paramsRaw,
			label+".params",
		)
		if err != nil {
			return CatalogMetadata{}, err
		}
	}

	metadata.Extra, err = decodeDiscoveryExtra(ctx, fields, isCatalogMetadataField)
	if err != nil {
		return CatalogMetadata{}, err
	}

	return metadata, nil
}

func decodeCatalogParams(
	ctx context.Context,
	raw json.RawMessage,
	label string,
) ([]CatalogParam, error) {
	items, err := decodeDiscoveryArray(ctx, raw, label)
	if err != nil {
		return nil, err
	}

	params := make([]CatalogParam, len(items))
	for index, item := range items {
		if err := checkDiscoveryContext(ctx); err != nil {
			return nil, err
		}

		fields, decodeErr := decodeDiscoveryObject(
			ctx,
			item,
			fmt.Sprintf("%s[%d]", label, index),
		)
		if decodeErr != nil {
			return nil, decodeErr
		}

		nameRaw, ok := fields["name"]
		if !ok {
			return nil, fmt.Errorf(
				"%s[%d] does not contain name",
				label,
				index,
			)
		}
		requiredRaw, ok := fields["required"]
		if !ok {
			return nil, fmt.Errorf(
				"%s[%d] does not contain required",
				label,
				index,
			)
		}

		name, decodeErr := decodeDiscoveryString(
			ctx,
			nameRaw,
			fmt.Sprintf("%s[%d].name", label, index),
		)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if name == "" {
			return nil, fmt.Errorf(
				"%s[%d].name must not be empty",
				label,
				index,
			)
		}

		required, decodeErr := decodeDiscoveryBool(
			ctx,
			requiredRaw,
			fmt.Sprintf("%s[%d].required", label, index),
		)
		if decodeErr != nil {
			return nil, decodeErr
		}

		extra, decodeErr := decodeDiscoveryExtra(
			ctx,
			fields,
			isCatalogParamField,
		)
		if decodeErr != nil {
			return nil, decodeErr
		}

		params[index] = CatalogParam{
			Name:     name,
			Required: required,
			Extra:    extra,
		}
	}

	return params, nil
}

func decodeDiscoveryObject(
	ctx context.Context,
	raw json.RawMessage,
	label string,
) (map[string]json.RawMessage, error) {
	if err := checkDiscoveryContext(ctx); err != nil {
		return nil, err
	}
	if err := codec.ValidateUTF8(ctx, raw); err != nil {
		return nil, err
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("%s must be a JSON object", label)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	if fields == nil {
		return nil, fmt.Errorf("%s must be a JSON object", label)
	}

	if err := checkDiscoveryContext(ctx); err != nil {
		return nil, err
	}

	return fields, nil
}

func decodeDiscoveryArray(
	ctx context.Context,
	raw json.RawMessage,
	label string,
) ([]json.RawMessage, error) {
	if err := checkDiscoveryContext(ctx); err != nil {
		return nil, err
	}
	if err := codec.ValidateUTF8(ctx, raw); err != nil {
		return nil, err
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, fmt.Errorf("%s must be a JSON array", label)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(trimmed, &items); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	if items == nil {
		// The only valid JSON array which can otherwise result in a nil slice is
		// an explicitly empty array; preserve its presence as a non-nil slice.
		items = make([]json.RawMessage, 0)
	}

	if err := checkDiscoveryContext(ctx); err != nil {
		return nil, err
	}

	return items, nil
}

func decodeDiscoveryString(
	ctx context.Context,
	raw json.RawMessage,
	label string,
) (string, error) {
	if err := checkDiscoveryContext(ctx); err != nil {
		return "", err
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a JSON string", label)
	}

	value, err := codec.DecodeJSONString(ctx, raw)
	if err != nil {
		return "", fmt.Errorf("decode %s: %w", label, err)
	}

	return value, nil
}

func decodeDiscoveryBool(
	ctx context.Context,
	raw json.RawMessage,
	label string,
) (bool, error) {
	if err := checkDiscoveryContext(ctx); err != nil {
		return false, err
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, fmt.Errorf("%s must be a JSON boolean", label)
	}

	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("decode %s: %w", label, err)
	}
	if err := checkDiscoveryContext(ctx); err != nil {
		return false, err
	}

	return value, nil
}

func decodeDiscoveryExtra(
	ctx context.Context,
	fields map[string]json.RawMessage,
	isKnown func(string) bool,
) (map[string]any, error) {
	if err := checkDiscoveryContext(ctx); err != nil {
		return nil, err
	}

	extraCount := 0
	for key := range fields {
		if !isKnown(key) {
			extraCount++
		}
	}
	if extraCount == 0 {
		if err := checkDiscoveryContext(ctx); err != nil {
			return nil, err
		}
		return nil, nil
	}

	extra := make(map[string]any, extraCount)
	for _, key := range sortedDiscoveryKeys(fields) {
		if isKnown(key) {
			continue
		}
		if err := checkDiscoveryContext(ctx); err != nil {
			return nil, err
		}

		if err := codec.ValidateUTF8(ctx, fields[key]); err != nil {
			return nil, err
		}

		var value any
		if err := json.Unmarshal(fields[key], &value); err != nil {
			return nil, fmt.Errorf("decode unknown catalog field %q: %w", key, err)
		}
		extra[key] = cloneJSONValue(value)
	}

	if err := checkDiscoveryContext(ctx); err != nil {
		return nil, err
	}

	return extra, nil
}

func sortedDiscoveryKeys(fields map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func checkDiscoveryContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	return ctx.Err()
}

func isCommandCatalogField(key string) bool {
	return key == "rest" || key == "bidi"
}

func isExtensionCatalogField(key string) bool {
	return key == "rest"
}

func isRestCommandCatalogField(key string) bool {
	return key == "base" || key == "driver" || key == "plugins"
}

func isBiDiCommandCatalogField(key string) bool {
	return key == "base" || key == "driver" || key == "plugins"
}

func isRestExtensionCatalogField(key string) bool {
	return key == "driver" || key == "plugins"
}

func isCatalogMetadataField(key string) bool {
	return key == "command" || key == "deprecated" || key == "info" || key == "params"
}

func isCatalogParamField(key string) bool {
	return key == "name" || key == "required"
}
