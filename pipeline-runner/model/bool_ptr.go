package model

import "strconv"

// BoolPtr Nillable bool type
type BoolPtr struct {
	value *bool
}

// String returns the string representation of the nillable boolean
func (b *BoolPtr) String() string {
	if b.value == nil {
		return "nil"
	}
	return strconv.FormatBool(*b.value)
}

// Set a new value
func (b *BoolPtr) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	b.value = &v
	return nil
}

// Get returns the value of the nillable boolean
func (b *BoolPtr) Get() *bool {
	return b.value
}

// Type returns the type of the nillable boolean
func (b *BoolPtr) Type() string {
	return "bool"
}
