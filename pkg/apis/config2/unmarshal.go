package config2

import (
	"encoding"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
)

// encoding/json/v2 ignores encoding.BinaryUnmarshaler, so types like url.URL need it wired up manually.
var binaryUnmarshaler = json.WithUnmarshalers(json.UnmarshalFromFunc(func(dec *jsontext.Decoder, v any) error {
	unmarshaler, ok := v.(encoding.BinaryUnmarshaler)
	if !ok {
		return errors.ErrUnsupported // fall back to the default decoding
	}
	var raw string
	if err := json.UnmarshalDecode(dec, &raw); err != nil {
		return err
	}
	return unmarshaler.UnmarshalBinary([]byte(raw))
}))
