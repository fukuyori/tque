// Package strictjson rejects duplicate members, nulls and trailing JSON values.
package strictjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"
)

func Decode(data []byte, target any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("invalid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := value(d); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(target)
}

func value(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("null is not allowed")
	}
	if c, ok := t.(json.Delim); ok {
		switch c {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return err
				}
				key, ok := k.(string)
				if !ok || seen[key] {
					return fmt.Errorf("duplicate or invalid member %v", k)
				}
				seen[key] = true
				if err := value(d); err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}
			}
		case '[':
			for d.More() {
				if err := value(d); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unexpected delimiter %s", c)
		}
		_, err = d.Token()
	}
	return err
}

func Required(data []byte, names ...string) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	for _, name := range names {
		if _, ok := obj[name]; !ok {
			return fmt.Errorf("missing %s", name)
		}
	}
	return nil
}

func Allowed(data []byte, names ...string) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, name := range names {
		allowed[name] = true
	}
	for key := range obj {
		if !allowed[key] {
			return fmt.Errorf("unknown member %q", key)
		}
	}
	return nil
}
