package configcodec

import (
	"encoding"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"time"
)

var marshalers = json.WithMarshalers(json.JoinMarshalers(
	binaryMarshaler,
	durationMarshaler,
))

var durationMarshaler = json.MarshalToFunc(func(enc *jsontext.Encoder, val time.Duration) error {
	return json.MarshalEncode(enc, val.String())
})

var binaryMarshaler = json.MarshalToFunc(func(enc *jsontext.Encoder, v any) error {
	marshaler, ok := v.(encoding.BinaryMarshaler)
	if !ok {
		return errors.ErrUnsupported // fall back to the default encoding
	}
	raw, err := marshaler.MarshalBinary()
	if err != nil {
		return err
	}
	return json.MarshalEncode(enc, string(raw))
})
