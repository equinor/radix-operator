package e2e

import (
	"strings"
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestRadixDeploymentKubebuilderValidations tests that kubebuilder validation rules are applied
func TestRadixDeploymentKubebuilderValidations(t *testing.T) {
	c := getClient(t)

	t.Run("AppName validation applied", func(t *testing.T) {
		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec:       v1.RadixDeploymentSpec{AppName: "myapp", Environment: "dev"},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid appName should be accepted")

		// Invalid case - violates pattern
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec:       v1.RadixDeploymentSpec{AppName: "MyApp", Environment: "dev"},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid appName should be rejected")
	})

	t.Run("Environment validation applied", func(t *testing.T) {
		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec:       v1.RadixDeploymentSpec{AppName: "myapp", Environment: "dev"},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid environment should be accepted")

		// Invalid case - violates pattern
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec:       v1.RadixDeploymentSpec{AppName: "myapp", Environment: "Production"},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid environment should be rejected")
	})

	t.Run("Component Name validation applied", func(t *testing.T) {
		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Components:  []v1.RadixDeployComponent{{Name: "web", Image: "nginx"}},
			},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid component name should be accepted")

		// Invalid case - exceeds max length
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Components:  []v1.RadixDeployComponent{{Name: strings.Repeat("a", 51), Image: "nginx"}},
			},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid component name should be rejected")
	})

	t.Run("Component Image validation applied", func(t *testing.T) {
		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Components:  []v1.RadixDeployComponent{{Name: "web", Image: "nginx"}},
			},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid image should be accepted")

		// Invalid case - empty
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Components:  []v1.RadixDeployComponent{{Name: "web", Image: ""}},
			},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid image should be rejected")
	})

	t.Run("Component Replicas validation applied", func(t *testing.T) {
		intPtr := func(i int) *int { return &i }

		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Components:  []v1.RadixDeployComponent{{Name: "web", Image: "nginx", Replicas: intPtr(3)}},
			},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid replicas should be accepted")

		// Invalid case - exceeds maximum
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Components:  []v1.RadixDeployComponent{{Name: "web", Image: "nginx", Replicas: intPtr(65)}},
			},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid replicas should be rejected")
	})

	t.Run("Component ReplicasOverride validation applied", func(t *testing.T) {
		intPtr := func(i int) *int { return &i }

		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Components:  []v1.RadixDeployComponent{{Name: "web", Image: "nginx", ReplicasOverride: intPtr(5)}},
			},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid replicasOverride should be accepted")

		// Invalid case - negative
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Components:  []v1.RadixDeployComponent{{Name: "web", Image: "nginx", ReplicasOverride: intPtr(-1)}},
			},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid replicasOverride should be rejected")
	})

	t.Run("Component RunAsUser validation applied", func(t *testing.T) {
		int64Ptr := func(i int64) *int64 { return &i }

		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Components:  []v1.RadixDeployComponent{{Name: "web", Image: "nginx", RunAsUser: int64Ptr(1000)}},
			},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid runAsUser should be accepted")

		// Invalid case - below minimum
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Components:  []v1.RadixDeployComponent{{Name: "web", Image: "nginx", RunAsUser: int64Ptr(0)}},
			},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid runAsUser should be rejected")
	})

	t.Run("ExternalDNS FQDN validation applied", func(t *testing.T) {
		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Components: []v1.RadixDeployComponent{{
					Name:  "web",
					Image: "nginx",
					ExternalDNS: []v1.RadixDeployExternalDNS{
						{FQDN: "app.example.com", UseCertificateAutomation: false},
					},
				}},
			},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid FQDN should be accepted")

		// Invalid case - too short
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Components: []v1.RadixDeployComponent{{
					Name:  "web",
					Image: "nginx",
					ExternalDNS: []v1.RadixDeployExternalDNS{
						{FQDN: "abc", UseCertificateAutomation: false},
					},
				}},
			},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid FQDN should be rejected")
	})

	t.Run("Job Name validation applied", func(t *testing.T) {
		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Jobs:        []v1.RadixDeployJobComponent{{Name: "compute", Image: "python", SchedulerPort: 8080}},
			},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid job name should be accepted")

		// Invalid case - violates pattern
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Jobs:        []v1.RadixDeployJobComponent{{Name: "Compute", Image: "python", SchedulerPort: 8080}},
			},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid job name should be rejected")
	})

	t.Run("Job Image validation applied", func(t *testing.T) {
		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Jobs:        []v1.RadixDeployJobComponent{{Name: "compute", Image: "python", SchedulerPort: 8080}},
			},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid job image should be accepted")

		// Invalid case - empty
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Jobs:        []v1.RadixDeployJobComponent{{Name: "compute", Image: "", SchedulerPort: 8080}},
			},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid job image should be rejected")
	})

	t.Run("Job SchedulerPort validation applied", func(t *testing.T) {
		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Jobs:        []v1.RadixDeployJobComponent{{Name: "compute", Image: "python", SchedulerPort: 8080}},
			},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid schedulerPort should be accepted")

		// Invalid case - below minimum
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Jobs:        []v1.RadixDeployJobComponent{{Name: "compute", Image: "python", SchedulerPort: 1023}},
			},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid schedulerPort should be rejected")
	})

	t.Run("Job TimeLimitSeconds validation applied", func(t *testing.T) {
		int64Ptr := func(i int64) *int64 { return &i }

		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Jobs:        []v1.RadixDeployJobComponent{{Name: "compute", Image: "python", SchedulerPort: 8080, TimeLimitSeconds: int64Ptr(3600)}},
			},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid timeLimitSeconds should be accepted")

		// Invalid case - below minimum
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Jobs:        []v1.RadixDeployJobComponent{{Name: "compute", Image: "python", SchedulerPort: 8080, TimeLimitSeconds: int64Ptr(0)}},
			},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid timeLimitSeconds should be rejected")
	})

	t.Run("Job BackoffLimit validation applied", func(t *testing.T) {
		int32Ptr := func(i int32) *int32 { return &i }

		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Jobs:        []v1.RadixDeployJobComponent{{Name: "compute", Image: "python", SchedulerPort: 8080, BackoffLimit: int32Ptr(3)}},
			},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid backoffLimit should be accepted")

		// Invalid case - below minimum
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Jobs:        []v1.RadixDeployJobComponent{{Name: "compute", Image: "python", SchedulerPort: 8080, BackoffLimit: int32Ptr(-1)}},
			},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid backoffLimit should be rejected")
	})

	t.Run("Job RunAsUser validation applied", func(t *testing.T) {
		int64Ptr := func(i int64) *int64 { return &i }

		// Valid case
		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Jobs:        []v1.RadixDeployJobComponent{{Name: "compute", Image: "python", SchedulerPort: 8080, RunAsUser: int64Ptr(1000)}},
			},
		}
		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Valid runAsUser should be accepted")

		// Invalid case - below minimum
		rd = &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rd", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "dev",
				Jobs:        []v1.RadixDeployJobComponent{{Name: "compute", Image: "python", SchedulerPort: 8080, RunAsUser: int64Ptr(0)}},
			},
		}
		err = c.Create(t.Context(), rd, client.DryRunAll)
		assert.Error(t, err, "Invalid runAsUser should be rejected")
	})

	t.Run("Complete valid RadixDeployment accepted", func(t *testing.T) {
		int64Ptr := func(i int64) *int64 { return &i }
		int32Ptr := func(i int32) *int32 { return &i }
		intPtr := func(i int) *int { return &i }

		rd := &v1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-complete", Namespace: "default"},
			Spec: v1.RadixDeploymentSpec{
				AppName:     "myapp",
				Environment: "production",
				Components: []v1.RadixDeployComponent{
					{
						Name:             "frontend",
						Image:            "myregistry/frontend:v1.2.3",
						Replicas:         intPtr(3),
						ReplicasOverride: intPtr(5),
						RunAsUser:        int64Ptr(1000),
						PublicPort:       "8080",
						ExternalDNS: []v1.RadixDeployExternalDNS{
							{FQDN: "myapp.example.com", UseCertificateAutomation: true},
						},
					},
				},
				Jobs: []v1.RadixDeployJobComponent{
					{
						Name:             "processor",
						Image:            "myregistry/processor:v1.0.0",
						SchedulerPort:    8080,
						TimeLimitSeconds: int64Ptr(3600),
						BackoffLimit:     int32Ptr(3),
						RunAsUser:        int64Ptr(1002),
					},
				},
				ImagePullSecrets: []corev1.LocalObjectReference{{Name: "my-secret"}},
			},
		}

		err := c.Create(t.Context(), rd, client.DryRunAll)
		assert.NoError(t, err, "Complete valid RadixDeployment should be accepted")
	})
}
