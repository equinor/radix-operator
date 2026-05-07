package models

// ObjectStateBuilder Build ObjectState DTOs
type ObjectStateBuilder interface {
	// WithPodState sets the PodState
	WithPodState(*PodState) ObjectStateBuilder
	// Build the ObjectState
	Build() *ObjectState
}

type objectStateBuilder struct {
	podState *PodState
}

// NewObjectStateBuilder Constructor for objectStateBuilder
func NewObjectStateBuilder() ObjectStateBuilder {
	return &objectStateBuilder{}
}

func (b *objectStateBuilder) WithPodState(v *PodState) ObjectStateBuilder {
	b.podState = v
	return b
}

func (b *objectStateBuilder) Build() *ObjectState {
	return &ObjectState{
		Pod: b.podState,
	}
}
