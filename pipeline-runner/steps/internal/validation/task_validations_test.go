package validation_test

import (
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/validation"
	"testing"

	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	operatorDefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/stretchr/testify/assert"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Spec struct {
	name          string
	task          pipelinev1.Task
	expecedErrors []error
}

func TestValidateTask(t *testing.T) {
	specs := []Spec{
		{
			name: "invalid task",
			task: pipelinev1.Task{
				ObjectMeta: v1.ObjectMeta{Name: "Test Task"},
				Spec:       pipelinev1.TaskSpec{},
			},
			expecedErrors: []error{validation.ErrEmptyStepList},
		},
		{
			name: "valid task",
			task: pipelinev1.Task{
				ObjectMeta: v1.ObjectMeta{Name: "Test Task"},
				Spec: pipelinev1.TaskSpec{
					Steps: []pipelinev1.Step{{}},
				},
			},
			expecedErrors: []error{},
		},
		{
			name: "no secrets allowed",
			task: pipelinev1.Task{
				ObjectMeta: v1.ObjectMeta{Name: "Test Task"},
				Spec: pipelinev1.TaskSpec{
					Steps: []pipelinev1.Step{{}},
					Volumes: []corev1.Volume{{
						Name: "testing-secret-mount",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "test-illegal-secret",
							},
						},
					}},
				},
			},
			expecedErrors: []error{validation.ErrSecretReferenceNotAllowed},
		},
		{
			name: "special radix volume allowed",
			task: pipelinev1.Task{
				ObjectMeta: v1.ObjectMeta{Name: "Test Task"},
				Spec: pipelinev1.TaskSpec{
					Steps: []pipelinev1.Step{{}},
					Volumes: []corev1.Volume{{
						Name: pipelineDefaults.SubstitutionRadixGitDeployKeyTarget,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: operatorDefaults.GitPrivateKeySecretName,
							},
						},
					}},
				},
			},
			expecedErrors: []error{},
		},
		{
			name: "no radix volume allowed",
			task: pipelinev1.Task{
				ObjectMeta: v1.ObjectMeta{Name: "Test Task"},
				Spec: pipelinev1.TaskSpec{
					Steps: []pipelinev1.Step{{}},
					Volumes: []corev1.Volume{{
						Name: "radix-hello-world",
					}},
				},
			},
			expecedErrors: []error{validation.ErrRadixVolumeNameNotAllowed},
		},
		{
			name: "host path is not allowed",
			task: pipelinev1.Task{
				ObjectMeta: v1.ObjectMeta{Name: "Test Task"},
				Spec: pipelinev1.TaskSpec{
					Steps: []pipelinev1.Step{{}},
					Volumes: []corev1.Volume{{
						Name: "test-host-path",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/tmp",
							},
						},
					}},
				},
			},
			expecedErrors: []error{validation.ErrHostPathNotAllowed},
		},
		{
			name: "Test allowed Pipeline labels and annotatoins",
			task: pipelinev1.Task{
				ObjectMeta: v1.ObjectMeta{
					Name: "Test Task",
					Labels: map[string]string{
						"azure.workload.identity/use": "true",
					},
					Annotations: map[string]string{
						"azure.workload.identity/skip-containers": "skip-id",
					},
				},
				Spec: pipelinev1.TaskSpec{
					Steps: []pipelinev1.Step{{}},
				},
			},
			expecedErrors: []error{},
		},
		{
			name: "Test illegal Pipeline labels and annotatoins",
			task: pipelinev1.Task{
				ObjectMeta: v1.ObjectMeta{
					Name: "Test Task",
					Labels: map[string]string{
						"ILLEGAL_LABEL": "true",
					},
					Annotations: map[string]string{
						"ILLEGAL_ANNOTATION": "true",
					},
				},
				Spec: pipelinev1.TaskSpec{
					Steps: []pipelinev1.Step{{}},
				},
			},
			expecedErrors: []error{validation.ErrIllegalTaskLabel, validation.ErrIllegalTaskAnnotation},
		},
		{
			name: "collection of errors",
			task: pipelinev1.Task{
				ObjectMeta: v1.ObjectMeta{
					Name: "Test Task",
					Labels: map[string]string{
						"ILLEGAL_LABEL": "true",
					},
					Annotations: map[string]string{
						"ILLEGAL_ANNOTATION": "true",
					}},
				Spec: pipelinev1.TaskSpec{
					Volumes: []corev1.Volume{
						{
							Name: "test-host-path",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/tmp",
								},
							},
						},
						{
							Name: "radix-hello-world",
						},
						{
							Name: "testing-secret-mount",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "test-illegal-secret",
								},
							},
						},
					},
				},
			},
			expecedErrors: []error{
				validation.ErrEmptyStepList,
				validation.ErrHostPathNotAllowed,
				validation.ErrRadixVolumeNameNotAllowed,
				validation.ErrSecretReferenceNotAllowed,
				validation.ErrIllegalTaskAnnotation,
				validation.ErrIllegalTaskLabel,
			},
		},
	}

	for _, spec := range specs {
		spec := spec
		t.Run(spec.name, func(t *testing.T) {
			t.Parallel()

			err := validation.ValidateTask(&spec.task)

			if len(spec.expecedErrors) == 0 {
				assert.NoError(t, err)
			} else {
				for _, expected := range spec.expecedErrors {
					assert.ErrorIs(t, err, expected)
				}
			}

		})
	}

}
