package preparepipeline_test

import (
	"context"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/ownerreferences"
	prepareInternal "github.com/equinor/radix-operator/pipeline-runner/steps/preparepipeline/internal"
	"github.com/equinor/radix-operator/pipeline-runner/utils/git"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/golang/mock/gomock"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/labels"
	internalTest "github.com/equinor/radix-operator/pipeline-runner/steps/internal/test"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/validation"
	"github.com/equinor/radix-operator/pipeline-runner/steps/preparepipeline"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	operatorDefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektonfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	sampleAppRadixConfigFileName = "/radixconfig.yaml"
	sampleAppWorkspace           = "../internal/test/testdata"
)

func Test_RunTestSuite(t *testing.T) {
	suite.Run(t, new(stepTestSuite))
}

type stepTestSuite struct {
	suite.Suite
	kubeClient  *kubefake.Clientset
	radixClient *radixfake.Clientset
	kedaClient  *kedafake.Clientset
	promClient  *prometheusfake.Clientset
	kubeUtil    *kube.Kube
	tknClient   tektonclient.Interface
	ctrl        *gomock.Controller
}

func (s *stepTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.tknClient = tektonfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, nil)
	s.ctrl = gomock.NewController(s.T())
}

func (s *stepTestSuite) SetupSubTest() {
	s.SetupTest()
}

type fields struct {
	radixApplicationBuilder utils.ApplicationBuilder
	targetEnvironments      []string
	hash                    string
}

const (
	appName              = "test-app"
	envDev               = "dev"
	branchMain           = "main"
	radixImageTag        = "tag-123"
	radixPipelineJobName = "pipeline-job-123"
	hash                 = "some-hash"
)

type args struct {
	envName   string
	pipeline  *pipelinev1.Pipeline
	tasks     []pipelinev1.Task
	timestamp string
}

