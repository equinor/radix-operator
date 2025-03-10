package validation

import "github.com/pkg/errors"

var (
	ErrEmptyStepList             = errors.New("step list is empty")
	ErrSecretReferenceNotAllowed = errors.New("references to secrets are not allowed")
	ErrRadixVolumeNameNotAllowed = errors.New("volume name starting with radix are not allowed")
	ErrHostPathNotAllowed        = errors.New("HostPath is not allowed")

	ErrIllegalTaskAnnotation = errors.New("annotation is not allowed")
	ErrIllegalTaskLabel      = errors.New("label is not allowed")
	ErrInvalidTaskLabelValue = errors.New("label value is not allowed")
	ErrSkipStepNotFound      = errors.New("skip step is not found in task")
)
