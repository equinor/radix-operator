package validator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const (
	annotation                  = "easyauth.radix.equinor.com"
	allowedIdentitiesAnnotation = "easyauth.radix.equinor.com/allowed-identities"
)

func newRA(annotations map[string]string) *radixv1.RadixApplication {
	return &radixv1.RadixApplication{Name: "app", Namespace: "app-app", Annotations: annotations}
}

func TestValidate(t *testing.T) {
	tests := map[string]struct {
		enforced      bool
		defaultGroups []string
		annotations   map[string]string
		wantErr       string
		wantWarning   bool
	}{
		"not enforced, without easyauth annotation should pass":               {enforced: false, annotations: nil},
		"enforced, without easyauth annotation should fail":                   {enforced: true, annotations: nil, wantErr: annotation},
		"enforced, with only other annotations should fail":                   {enforced: true, annotations: map[string]string{"other": "x"}, wantErr: annotation},
		"enforced, easyauth annotation with a value should fail":              {enforced: true, defaultGroups: []string{"g"}, annotations: map[string]string{annotation: "true"}, wantErr: annotation},
		"not enforced, easyauth annotation with a value should fail":          {enforced: false, defaultGroups: []string{"g"}, annotations: map[string]string{annotation: "true"}, wantErr: annotation},
		"enforced, empty easyauth annotation with default groups should pass": {enforced: true, defaultGroups: []string{"g"}, annotations: map[string]string{annotation: ""}},
		"no default groups and no allowed identities annotation should fail": {
			annotations: map[string]string{annotation: ""},
			wantErr:     allowedIdentitiesAnnotation,
		},
		"no default groups and blank allowed identities annotation should fail": {
			annotations: map[string]string{annotation: "", allowedIdentitiesAnnotation: " "},
			wantErr:     allowedIdentitiesAnnotation,
		},
		"no default groups and allowed identities annotation should pass": {
			annotations: map[string]string{annotation: "", allowedIdentitiesAnnotation: "group-a,group-b"},
		},
		"allowed identities annotation with spaces around a group should fail": {
			annotations: map[string]string{annotation: "", allowedIdentitiesAnnotation: "group-a, group-b"},
			wantErr:     allowedIdentitiesAnnotation,
		},
		"allowed identities annotation with trailing comma should fail": {
			annotations: map[string]string{annotation: "", allowedIdentitiesAnnotation: "group-a,"},
			wantErr:     allowedIdentitiesAnnotation,
		},
		"default groups and allowed identities annotation should pass": {
			defaultGroups: []string{"g"},
			annotations:   map[string]string{annotation: "", allowedIdentitiesAnnotation: "group-a"},
		},
		"allowed identities annotation with empty group should fail": {
			defaultGroups: []string{"g"},
			annotations:   map[string]string{annotation: "", allowedIdentitiesAnnotation: "group-a,,group-b"},
			wantErr:       allowedIdentitiesAnnotation,
		},
		"allowed identities annotation without easyauth annotation should fail": {
			annotations: map[string]string{allowedIdentitiesAnnotation: "group-a"},
			wantErr:     annotation,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			v := NewRadixApplicationValidator(tt.defaultGroups, tt.enforced)
			ra := newRA(tt.annotations)

			createWarnings, createErr := v.ValidateCreate(context.Background(), ra)
			updateWarnings, updateErr := v.ValidateUpdate(context.Background(), newRA(nil), ra)
			if tt.wantErr != "" {
				assert.ErrorContains(t, createErr, tt.wantErr)
				assert.ErrorContains(t, updateErr, tt.wantErr)
			} else {
				assert.NoError(t, createErr)
				assert.NoError(t, updateErr)
			}
			if tt.wantWarning {
				assert.NotEmpty(t, createWarnings)
				assert.NotEmpty(t, updateWarnings)
			} else {
				assert.Empty(t, createWarnings)
				assert.Empty(t, updateWarnings)
			}
		})
	}
}

func TestValidateDelete_NeverRejects(t *testing.T) {
	v := NewRadixApplicationValidator(nil, true)
	_, err := v.ValidateDelete(context.Background(), newRA(nil))
	assert.NoError(t, err)
}
