package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// The JSON types below keep the key order and the one-line formatting of the decoded
// document, so config files edited by ngcheck init only change where something was added.

type jsonObject struct {
	keys   []string
	values map[string]any
	// inline objects were written on one line
	inline bool
}

type jsonArray struct {
	items  []any
	inline bool
}

func newJSONObject() *jsonObject {
	return &jsonObject{values: map[string]any{}}
}

func (o *jsonObject) get(key string) (any, bool) {
	value, ok := o.values[key]

	return value, ok
}

// set replaces the value of an existing key in place, or appends a new key.
func (o *jsonObject) set(key string, value any) {
	if _, ok := o.values[key]; !ok {
		o.keys = append(o.keys, key)
	}

	o.values[key] = value
}

// decodeJSON parses a document into *jsonObject, *jsonArray, string, json.Number, bool and nil values.
func decodeJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	value, err := decodeJSONValue(decoder, data)
	if err != nil {
		return nil, err
	}

	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected content after the JSON document")
	}

	return value, nil
}

func decodeJSONValue(decoder *json.Decoder, data []byte) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	delimiter, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}

	// The offset right after the opening delimiter.
	start := decoder.InputOffset()

	switch delimiter {
	case '{':
		object := newJSONObject()

		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}

			value, err := decodeJSONValue(decoder, data)
			if err != nil {
				return nil, err
			}

			object.set(keyToken.(string), value)
		}

		if _, err := decoder.Token(); err != nil {
			return nil, err
		}

		object.inline = !bytes.ContainsRune(data[start:decoder.InputOffset()], '\n')

		return object, nil
	case '[':
		array := &jsonArray{}

		for decoder.More() {
			value, err := decodeJSONValue(decoder, data)
			if err != nil {
				return nil, err
			}

			array.items = append(array.items, value)
		}

		if _, err := decoder.Token(); err != nil {
			return nil, err
		}

		array.inline = !bytes.ContainsRune(data[start:decoder.InputOffset()], '\n')

		return array, nil
	default:
		return nil, fmt.Errorf("unexpected %q", delimiter)
	}
}

// encodeJSON writes a document with 4-space indentation, the CaptainHook and Composer
// style. Objects and arrays decoded from one line stay on one line, as long as all
// their values do.
func encodeJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer

	if err := writeJSONValue(&buffer, value, 0); err != nil {
		return nil, err
	}

	buffer.WriteByte('\n')

	return buffer.Bytes(), nil
}

func writeJSONValue(buffer *bytes.Buffer, value any, depth int) error {
	switch v := value.(type) {
	case *jsonObject:
		entries := make([]func() error, 0, len(v.keys))
		for _, key := range v.keys {
			entries = append(entries, func() error {
				writeJSONString(buffer, key)
				buffer.WriteString(": ")

				return writeJSONValue(buffer, v.values[key], depth+1)
			})
		}

		return writeJSONContainer(buffer, "{", "}", entries, isInline(v), depth)
	case *jsonArray:
		entries := make([]func() error, 0, len(v.items))
		for _, item := range v.items {
			entries = append(entries, func() error {
				return writeJSONValue(buffer, item, depth+1)
			})
		}

		return writeJSONContainer(buffer, "[", "]", entries, isInline(v), depth)
	case string:
		writeJSONString(buffer, v)
	case json.Number:
		buffer.WriteString(v.String())
	case bool:
		fmt.Fprint(buffer, v)
	case nil:
		buffer.WriteString("null")
	default:
		return fmt.Errorf("unsupported JSON value %T", value)
	}

	return nil
}

func writeJSONContainer(buffer *bytes.Buffer, open, close string, entries []func() error, inline bool, depth int) error {
	buffer.WriteString(open)

	if len(entries) == 0 {
		buffer.WriteString(close)
		return nil
	}

	separator := ", "
	indent := ""
	if !inline {
		separator = ","
		indent = "\n" + strings.Repeat("    ", depth+1)
	}

	for i, entry := range entries {
		if i > 0 {
			buffer.WriteString(separator)
		}
		buffer.WriteString(indent)

		if err := entry(); err != nil {
			return err
		}
	}

	if !inline {
		buffer.WriteString("\n" + strings.Repeat("    ", depth))
	}
	buffer.WriteString(close)

	return nil
}

// isInline reports whether a value can be written on one line: scalars, and containers
// decoded from one line whose values are all inline too.
func isInline(value any) bool {
	switch v := value.(type) {
	case *jsonObject:
		if !v.inline {
			return false
		}
		for _, key := range v.keys {
			if !isInline(v.values[key]) {
				return false
			}
		}
	case *jsonArray:
		if !v.inline {
			return false
		}
		for _, item := range v.items {
			if !isInline(item) {
				return false
			}
		}
	}

	return true
}

func writeJSONString(buffer *bytes.Buffer, value string) {
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)

	// Encode adds a newline
	buffer.Truncate(buffer.Len() - 1)
}
