package mergo

import (
	"reflect"

	"dario.cat/mergo"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ mergo.Transformers = BoolPtrTransformer{}
var _ mergo.Transformers = CombinedTransformer{}
var _ mergo.Transformers = ResourceQuantityTransformer{}

type CombinedTransformer struct {
	Transformers []mergo.Transformers
}

func (ct CombinedTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	for _, t := range ct.Transformers {
		if f := t.Transformer(typ); f != nil {
			return f
		}
	}
	return nil
}

type BoolPtrTransformer struct{}

func (t BoolPtrTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeFor[*bool]() {
		return func(dst, src reflect.Value) error {
			if !src.IsNil() && dst.CanSet() {
				dst.Set(src)
			}
			return nil
		}
	}
	return nil
}

// ResourceQuantityTransformer is a dario.cat/mergo Transformers implementation that handles merging of k8s.io/apimachinery/pkg/api/resource Quantity types
type ResourceQuantityTransformer struct {
}

func (t ResourceQuantityTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeFor[resource.Quantity]() {
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				srcVal := (src.Interface()).(resource.Quantity)
				if !srcVal.IsZero() {
					dst.Set(src)
				}
			}
			return nil
		}
	}
	return nil
}
