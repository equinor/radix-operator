package pipelinejob

import "k8s.io/apimachinery/pkg/api/resource"

// Quantity wraps resource.Quantity and implements envconfig.Decoder
type Quantity struct {
	resource.Quantity
}

// Decode implements envconfig.Decoder for parsing Kubernetes resource quantities from environment variables
func (q *Quantity) Decode(value string) error {
	parsed, err := resource.ParseQuantity(value)
	if err != nil {
		return err
	}
	q.Quantity = parsed
	return nil
}

// QuantityPtr creates a *Quantity from a *resource.Quantity, returning nil if input is nil
func QuantityPtr(q *resource.Quantity) *Quantity {
	if q == nil {
		return nil
	}
	return &Quantity{Quantity: *q}
}

func MustParse(value string) *Quantity {
	q := &Quantity{}
	if err := q.Decode(value); err != nil {
		panic(err)
	}
	return q
}
