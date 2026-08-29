package contracttest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
)

// MatchMethod 精确匹配请求的 HTTP Method。
func MatchMethod(
	request RecordedRequest,
	expected string,
) error {
	if expected == "" {
		return errors.New(
			"expected HTTP method must not be empty",
		)
	}

	if request.Method != expected {
		return fmt.Errorf(
			"HTTP method mismatch: expected %q, got %q",
			expected,
			request.Method,
		)
	}

	return nil
}

// MatchRequestURI 精确匹配请求的原始 Request URI。
//
// 该匹配基于 Recorder 保存的 RequestURI，
// 因此能够验证 Path escaping 和 Raw Query 的真实 wire 形态。
func MatchRequestURI(
	request RecordedRequest,
	expected string,
) error {
	if expected == "" {
		return errors.New(
			"expected request URI must not be empty",
		)
	}

	if request.RequestURI != expected {
		return fmt.Errorf(
			"request URI mismatch: expected %q, got %q",
			expected,
			request.RequestURI,
		)
	}

	return nil
}

// MatchHeader 精确匹配一个单值 HTTP Header。
//
// Header 名称按照 net/http 的规则进行大小写无关查找。
// 如果 Header 不存在、存在多个值或值不一致，都视为不匹配。
func MatchHeader(
	request RecordedRequest,
	name string,
	expected string,
) error {
	if name == "" {
		return errors.New(
			"expected header name must not be empty",
		)
	}

	values := request.Header.Values(name)

	if len(values) == 0 {
		return fmt.Errorf(
			"HTTP header %q is missing",
			name,
		)
	}

	if len(values) != 1 {
		return fmt.Errorf(
			"HTTP header %q value count mismatch: expected 1, got %d",
			name,
			len(values),
		)
	}

	if values[0] != expected {
		return fmt.Errorf(
			"HTTP header %q mismatch: expected %q, got %q",
			name,
			expected,
			values[0],
		)
	}

	return nil
}

// MatchBody 精确匹配请求 Body 字节。
//
// nil 和空字节切片都表示长度为零的 Body。
// JSON 请求通常应使用 MatchJSONBody，避免测试依赖对象字段顺序或空白格式。
func MatchBody(
	request RecordedRequest,
	expected []byte,
) error {
	if !bytes.Equal(
		request.Body,
		expected,
	) {
		return fmt.Errorf(
			"request body mismatch: expected %q, got %q",
			expected,
			request.Body,
		)
	}

	return nil
}

// MatchJSONBody 按 JSON 结构精确匹配请求 Body。
//
// expected 可以是任意能够被 encoding/json 编码的 Go 值，
// 也可以是 json.RawMessage。
//
// JSON 对象字段顺序和无意义空白不参与匹配；
// 数组顺序、字段类型和值必须一致。
// JSON number 使用 json.Number 解码，避免先转换为 float64 后丢失整数精度。
func MatchJSONBody(
	request RecordedRequest,
	expected any,
) error {
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return fmt.Errorf(
			"encode expected JSON body: %w",
			err,
		)
	}

	actualValue, err := decodeJSONValue(
		request.Body,
	)
	if err != nil {
		return fmt.Errorf(
			"decode actual JSON body: %w",
			err,
		)
	}

	expectedValue, err := decodeJSONValue(
		expectedJSON,
	)
	if err != nil {
		return fmt.Errorf(
			"decode expected JSON body: %w",
			err,
		)
	}

	if !reflect.DeepEqual(
		actualValue,
		expectedValue,
	) {
		return fmt.Errorf(
			"JSON body mismatch: expected %s, got %s",
			expectedJSON,
			request.Body,
		)
	}

	return nil
}

// decodeJSONValue 严格解码一个完整 JSON 值。
//
// UseNumber 保留 JSON number 的文本精度，
// 第二次 Decode 用于拒绝尾随的额外 JSON 值。
func decodeJSONValue(
	data []byte,
) (any, error) {
	decoder := json.NewDecoder(
		bytes.NewReader(data),
	)
	decoder.UseNumber()

	var value any

	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	var trailing any

	err := decoder.Decode(&trailing)
	if !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New(
				"JSON body contains multiple values",
			)
		}

		return nil, err
	}

	return value, nil
}
