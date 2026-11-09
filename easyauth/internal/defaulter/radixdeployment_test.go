package defaulter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
)

func newRDDefaulter(objs ...client.Object) *RadixDeploymentDefaulter {
	c := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(objs...).Build()
	return NewRadixDeploymentDefaulter(c, annotation)
}

func newRA(appName string, annotations map[string]string) *radixv1.RadixApplication {
	return &radixv1.RadixApplication{Name: appName, Namespace: appName + "-app", Annotations: annotations}
}

func newRD(appName string, annotations map[string]string) *radixv1.RadixDeployment {
	return &radixv1.RadixDeployment{
		Name:        "rd-1",
		Namespace:   appName + "-dev",
		Annotations: annotations,
		Spec:        radixv1.RadixDeploymentSpec{AppName: appName}, //nolint:staticcheck
	}
}

func requestContext(op admissionv1.Operation) context.Context {
	return admission.NewContextWithRequest(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{Operation: op},
	})
}

func TestRadixDeploymentDefault_OnCreate(t *testing.T) {
	tests := map[string]struct {
		ra       *radixv1.RadixApplication
		rd       *radixv1.RadixDeployment
		expected map[string]string
	}{
		"adds annotation when enabled": {
			ra:       newRA("my-app", map[string]string{annotation: "group-a"}),
			rd:       newRD("my-app", nil),
			expected: map[string]string{annotation: "group-a"},
		},
		"overwrites annotation value from RadixApplication": {
			ra:       newRA("my-app", map[string]string{annotation: ""}),
			rd:       newRD("my-app", map[string]string{annotation: "old", "other": "keep"}),
			expected: map[string]string{annotation: "", "other": "keep"},
		},
		"removes annotation when disabled": {
			ra:       newRA("my-app", map[string]string{"other": "ra"}),
			rd:       newRD("my-app", map[string]string{annotation: "", "other": "keep"}),
			expected: map[string]string{"other": "keep"},
		},
		"removes annotation when RadixApplication is not found": {
			rd:       newRD("my-app", map[string]string{annotation: ""}),
			expected: map[string]string{},
		},
		"does nothing without annotations when disabled": {
			ra:       newRA("my-app", nil),
			rd:       newRD("my-app", nil),
			expected: nil,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var objs []client.Object
			if tt.ra != nil {
				objs = append(objs, tt.ra)
			}
			objs = append(objs, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name:   tt.rd.Namespace,
				Labels: map[string]string{kube.RadixAppLabel: "my-app"},
			}})
			d := newRDDefaulter(objs...)
			require.NoError(t, d.Default(requestContext(admissionv1.Create), tt.rd))
			assert.Equal(t, tt.expected, tt.rd.Annotations)
		})
	}
}

func TestRadixDeploymentDefault_FailsWithoutAppName(t *testing.T) {
	rd := newRD("my-app", nil)
	rd.Spec.AppName = "" //nolint:staticcheck

	d := newRDDefaulter()
	assert.Error(t, d.Default(requestContext(admissionv1.Create), rd))
}

func TestRadixDeploymentDefault_IgnoresUpdate(t *testing.T) {
	tests := map[string]struct {
		ra *radixv1.RadixApplication
		rd *radixv1.RadixDeployment
	}{
		"keeps annotation when disabled": {
			ra: newRA("my-app", nil),
			rd: newRD("my-app", map[string]string{annotation: ""}),
		},
		"does not add annotation when enabled": {
			ra: newRA("my-app", map[string]string{annotation: ""}),
			rd: newRD("my-app", nil),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			expected := tt.rd.DeepCopy()
			d := newRDDefaulter(tt.ra)
			require.NoError(t, d.Default(requestContext(admissionv1.Update), tt.rd))
			assert.Equal(t, expected, tt.rd)
		})
	}
}
