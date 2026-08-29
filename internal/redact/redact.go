package redact

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const redactedValue = "[REDACTED]"

var sensitiveKeys = map[string]struct{}{
	"authorization": {},
	"password":      {},
	"passwd":        {},
	"secret":        {},
	"apikey":        {},
	"accesstoken":   {},
	"refreshtoken":  {},
	"sessiontoken":  {},
	"authtoken":     {},
	"idtoken":       {},
	"credential":    {},
	"credentials":   {},
	"privatekey":    {},
	"clientsecret":  {},
}

// Text 对可能包含敏感信息的文本进行保守脱敏。
//
// 如果文本中出现已知的敏感字段标识，则整个文本都会被替换，
// 避免尝试解析不确定格式的自由文本而造成敏感信息泄露。
func Text(value string) string {
	lower := strings.ToLower(value)

	for key := range sensitiveKeys {
		if strings.Contains(normalizeKey(lower), key) {
			return redactedValue
		}
	}

	return value
}

// JSON 对 JSON 数据中的敏感字段值进行脱敏。
//
// JSON 对象会递归处理；数组中的对象同样会被处理。
// 非敏感字段及其原始值类型保持不变。
func JSON(data []byte) ([]byte, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("JSON data is empty")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("JSON data contains multiple values")
		}
		return nil, err
	}

	redactValue(value)

	return json.Marshal(value)
}

// redactValue 递归处理 JSON 解码后的值。
func redactValue(value any) {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			if isSensitiveKey(key) {
				current[key] = redactedValue
				continue
			}
			redactValue(child)
		}

	case []any:
		for _, child := range current {
			redactValue(child)
		}
	}
}

// isSensitiveKey 判断 JSON 字段名是否属于敏感字段。
func isSensitiveKey(key string) bool {
	_, ok := sensitiveKeys[normalizeKey(key)]
	return ok
}

// normalizeKey 将字段名转换成用于敏感字段匹配的规范形式。
func normalizeKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))

	replacer := strings.NewReplacer(
		"_", "",
		"-", "",
		" ", "",
	)

	return replacer.Replace(key)
}
