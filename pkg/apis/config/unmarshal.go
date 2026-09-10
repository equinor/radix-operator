package config

import (
	"encoding"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"
)

// encoding/json/v2 ignores encoding.BinaryUnmarshaler, so types like url.URL need it wired up manually.
var BinaryUnmarshaler = json.WithUnmarshalers(json.UnmarshalFromFunc(func(dec *jsontext.Decoder, v any) error {
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

var DurationUnmarshaler = json.WithUnmarshalers(json.UnmarshalFromFunc(func(dec *jsontext.Decoder, v any) error {
	val, ok := v.(*time.Duration)
	if !ok {
		return errors.ErrUnsupported // fall back to the default decoding
	}
	// Read the next token to determine if it's a string or a number
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}

	switch tok.Kind() {
	case '"': // Handle string types (e.g., "5s", "1h30m")
		str := tok.String()
		d, err := time.ParseDuration(str)
		if err != nil {
			return fmt.Errorf("invalid duration string %q: %v", str, err)
		}
		*val = d
		return nil

	case '0': // Handle numeric types (interpreting the number as nanoseconds)
		num, err := tok.Float() // or tok.Int() if it fits
		if err != nil {
			return err
		}
		*val = time.Duration(num)
		return nil

	default:
		return fmt.Errorf("cannot unmarshal JSON kind %c into time.Duration", tok.Kind())
	}
}))

var DurationMarshaller = json.WithMarshalers(
	json.MarshalToFunc(func(enc *jsontext.Encoder, val time.Duration) error {
		return json.MarshalEncode(enc, val.String())
	}),
)