func (s *stepTestSuite) Test_pipelineContext_createPipeline() {
	scenarios := map[string]struct {
		fields         fields
		args           args
		wantErr        func(t *testing.T, err error)
		assertScenario func(t *testing.T, step model.Step, pipelineName string)
	}{
		"one default task": {
			fields: fields{
				targetEnvironments: []string{envDev},
				hash:               hash,
			},
			args: args{envName: envDev, pipeline: getTestPipeline(func(pipeline *pipelinev1.Pipeline) {
				pipeline.ObjectMeta.Name = "pipeline1"
				pipeline.Spec.Tasks = []pipelinev1.PipelineTask{{Name: "task1", TaskRef: &pipelinev1.TaskRef{Name: "task1"}}}
			}),
				tasks: []pipelinev1.Task{*getTestTask(func(task *pipelinev1.Task) {
					task.ObjectMeta.Name = "task1"
					task.Spec = pipelinev1.TaskSpec{
						Steps:        []pipelinev1.Step{{Name: "step1"}},
						Sidecars:     []pipelinev1.Sidecar{{Name: "sidecar1"}},
						StepTemplate: &pipelinev1.StepTemplate{Image: "image1"},
					}
				})}, timestamp: "2020-01-01T00:00:00Z"},
			wantErr: func(t *testing.T, err error) {
				assert.Nil(t, err)
			},
			assertScenario: func(t *testing.T, step model.Step, pipelineName string) {
				pipeline, err := step.GetTektonClient().TektonV1().Pipelines(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipelineName, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, pipeline.Spec.Tasks, 1)

				task, err := step.GetTektonClient().TektonV1().Tasks(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipeline.Spec.Tasks[0].TaskRef.Name, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, task.Spec.Steps, 1)
				require.Len(t, task.Spec.Sidecars, 1)

				assert.Equal(t, "task1", task.ObjectMeta.Annotations[operatorDefaults.PipelineTaskNameAnnotation])
				assert.Equal(t, "step1", task.Spec.Steps[0].Name)
				assert.Equal(t, "sidecar1", task.Spec.Sidecars[0].Name)
				assert.Equal(t, "image1", task.Spec.StepTemplate.Image)
				assert.NotNilf(t, pipeline.ObjectMeta.OwnerReferences, "Expected owner reference to be set")

				require.Len(t, pipeline.ObjectMeta.OwnerReferences, 1)
				assert.Equal(t, "RadixApplication", pipeline.ObjectMeta.OwnerReferences[0].Kind)
				assert.Equal(t, appName, pipeline.ObjectMeta.OwnerReferences[0].Name)
			},
		},
		"set SecurityContexts in task step": {
			fields: fields{
				targetEnvironments: []string{envDev},
			},
			args: args{
				envName: envDev,
				pipeline: getTestPipeline(func(pipeline *pipelinev1.Pipeline) {
					pipeline.ObjectMeta.Name = "pipeline1"
					pipeline.Spec.Tasks = []pipelinev1.PipelineTask{{Name: "task1", TaskRef: &pipelinev1.TaskRef{Name: "task1"}}}
				}),
				tasks: []pipelinev1.Task{*getTestTask(func(task *pipelinev1.Task) {
					task.Spec = pipelinev1.TaskSpec{
						Steps: []pipelinev1.Step{
							{
								Name: "step1",
								SecurityContext: &corev1.SecurityContext{
									Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"None"}},
									Privileged:               commonUtils.BoolPtr(true),
									SELinuxOptions:           &corev1.SELinuxOptions{},
									WindowsOptions:           &corev1.WindowsSecurityContextOptions{},
									RunAsUser:                pointers.Ptr(int64(0)),
									RunAsGroup:               pointers.Ptr(int64(0)),
									RunAsNonRoot:             commonUtils.BoolPtr(false),
									AllowPrivilegeEscalation: commonUtils.BoolPtr(true),
									ProcMount:                pointers.Ptr(corev1.ProcMountType("Default")),
									SeccompProfile:           &corev1.SeccompProfile{},
								},
							},
						},
					}
				})}, timestamp: "2020-01-01T00:00:00Z"},
			wantErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			assertScenario: func(t *testing.T, step model.Step, pipelineName string) {
				pipeline, err := step.GetTektonClient().TektonV1().Pipelines(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipelineName, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, pipeline.Spec.Tasks, 1)
				task, err := step.GetTektonClient().TektonV1().Tasks(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipeline.Spec.Tasks[0].TaskRef.Name, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, task.Spec.Steps, 1)
				taskStep := task.Spec.Steps[0]
				assert.NotNil(t, taskStep.SecurityContext)
				assert.Equal(t, commonUtils.BoolPtr(true), taskStep.SecurityContext.RunAsNonRoot)
				assert.Equal(t, commonUtils.BoolPtr(false), taskStep.SecurityContext.Privileged)
				assert.Equal(t, commonUtils.BoolPtr(false), taskStep.SecurityContext.AllowPrivilegeEscalation)
				assert.Nil(t, taskStep.SecurityContext.RunAsUser)
				assert.Nil(t, taskStep.SecurityContext.RunAsGroup)
				assert.Nil(t, taskStep.SecurityContext.WindowsOptions)
				assert.Nil(t, taskStep.SecurityContext.SELinuxOptions)
				assert.NotNil(t, taskStep.SecurityContext.Capabilities)
				assert.Equal(t, []corev1.Capability{"ALL"}, taskStep.SecurityContext.Capabilities.Drop)
			},
		},
		"set SecurityContexts in task sidecar": {
			fields: fields{
				targetEnvironments: []string{envDev},
			},
			args: args{
				envName: envDev,
				pipeline: getTestPipeline(func(pipeline *pipelinev1.Pipeline) {
					pipeline.ObjectMeta.Name = "pipeline1"
					pipeline.Spec.Tasks = []pipelinev1.PipelineTask{{Name: "task1", TaskRef: &pipelinev1.TaskRef{Name: "task1"}}}
				}),
				tasks: []pipelinev1.Task{*getTestTask(func(task *pipelinev1.Task) {
					task.Spec = pipelinev1.TaskSpec{
						Steps: []pipelinev1.Step{{Name: "step1"}},
						Sidecars: []pipelinev1.Sidecar{
							{
								SecurityContext: &corev1.SecurityContext{
									Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"None"}},
									Privileged:               commonUtils.BoolPtr(true),
									SELinuxOptions:           &corev1.SELinuxOptions{},
									WindowsOptions:           &corev1.WindowsSecurityContextOptions{},
									RunAsUser:                pointers.Ptr(int64(0)),
									RunAsGroup:               pointers.Ptr(int64(0)),
									RunAsNonRoot:             commonUtils.BoolPtr(false),
									AllowPrivilegeEscalation: commonUtils.BoolPtr(true),
									ProcMount:                pointers.Ptr(corev1.ProcMountType("Default")),
									SeccompProfile:           &corev1.SeccompProfile{},
								},
							},
						},
					}
				},
				)}, timestamp: "2020-01-01T00:00:00Z"},
			wantErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			assertScenario: func(t *testing.T, step model.Step, pipelineName string) {
				pipeline, err := step.GetTektonClient().TektonV1().Pipelines(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipelineName, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, pipeline.Spec.Tasks, 1)

				task, err := step.GetTektonClient().TektonV1().Tasks(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipeline.Spec.Tasks[0].TaskRef.Name, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, task.Spec.Steps, 1)

				require.Len(t, task.Spec.Sidecars, 1)
				sidecar := task.Spec.Sidecars[0]
				assert.NotNil(t, sidecar.SecurityContext)
				assert.Equal(t, commonUtils.BoolPtr(true), sidecar.SecurityContext.RunAsNonRoot)
				assert.Equal(t, commonUtils.BoolPtr(false), sidecar.SecurityContext.Privileged)
				assert.Equal(t, commonUtils.BoolPtr(false), sidecar.SecurityContext.AllowPrivilegeEscalation)
				assert.Nil(t, sidecar.SecurityContext.RunAsUser)
				assert.Nil(t, sidecar.SecurityContext.RunAsGroup)
				assert.Nil(t, sidecar.SecurityContext.WindowsOptions)
				assert.Nil(t, sidecar.SecurityContext.SELinuxOptions)
				assert.NotNil(t, sidecar.SecurityContext.Capabilities)
				assert.Equal(t, []corev1.Capability{"ALL"}, sidecar.SecurityContext.Capabilities.Drop)
			},
		},
		"set SecurityContexts in task stepTemplate": {
			fields: fields{
				targetEnvironments: []string{envDev},
			},
			args: args{
				envName: envDev,
				pipeline: getTestPipeline(func(pipeline *pipelinev1.Pipeline) {
					pipeline.ObjectMeta.Name = "pipeline1"
					pipeline.Spec.Tasks = []pipelinev1.PipelineTask{{Name: "task1", TaskRef: &pipelinev1.TaskRef{Name: "task1"}}}
				}),
				tasks: []pipelinev1.Task{*getTestTask(func(task *pipelinev1.Task) {
					task.Spec = pipelinev1.TaskSpec{
						Steps: []pipelinev1.Step{{Name: "step1"}},
						StepTemplate: &pipelinev1.StepTemplate{
							SecurityContext: &corev1.SecurityContext{
								Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"None"}},
								Privileged:               commonUtils.BoolPtr(true),
								SELinuxOptions:           &corev1.SELinuxOptions{},
								WindowsOptions:           &corev1.WindowsSecurityContextOptions{},
								RunAsUser:                pointers.Ptr(int64(0)),
								RunAsGroup:               pointers.Ptr(int64(0)),
								RunAsNonRoot:             commonUtils.BoolPtr(false),
								AllowPrivilegeEscalation: commonUtils.BoolPtr(true),
								ProcMount:                pointers.Ptr(corev1.ProcMountType("Default")),
								SeccompProfile:           &corev1.SeccompProfile{},
							},
						},
					}
				})}, timestamp: "2020-01-01T00:00:00Z"},
			wantErr: func(t *testing.T, err error) {
				assert.Nil(t, err)
			},
			assertScenario: func(t *testing.T, step model.Step, pipelineName string) {
				pipeline, err := step.GetTektonClient().TektonV1().Pipelines(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipelineName, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, pipeline.Spec.Tasks, 1)

				task, err := step.GetTektonClient().TektonV1().Tasks(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipeline.Spec.Tasks[0].TaskRef.Name, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, task.Spec.Steps, 1)
				stepTemplate := task.Spec.StepTemplate
				assert.NotNil(t, stepTemplate)
				assert.NotNil(t, stepTemplate.SecurityContext)
				assert.Equal(t, commonUtils.BoolPtr(true), stepTemplate.SecurityContext.RunAsNonRoot)
				assert.Equal(t, commonUtils.BoolPtr(false), stepTemplate.SecurityContext.Privileged)
				assert.Equal(t, commonUtils.BoolPtr(false), stepTemplate.SecurityContext.AllowPrivilegeEscalation)
				assert.Nil(t, stepTemplate.SecurityContext.RunAsUser)
				assert.Nil(t, stepTemplate.SecurityContext.RunAsGroup)
				assert.Nil(t, stepTemplate.SecurityContext.WindowsOptions)
				assert.Nil(t, stepTemplate.SecurityContext.SELinuxOptions)
				assert.NotNil(t, stepTemplate.SecurityContext.Capabilities)
				assert.Equal(t, []corev1.Capability{"ALL"}, stepTemplate.SecurityContext.Capabilities.Drop)
			},
		},
		"allow in the SecurityContext in task step non-root RunAsUser and RunAsGroup": {
			fields: fields{
				targetEnvironments: []string{envDev},
			},
			args: args{
				envName: envDev,
				pipeline: getTestPipeline(func(pipeline *pipelinev1.Pipeline) {
					pipeline.ObjectMeta.Name = "pipeline1"
					pipeline.Spec.Tasks = []pipelinev1.PipelineTask{{Name: "task1", TaskRef: &pipelinev1.TaskRef{Name: "task1"}}}
				}),
				tasks: []pipelinev1.Task{*getTestTask(func(task *pipelinev1.Task) {
					task.Spec = pipelinev1.TaskSpec{
						Steps: []pipelinev1.Step{
							{
								Name: "step1",
								SecurityContext: &corev1.SecurityContext{
									RunAsUser:  pointers.Ptr(int64(10)),
									RunAsGroup: pointers.Ptr(int64(20)),
								},
							},
						},
					}
				})}, timestamp: "2020-01-01T00:00:00Z"},
			wantErr: func(t *testing.T, err error) {
				assert.Nil(t, err)
			},
			assertScenario: func(t *testing.T, step model.Step, pipelineName string) {
				pipeline, err := step.GetTektonClient().TektonV1().Pipelines(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipelineName, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, pipeline.Spec.Tasks, 1)

				task, err := step.GetTektonClient().TektonV1().Tasks(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipeline.Spec.Tasks[0].TaskRef.Name, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, task.Spec.Steps, 1)

				require.Len(t, task.Spec.Steps, 1)
				taskStep := task.Spec.Steps[0]
				assert.Equal(t, int64(10), *taskStep.SecurityContext.RunAsUser)
				assert.Equal(t, int64(20), *taskStep.SecurityContext.RunAsGroup)
			},
		},
		"allow set SecurityContexts in task sidecar non-root RunAsUser and RunAsGroup": {
			fields: fields{
				targetEnvironments: []string{envDev},
			},
			args: args{
				envName: envDev,
				pipeline: getTestPipeline(func(pipeline *pipelinev1.Pipeline) {
					pipeline.ObjectMeta.Name = "pipeline1"
					pipeline.Spec.Tasks = []pipelinev1.PipelineTask{{Name: "task1", TaskRef: &pipelinev1.TaskRef{Name: "task1"}}}
				}),
				tasks: []pipelinev1.Task{*getTestTask(func(task *pipelinev1.Task) {
					task.Spec = pipelinev1.TaskSpec{
						Steps: []pipelinev1.Step{{Name: "step1"}},
						Sidecars: []pipelinev1.Sidecar{
							{
								SecurityContext: &corev1.SecurityContext{
									RunAsUser:  pointers.Ptr(int64(10)),
									RunAsGroup: pointers.Ptr(int64(20)),
								},
							},
						},
					}
				},
				)}, timestamp: "2020-01-01T00:00:00Z"},
			wantErr: func(t *testing.T, err error) {
				assert.Nil(t, err)
			},
			assertScenario: func(t *testing.T, step model.Step, pipelineName string) {
				pipeline, err := step.GetTektonClient().TektonV1().Pipelines(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipelineName, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, pipeline.Spec.Tasks, 1)

				task, err := step.GetTektonClient().TektonV1().Tasks(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipeline.Spec.Tasks[0].TaskRef.Name, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, task.Spec.Steps, 1)

				require.Len(t, task.Spec.Sidecars, 1)
				sidecar := task.Spec.Sidecars[0]
				assert.Equal(t, int64(10), *sidecar.SecurityContext.RunAsUser)
				assert.Equal(t, int64(20), *sidecar.SecurityContext.RunAsGroup)
			},
		},
		"allow set SecurityContexts in task stepTemplate non-root RunAsUser and RunAsGroup": {
			fields: fields{
				targetEnvironments: []string{envDev},
			},
			args: args{
				envName: envDev,
				pipeline: getTestPipeline(func(pipeline *pipelinev1.Pipeline) {
					pipeline.ObjectMeta.Name = "pipeline1"
					pipeline.Spec.Tasks = []pipelinev1.PipelineTask{{Name: "task1", TaskRef: &pipelinev1.TaskRef{Name: "task1"}}}
				}),
				tasks: []pipelinev1.Task{*getTestTask(func(task *pipelinev1.Task) {
					task.Spec = pipelinev1.TaskSpec{
						Steps: []pipelinev1.Step{{Name: "step1"}},
						StepTemplate: &pipelinev1.StepTemplate{
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  pointers.Ptr(int64(10)),
								RunAsGroup: pointers.Ptr(int64(20)),
							},
						},
					}
				})}, timestamp: "2020-01-01T00:00:00Z"},
			wantErr: func(t *testing.T, err error) {
				assert.Nil(t, err)
			},
			assertScenario: func(t *testing.T, step model.Step, pipelineName string) {
				pipeline, err := step.GetTektonClient().TektonV1().Pipelines(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipelineName, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, pipeline.Spec.Tasks, 1)

				task, err := step.GetTektonClient().TektonV1().Tasks(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipeline.Spec.Tasks[0].TaskRef.Name, metav1.GetOptions{})
				require.NoError(t, err)

				stepTemplate := task.Spec.StepTemplate
				assert.Equal(t, int64(10), *stepTemplate.SecurityContext.RunAsUser)
				assert.Equal(t, int64(20), *stepTemplate.SecurityContext.RunAsGroup)
			},
		},
		"Test sanitizeAzureSkipContainersAnnotation in task stepTemplate": {
			args: args{
				envName: envDev,
				pipeline: getTestPipeline(func(pipeline *pipelinev1.Pipeline) {
					pipeline.ObjectMeta.Name = "pipeline1"
					pipeline.Spec.Tasks = append(pipeline.Spec.Tasks, pipelinev1.PipelineTask{Name: "identity", TaskRef: &pipelinev1.TaskRef{Name: "task1"}})
				}),
				tasks: []pipelinev1.Task{{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "task1",
						Annotations: map[string]string{"azure.workload.identity/skip-containers": "skip-id"},
						Labels:      map[string]string{labels.AzureWorkloadIdentityUse: "true"},
					},
					Spec: pipelinev1.TaskSpec{Steps: []pipelinev1.Step{
						{Name: "get-secret"}, {Name: "skip-id"}},
					}},
				},
			},
			wantErr: func(t *testing.T, err error) {
				assert.Nil(t, err)
			},
			assertScenario: func(t *testing.T, step model.Step, pipelineName string) {
				pipeline, err := step.GetTektonClient().TektonV1().Pipelines(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipelineName, metav1.GetOptions{})
				require.NoError(t, err)
				require.Len(t, pipeline.Spec.Tasks, 1)
				task, err := step.GetTektonClient().TektonV1().Tasks(utils.GetAppNamespace(step.GetAppName())).Get(context.Background(), pipeline.Spec.Tasks[0].TaskRef.Name, metav1.GetOptions{})
				require.NoError(t, err)

				skipContainers := strings.Split(task.Annotations["azure.workload.identity/skip-containers"], ";")
				assert.Len(t, skipContainers, 3)
				assert.Contains(t, skipContainers, "step-skip-id")
				assert.Contains(t, skipContainers, "place-scripts")
				assert.Contains(t, skipContainers, "prepare")
			},
		},
		"Test unknown steps is not allowed in sanitizeAzureSkipContainersAnnotation": {
			args: args{
				envName: envDev,
				pipeline: getTestPipeline(func(pipeline *pipelinev1.Pipeline) {
					pipeline.ObjectMeta.Name = "pipeline1"
					pipeline.Spec.Tasks = append(pipeline.Spec.Tasks, pipelinev1.PipelineTask{Name: "identity", TaskRef: &pipelinev1.TaskRef{Name: "task1"}})
				}),
				tasks: []pipelinev1.Task{{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "task1",
						Annotations: map[string]string{"azure.workload.identity/skip-containers": "skip-id;unknown-step"},
						Labels:      map[string]string{labels.AzureWorkloadIdentityUse: "true"},
					},
					Spec: pipelinev1.TaskSpec{Steps: []pipelinev1.Step{
						{Name: "get-secret"}, {Name: "skip-id"}},
					}},
				},
			},
			wantErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, validation.ErrSkipStepNotFound)
			},
			assertScenario: func(t *testing.T, step model.Step, pipelineName string) {},
		},
		"Test illegal azure WI label value in task": {
			args: args{
				envName: envDev,
				pipeline: getTestPipeline(func(pipeline *pipelinev1.Pipeline) {
					pipeline.ObjectMeta.Name = "pipeline1"
					pipeline.Spec.Tasks = append(pipeline.Spec.Tasks, pipelinev1.PipelineTask{Name: "identity", TaskRef: &pipelinev1.TaskRef{Name: "task1"}})
				}),
				tasks: []pipelinev1.Task{{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "task1",
						Labels: map[string]string{labels.AzureWorkloadIdentityUse: "True"}, // must be lowercase 'true'
					},
					Spec: pipelinev1.TaskSpec{Steps: []pipelinev1.Step{}},
				}},
			},
			wantErr: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, validation.ErrInvalidTaskLabelValue)
			},
			assertScenario: func(t *testing.T, step model.Step, pipelineName string) {},
		},
	}
	for testName, ts := range scenarios {
		s.Run(testName, func() {
			applicationBuilder := ts.fields.radixApplicationBuilder
			if applicationBuilder == nil {
				applicationBuilder = getRadixApplicationBuilder(appName, envDev, branchMain)
			}
			rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
			_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.TODO(), rr, metav1.CreateOptions{})
			s.NoError(err, "Failed to create radix registration. Error %v", err)
			radixPipelineType := v1.Deploy
			pipelineType, err := pipeline.GetPipelineFromName(string(radixPipelineType))
			s.Require().NoError(err, "Failed to get pipeline type. Error %v", err)
			pipelineInfo := &model.PipelineInfo{
				Definition: pipelineType,
				PipelineArguments: model.PipelineArguments{
					AppName:         appName,
					ImageTag:        radixImageTag,
					JobName:         radixPipelineJobName,
					Branch:          branchMain,
					PipelineType:    string(radixPipelineType),
					ToEnvironment:   internalTest.Env1,
					DNSConfig:       &dnsalias.DNSConfig{},
					RadixConfigFile: sampleAppRadixConfigFileName,
					GitWorkspace:    sampleAppWorkspace,
				},
				RadixRegistration: rr,
			}
			buildContext := &model.BuildContext{}
			mockGitRepo := git.NewMockRepository(s.ctrl)
			mockGitRepo.EXPECT().Checkout(gomock.Any()).AnyTimes().Return(nil)
			mockGitRepo.EXPECT().ResolveCommitForReference(gomock.Any()).AnyTimes().Return("anycommitid", nil)
			mockGitRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).AnyTimes().Return(true, nil)
			mockGitRepo.EXPECT().ResolveTagsForCommit(gomock.Any()).AnyTimes().Return(nil, nil)
			mockRadixConfigReader := prepareInternal.NewMockRadixConfigReader(s.ctrl)
			mockRadixConfigReader.EXPECT().Read(pipelineInfo).Return(applicationBuilder.BuildRA(), nil).Times(1)
			mockContextBuilder := prepareInternal.NewMockContextBuilder(s.ctrl)
			mockContextBuilder.EXPECT().GetBuildContext(pipelineInfo, mockGitRepo).Return(buildContext, nil).AnyTimes()
			mockSubPipelineReader := prepareInternal.NewMockSubPipelineReader(s.ctrl)
			mockSubPipelineReader.EXPECT().ReadPipelineAndTasks(pipelineInfo, internalTest.Env1).Return(true, "tekton/pipeline.yaml", ts.args.pipeline, ts.args.tasks, nil).AnyTimes()
			mockOwnerReferenceFactory := ownerreferences.NewMockOwnerReferenceFactory(s.ctrl)
			mockOwnerReferenceFactory.EXPECT().Create().Return(&metav1.OwnerReference{Kind: "RadixApplication", Name: appName}).AnyTimes()
			step := preparepipeline.NewPreparePipelinesStep(
				preparepipeline.WithRadixConfigReader(mockRadixConfigReader),
				preparepipeline.WithSubPipelineReader(mockSubPipelineReader),
				preparepipeline.WithBuildContextBuilder(mockContextBuilder),
				preparepipeline.WithOwnerReferenceFactory(mockOwnerReferenceFactory),
				preparepipeline.WithOpenGitRepoFunc(func(_ string) (git.Repository, error) { return mockGitRepo, nil }),
			)
			ctx := context.Background()
			step.Init(ctx, s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, s.tknClient, rr)
			err = step.Run(ctx, pipelineInfo)
			ts.wantErr(s.T(), err)
			ts.assertScenario(s.T(), step, ts.args.pipeline.ObjectMeta.Name)
		})
	}
}

