package models

import (
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
)

// EventBuilder Build Event DTOs
type EventBuilder interface {
	WithKubernetesEvent(v1.Event) EventBuilder
	WithLastTimestamp(time.Time) EventBuilder
	WithInvolvedObjectKind(string) EventBuilder
	WithInvolvedObjectNamespace(string) EventBuilder
	WithInvolvedObjectName(string) EventBuilder
	WithInvolvedObjectState(*ObjectState) EventBuilder
	WithType(string) EventBuilder
	WithReason(string) EventBuilder
	WithMessage(string) EventBuilder
	Build() *Event
}

type eventBuilder struct {
	lastTimestamp           time.Time
	involvedObjectKind      string
	involvedObjectNamespace string
	involvedObjectName      string
	involvedObjectState     *ObjectState
	eventType               string
	reason                  string
	message                 string
}

// NewEventBuilder Constructor for eventBuilder
func NewEventBuilder() EventBuilder {
	return &eventBuilder{}
}

func (eb *eventBuilder) WithKubernetesEvent(v v1.Event) EventBuilder {
	if !v.LastTimestamp.IsZero() {
		eb.WithLastTimestamp(v.LastTimestamp.Time)
	} else {
		eb.WithLastTimestamp(v.EventTime.Time)
	}
	eb.WithInvolvedObjectKind(v.InvolvedObject.Kind)
	eb.WithInvolvedObjectNamespace(v.InvolvedObject.Namespace)
	eb.WithInvolvedObjectName(v.InvolvedObject.Name)
	eb.WithType(v.Type)
	eb.WithReason(v.Reason)
	eb.WithMessage(v.Message)
	return eb
}

func (eb *eventBuilder) WithLastTimestamp(v time.Time) EventBuilder {
	eb.lastTimestamp = v
	return eb
}

func (eb *eventBuilder) WithInvolvedObjectKind(v string) EventBuilder {
	eb.involvedObjectKind = v
	return eb
}

func (eb *eventBuilder) WithInvolvedObjectNamespace(v string) EventBuilder {
	eb.involvedObjectNamespace = v
	return eb
}

func (eb *eventBuilder) WithInvolvedObjectName(v string) EventBuilder {
	eb.involvedObjectName = v
	return eb
}

func (eb *eventBuilder) WithInvolvedObjectState(v *ObjectState) EventBuilder {
	eb.involvedObjectState = v
	return eb
}

func (eb *eventBuilder) WithType(v string) EventBuilder {
	eb.eventType = v
	return eb
}

func (eb *eventBuilder) WithReason(v string) EventBuilder {
	eb.reason = v
	return eb
}

func (eb *eventBuilder) WithMessage(v string) EventBuilder {
	eb.message = v
	return eb
}

func (eb *eventBuilder) Build() *Event {
	return &Event{
		LastTimestamp:           eb.lastTimestamp,
		InvolvedObjectKind:      eb.involvedObjectKind,
		InvolvedObjectNamespace: eb.involvedObjectNamespace,
		InvolvedObjectName:      eb.involvedObjectName,
		InvolvedObjectState:     eb.involvedObjectState,
		Type:                    eb.eventType,
		Reason:                  eb.reason,
		Message:                 strings.ReplaceAll(eb.message, `\"`, "'"),
	}
}
