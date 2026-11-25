package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// jsonObject is a JSON object that keeps its key order, so config files edited by
// ngcheck init only change where something was added.
type jsonObject struct {
	keys   []string
	values map[string]any
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

// decodeJSON parses a document into *jsonObject, []any, string, json.Number, bool and nil values.
func decodeJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	value, err := decodeJSONValue(decoder)
	if err != nil {
		return nil, err
	}

	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected content after the JSON document")
	}

	return value, nil
}

func decodeJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	delimiter, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}

	switch delimiter {
	case '{':
		object := newJSONObject()

		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}

			value, err := decodeJSONValue(decoder)
			if err != nil {
				return nil, err
			}

			object.set(keyToken.(string), value)
		}

		_, err = decoder.Token()

		return object, err
	case '[':
		array := []any{}

		for decoder.More() {
			value, err := decodeJSONValue(decoder)
			if err != nil {
				return nil, err
			}

			array = append(array, value)
		}

		_, err = decoder.Token()

		return array, err
	default:
		return nil, fmt.Errorf("unexpected %q", delimiter)
	}
}

// encodeJSON writes a document with 4-space indentation, the CaptainHook and Composer style.
func encodeJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer

	if err := writeJSONValue(&buffer, value, 0); err != nil {
		return nil, err
	}

	buffer.WriteByte('\n')

	return buffer.Bytes(), nil
}

func writeJSONValue(buffer *bytes.Buffer, value any, depth int) error {
	indent := strings.Repeat("    ", depth+1)
	closingIndent := strings.Repeat("    ", depth)

	switch v := value.(type) {
	case *jsonObject:
		if len(v.keys) == 0 {
			buffer.WriteString("{}")
			return nil
		}

		buffer.WriteString("{\n")
		for i, key := range v.keys {
			buffer.WriteString(indent)
			writeJSONString(buffer, key)
			buffer.WriteString(": ")

			if err := writeJSONValue(buffer, v.values[key], depth+1); err != nil {
				return err
			}

			if i < len(v.keys)-1 {
				buffer.WriteByte(',')
			}
			buffer.WriteByte('\n')
		}
		buffer.WriteString(closingIndent + "}")
	case []any:
		if len(v) == 0 {
			buffer.WriteString("[]")
			return nil
		}

		buffer.WriteString("[\n")
		for i, item := range v {
			buffer.WriteString(indent)

			if err := writeJSONValue(buffer, item, depth+1); err != nil {
				return err
			}

			if i < len(v)-1 {
				buffer.WriteByte(',')
			}
			buffer.WriteByte('\n')
		}
		buffer.WriteString(closingIndent + "]")
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

func writeJSONString(buffer *bytes.Buffer, value string) {
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)

	// Encode adds a newline
	buffer.Truncate(buffer.Len() - 1)
}