func (s *stepTestSuite) Test_prepare_webhookEnabled() {
	scenarios := map[string]struct {
		raEnvs               []v1.Environment
		triggeredFromWebhook bool
		expectedTargetEnvs   []string
	}{
		"one env, WebhookEnabled nil, not triggered from webhook": {
			raEnvs: []v1.Environment{{
				Name: internalTest.Env1,
				Build: v1.EnvBuild{
					From:           "main",
					WebhookEnabled: nil,
				},
			}},
			triggeredFromWebhook: false,
			expectedTargetEnvs:   []string{internalTest.Env1},
		},
		"one env, WebhookEnabled true, not triggered from webhook": {
			raEnvs: []v1.Environment{{
				Name: internalTest.Env1,
				Build: v1.EnvBuild{
					From:           "main",
					WebhookEnabled: pointers.Ptr(true),
				},
			}},
			triggeredFromWebhook: false,
			expectedTargetEnvs:   []string{internalTest.Env1},
		},
		"one env, WebhookEnabled false, not triggered from webhook": {
			raEnvs: []v1.Environment{{
				Name: internalTest.Env1,
				Build: v1.EnvBuild{
					From:           "main",
					WebhookEnabled: pointers.Ptr(false),
				},
			},
			},
			triggeredFromWebhook: false,
			expectedTargetEnvs:   []string{internalTest.Env1},
		},
		"one env, WebhookEnabled false, triggered from webhook": {
			raEnvs: []v1.Environment{{
				Name: internalTest.Env1,
				Build: v1.EnvBuild{
					From:           "main",
					WebhookEnabled: pointers.Ptr(false),
				},
			},
			},
			triggeredFromWebhook: true,
			expectedTargetEnvs:   nil,
		},
		"multiple envs, WebhookEnabled nil, not triggered from webhook": {
			raEnvs: []v1.Environment{{
				Name: internalTest.Env1,
				Build: v1.EnvBuild{
					From:           "main",
					WebhookEnabled: nil,
				},
			},
				{
					Name: internalTest.Env2,
					Build: v1.EnvBuild{
						From:           "main",
						WebhookEnabled: nil,
					},
				}},
			triggeredFromWebhook: false,
			expectedTargetEnvs:   []string{internalTest.Env1, internalTest.Env2},
		},
		"multiple envs, WebhookEnabled true, not triggered from webhook": {
			raEnvs: []v1.Environment{{
				Name: internalTest.Env1,
				Build: v1.EnvBuild{
					From:           "main",
					WebhookEnabled: pointers.Ptr(true),
				},
			},
				{
					Name: internalTest.Env2,
					Build: v1.EnvBuild{
						From:           "main",
						WebhookEnabled: nil,
					},
				}},
			triggeredFromWebhook: false,
			expectedTargetEnvs:   []string{internalTest.Env1, internalTest.Env2},
		},
		"multiple envs, WebhookEnabled false, not triggered from webhook": {
			raEnvs: []v1.Environment{{
				Name: internalTest.Env1,
				Build: v1.EnvBuild{
					From:           "main",
					WebhookEnabled: pointers.Ptr(false),
				},
			},
				{
					Name: internalTest.Env2,
					Build: v1.EnvBuild{
						From:           "main",
						WebhookEnabled: nil,
					},
				},
			},
			triggeredFromWebhook: false,
			expectedTargetEnvs:   []string{internalTest.Env1, internalTest.Env2},
		},
		"multiple envs, WebhookEnabled false, triggered from webhook": {
			raEnvs: []v1.Environment{{
				Name: internalTest.Env1,
				Build: v1.EnvBuild{
					From:           "main",
					WebhookEnabled: pointers.Ptr(false),
				},
			},
				{
					Name: internalTest.Env2,
					Build: v1.EnvBuild{
						From:           "main",
						WebhookEnabled: nil,
					},
				},
			},
			triggeredFromWebhook: true,
			expectedTargetEnvs:   []string{internalTest.Env2},
		},
	}
	for testName, ts := range scenarios {
		s.Run(testName, func() {
			rrBuilder := utils.NewRegistrationBuilder().WithName(appName)
			rr := rrBuilder.BuildRR()
			_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.TODO(), rr, metav1.CreateOptions{})
			s.Require().NoError(err, "Failed to create radix registration. Error %v", err)

			var envBuilders []utils.ApplicationEnvironmentBuilder
			for _, env := range ts.raEnvs {
				envBuilders = append(envBuilders,
					utils.NewApplicationEnvironmentBuilder().
						WithName(env.Name).
						WithBuildFrom(env.Build.From).
						WithWebhookEnabled(env.Build.WebhookEnabled))
			}
			ra := utils.NewRadixApplicationBuilder().WithAppName(appName).
				WithRadixRegistration(rrBuilder).
				WithComponent(getComponentBuilder()).
				WithApplicationEnvironmentBuilders(envBuilders...).BuildRA()

			radixPipelineType := v1.BuildDeploy
			pipelineType, err := pipeline.GetPipelineFromName(string(radixPipelineType))
			s.Require().NoError(err, "Failed to get pipeline type. Error %v", err)
			pipelineInfo := &model.PipelineInfo{
				Definition: pipelineType,
				PipelineArguments: model.PipelineArguments{
					AppName:              appName,
					ImageTag:             radixImageTag,
					JobName:              radixPipelineJobName,
					Branch:               branchMain,
					PipelineType:         string(radixPipelineType),
					DNSConfig:            &dnsalias.DNSConfig{},
					RadixConfigFile:      sampleAppRadixConfigFileName,
					GitWorkspace:         sampleAppWorkspace,
					TriggeredFromWebhook: ts.triggeredFromWebhook,
				},
				RadixRegistration: rr,
				RadixApplication:  ra,
			}
			buildContext := &model.BuildContext{}
			mockGitRepo := git.NewMockRepository(s.ctrl)
			mockGitRepo.EXPECT().Checkout(gomock.Any()).AnyTimes().Return(nil)
			mockGitRepo.EXPECT().ResolveCommitForReference(gomock.Any()).AnyTimes().Return("anycommitid", nil)
			mockGitRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).AnyTimes().Return(true, nil)
			mockGitRepo.EXPECT().ResolveTagsForCommit(gomock.Any()).AnyTimes().Return(nil, nil)
			mockContextBuilder := prepareInternal.NewMockContextBuilder(s.ctrl)
			mockContextBuilder.EXPECT().GetBuildContext(pipelineInfo, mockGitRepo).Return(buildContext, nil).AnyTimes()
			mockRadixConfigReader := prepareInternal.NewMockRadixConfigReader(s.ctrl)
			mockRadixConfigReader.EXPECT().Read(pipelineInfo).Return(ra, nil).AnyTimes()
			mockSubPipelineReader := prepareInternal.NewMockSubPipelineReader(s.ctrl)
			for _, env := range ts.expectedTargetEnvs {
				mockSubPipelineReader.EXPECT().ReadPipelineAndTasks(pipelineInfo, env).Return(false, "", nil, nil, nil).Times(1)
			}
			mockOwnerReferenceFactory := ownerreferences.NewMockOwnerReferenceFactory(s.ctrl)
			mockOwnerReferenceFactory.EXPECT().Create().Return(&metav1.OwnerReference{Kind: "RadixApplication", Name: appName}).AnyTimes()
			step := preparepipeline.NewPreparePipelinesStep(
				preparepipeline.WithRadixConfigReader(mockRadixConfigReader),
				preparepipeline.WithSubPipelineReader(mockSubPipelineReader),
				preparepipeline.WithBuildContextBuilder(mockContextBuilder),
				preparepipeline.WithOwnerReferenceFactory(mockOwnerReferenceFactory),
				preparepipeline.WithOpenGitRepoFunc(func(_ string) (git.Repository, error) { return mockGitRepo, nil }),
			)
			ctx := context.Background()
			step.Init(ctx, s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, s.tknClient, rr)
			err = step.Run(ctx, pipelineInfo)
			s.Require().NoError(err)
		})
	}
}

func getTestPipeline(modify func(pipeline *pipelinev1.Pipeline)) *pipelinev1.Pipeline {
	pl := &pipelinev1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline1"},
		Spec: pipelinev1.PipelineSpec{
			Tasks: []pipelinev1.PipelineTask{},
		},
	}
	if modify != nil {
		modify(pl)
	}
	return pl
}

func getRadixApplicationBuilder(appName, environment, buildFrom string) utils.ApplicationBuilder {
	return utils.NewRadixApplicationBuilder().WithAppName(appName).
		WithEnvironment(environment, buildFrom).
		WithComponent(getComponentBuilder())
}

func getComponentBuilder() utils.RadixApplicationComponentBuilder {
	return utils.NewApplicationComponentBuilder().WithName("comp1").WithPort("http", 8080).WithPublicPort("http")
}

func getTestTask(modify func(task *pipelinev1.Task)) *pipelinev1.Task {
	task := &pipelinev1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "task1"},
		Spec: pipelinev1.TaskSpec{
			Steps: []pipelinev1.Step{{Name: "step1"}},
		},
	}
	if modify != nil {
		modify(task)
	}
	return task
}
