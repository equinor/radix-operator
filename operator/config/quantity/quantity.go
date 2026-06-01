package quantity

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
