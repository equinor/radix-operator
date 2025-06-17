package v1

import (
	"encoding/json"

	"github.com/oklog/ulid/v2"
)

// ULID uses the oklog/ulid package to represent a ULID (Universally Unique Lexicographically Sortable Identifier).
// +kubebuilder:validation:Type:=string
// +kubebuilder:validation:Format:=ulid
// +kubebuilder:validation:Pattern=`^[0-7][0-9A-HJKMNP-TV-Za-hjkmnp-tv-z]{25}$`
type ULID struct {
	ulid.ULID
}

func (t *ULID) UnmarshalJSON(b []byte) error {
	if len(b) == 4 && string(b) == "null" {
		*t = ULID{ulid.Zero}
		return nil
	}

	var str string
	err := json.Unmarshal(b, &str)
	if err != nil {
		return err
	}

	pt, err := ulid.ParseStrict(str)
	if err != nil {
		return err
	}

	*t = ULID{pt}
	return nil
}

func (t ULID) MarshalJSON() ([]byte, error) {

	if t.IsZero() {
		return []byte("null"), nil
	}

	return json.Marshal(t.String())
}
