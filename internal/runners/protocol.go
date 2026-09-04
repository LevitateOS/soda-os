package runners

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// DecodeRequest accepts exactly one small JSON object, rejects duplicate and
// unknown fields, and never logs its contents.
func DecodeRequest(reader io.Reader, destination any) error {
	contents, err := io.ReadAll(io.LimitReader(reader, 1<<20+1))
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	if len(contents) > 1<<20 {
		return errors.New("request exceeds 1 MiB")
	}
	if !utf8.Valid(contents) {
		return errors.New("request must contain valid UTF-8")
	}
	object, err := decodeUniqueObject(contents)
	if err != nil {
		return err
	}
	normalized, err := json.Marshal(object)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(normalized))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	return nil
}

func decodeUniqueObject(contents []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	if err := requireObject(decoder); err != nil {
		return nil, err
	}
	object, err := decodeFields(decoder)
	if err != nil {
		return nil, err
	}
	if err = finishObject(decoder); err != nil {
		return nil, err
	}
	return object, nil
}

func requireObject(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return errors.New("request must be one JSON object")
	}
	return nil
}

func decodeFields(decoder *json.Decoder) (map[string]json.RawMessage, error) {
	object := map[string]json.RawMessage{}
	for decoder.More() {
		field, err := decodeFieldName(decoder, object)
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode request field %q: %w", field, err)
		}
		object[field] = value
	}
	return object, nil
}

func decodeFieldName(decoder *json.Decoder, object map[string]json.RawMessage) (string, error) {
	token, err := decoder.Token()
	if err != nil {
		return "", fmt.Errorf("decode request: %w", err)
	}
	field, valid := token.(string)
	if !valid {
		return "", errors.New("request field name must be a string")
	}
	if _, duplicate := object[field]; duplicate {
		return "", fmt.Errorf("duplicate request field %q", field)
	}
	return field, nil
}

func finishObject(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '}' {
		return errors.New("request object is not closed")
	}
	if _, err = decoder.Token(); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	return errors.New("request must contain exactly one JSON object")
}
