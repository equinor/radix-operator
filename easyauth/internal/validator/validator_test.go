package validator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const annotation = "radix.equinor.com/easyauth"

func newRA(annotations map[string]string) *radixv1.RadixApplication {
	return &radixv1.RadixApplication{Name: "app", Namespace: "app-app", Annotations: annotations}
}

func TestValidate(t *testing.T) {
	tests := map[string]struct {
		enforced    bool
		annotations map[string]string
		wantErr     bool
	}{
		"not enforced, missing annotation":      {enforced: false, annotations: nil, wantErr: false},
		"enforced, missing annotation":          {enforced: true, annotations: nil, wantErr: true},
		"enforced, other annotation":            {enforced: true, annotations: map[string]string{"other": "x"}, wantErr: true},
		"enforced, annotation present":          {enforced: true, annotations: map[string]string{annotation: "true"}, wantErr: false},
		"enforced, annotation with empty value": {enforced: true, annotations: map[string]string{annotation: ""}, wantErr: false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			v := &RadixApplicationValidator{Enforced: tt.enforced, Annotation: annotation}
			ra := newRA(tt.annotations)

			_, createErr := v.ValidateCreate(context.Background(), ra)
			_, updateErr := v.ValidateUpdate(context.Background(), newRA(nil), ra)
			if tt.wantErr {
				assert.ErrorContains(t, createErr, annotation)
				assert.ErrorContains(t, updateErr, annotation)
			} else {
				assert.NoError(t, createErr)
				assert.NoError(t, updateErr)
			}
		})
	}
}

func TestValidateDelete_NeverRejects(t *testing.T) {
	v := &RadixApplicationValidator{Enforced: true, Annotation: annotation}
	_, err := v.ValidateDelete(context.Background(), newRA(nil))
	assert.NoError(t, err)
}
