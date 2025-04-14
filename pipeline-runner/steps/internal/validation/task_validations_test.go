package validation_test

import (
	"testing"

	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/validation"
	operatorDefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/stretchr/testify/assert"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Spec struct {
	name           string
	task           pipelinev1.Task
	expectedErrors []error
}

func TestValidateTask(t *testing.T) {
	specs := []Spec{
		{
			name: "invalid task",
			task: pipelinev1.Task{
				ObjectMeta: v1.ObjectMeta{Name: "Test Task"},
				Spec:       pipelinev1.TaskSpec{},
			},
			expectedErrors: []error{validation.ErrEmptyStepList},
		},
		{
			name: "valid task",
			task: pipelinev1.Task{
				ObjectMeta: v1.ObjectMeta{Name: "Test Task"},
				Spec: pipelinev1.TaskSpec{
					Steps: []pipelinev1.Step{{}},
				},
			},
			expectedErrors: []error{},
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
			expectedErrors: []error{validation.ErrSecretReferenceNotAllowed},
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
			expectedErrors: []error{},
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
			expectedErrors: []error{validation.ErrRadixVolumeNameNotAllowed},
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
			expectedErrors: []error{validation.ErrHostPathNotAllowed},
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
			expectedErrors: []error{},
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
			expectedErrors: []error{validation.ErrIllegalTaskLabel, validation.ErrIllegalTaskAnnotation},
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
			expectedErrors: []error{
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

			if len(spec.expectedErrors) == 0 {
				assert.NoError(t, err)
			} else {
				for _, expected := range spec.expectedErrors {
					assert.ErrorIs(t, err, expected)
				}
			}

		})
	}

}
