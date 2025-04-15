package preparepipeline

import (
	"context"
	"strings"
	"testing"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/labels"
	internalTest "github.com/equinor/radix-operator/pipeline-runner/steps/internal/test"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal/validation"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	operatorDefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
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

func (s *stepTestSuite) Test_RunTestSuite(t *testing.T) {
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
}

func (s *stepTestSuite) SetupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.promClient = prometheusfake.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.tknClient = tektonfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, nil)
}

func (s *stepTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *stepTestSuite) Test_ComponentHasChangedSource() {
	const (
		envDev1 = "dev1"
		envDev2 = "dev2"
	)
	var (
		testScenarios = []struct {
			description          string
			changedFolders       []string
			sourceFolder         string
			checkChangesEnvName  string
			envName1SourceFolder *string
			envName1Enabled      *bool
			envName1Image        string
			expectedResult       bool
		}{
			{
				description:         "sourceFolder is dot",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        ".",
				checkChangesEnvName: envDev1,
				expectedResult:      true,
			},
			{
				description:         "several dots and slashes",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "././",
				checkChangesEnvName: envDev1,
				expectedResult:      true,
			},
			{
				description:         "no changes in the sourceFolder folder with trailing slash",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "nonexistingdir/",
				checkChangesEnvName: envDev1,
				expectedResult:      false,
			},
			{
				description:         "no changes in the sourceFolder folder without slashes",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "nonexistingdir",
				checkChangesEnvName: envDev1,
				expectedResult:      false,
			},
			{
				description:         "real source dir with trailing slash",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "src/",
				checkChangesEnvName: envDev1,
				expectedResult:      true,
			},
			{
				description:         "sourceFolder has surrounding slashes and leading dot",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "./src/",
				checkChangesEnvName: envDev1,
				expectedResult:      true,
			},
			{
				description:         "real source dir without trailing slash",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "./src",
				checkChangesEnvName: envDev1,
				expectedResult:      true,
			},
			{
				description:         "changes in the sourceFolder folder subfolder",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "src",
				checkChangesEnvName: envDev1,
				expectedResult:      true,
			},
			{
				description:         "changes in the sourceFolder multiple element path subfolder",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "src/subdir",
				checkChangesEnvName: envDev1,
				expectedResult:      true,
			},
			{
				description:         "changes in the sourceFolder multiple element path subfolder with trailing slash",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "src/subdir/",
				checkChangesEnvName: envDev1,
				expectedResult:      true,
			},
			{
				description:         "no changes in the sourceFolder multiple element path subfolder",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "src/subdir/water",
				checkChangesEnvName: envDev1,
				expectedResult:      false,
			},
			{
				description:         "changes in the sourceFolder multiple element path subfolder with trailing slash",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "src/subdir/water/",
				checkChangesEnvName: envDev1,
				expectedResult:      false,
			},
			{
				description:         "sourceFolder has name of changed folder",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "notebooks",
				checkChangesEnvName: envDev1,
				expectedResult:      true,
			},
			{
				description:         "empty sourceFolder is affected by any changes",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        "",
				checkChangesEnvName: envDev1,
				expectedResult:      true,
			},
			{
				description:         "empty sourceFolder is affected by any changes",
				changedFolders:      []string{"src/some/subdir"},
				sourceFolder:        "",
				checkChangesEnvName: envDev1,
				expectedResult:      true,
			},
			{
				description:         "sourceFolder sub-folder in the root",
				changedFolders:      []string{".", "app1"},
				sourceFolder:        "./app1",
				checkChangesEnvName: envDev1,
				expectedResult:      true,
			},
			{
				description:         "sourceFolder sub-folder in the root, no changed folders",
				changedFolders:      []string{},
				sourceFolder:        ".",
				checkChangesEnvName: envDev1,
				expectedResult:      false,
			},
			{
				description:         "sourceFolder sub-folder in empty, no changed folders",
				changedFolders:      []string{},
				sourceFolder:        "",
				checkChangesEnvName: envDev1,
				expectedResult:      false,
			},
			{
				description:         "sourceFolder sub-folder in slash, no changed folders",
				changedFolders:      []string{},
				sourceFolder:        "/",
				checkChangesEnvName: envDev1,
				expectedResult:      false,
			},
			{
				description:         "sourceFolder sub-folder in slash with dot, no changed folders",
				changedFolders:      []string{},
				sourceFolder:        "/.",
				checkChangesEnvName: envDev1,
				expectedResult:      false,
			},
			{
				description:          "sourceFolder is dot, env sourceFolder is empty",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         ".",
				checkChangesEnvName:  envDev1,
				envName1SourceFolder: pointers.Ptr(""),
				expectedResult:       true,
			},
			{
				description:         "sourceFolder is dot, env sourceFolder has image",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        ".",
				checkChangesEnvName: envDev1,
				envName1Image:       "some-image",
				expectedResult:      false,
			},
			{
				description:         "sourceFolder is dot, disabled env sourceFolder has image",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        ".",
				checkChangesEnvName: envDev1,
				envName1Enabled:     pointers.Ptr(false),
				envName1Image:       "some-image",
				expectedResult:      false,
			},
			{
				description:         "sourceFolder is dot, enabled env sourceFolder has image",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        ".",
				checkChangesEnvName: envDev1,
				envName1Enabled:     pointers.Ptr(true),
				envName1Image:       "some-image",
				expectedResult:      false,
			},
			{
				description:         "sourceFolder is dot, env sourceFolder has no image",
				changedFolders:      []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:        ".",
				checkChangesEnvName: envDev2,
				envName1Image:       "some-image",
				expectedResult:      true,
			},
			{
				description:          "sourceFolder is not changed, env sourceFolder is changed",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         "noneexistingdir",
				checkChangesEnvName:  envDev1,
				envName1SourceFolder: pointers.Ptr("src"),
				expectedResult:       true,
			},
			{
				description:          "sourceFolder is not changed, enabled env sourceFolder is changed",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         "noneexistingdir",
				checkChangesEnvName:  envDev1,
				envName1SourceFolder: pointers.Ptr("src"),
				envName1Enabled:      pointers.Ptr(true),
				expectedResult:       true,
			},
			{
				description:          "sourceFolder is not changed, disabled env sourceFolder is changed",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         "noneexistingdir",
				checkChangesEnvName:  envDev1,
				envName1SourceFolder: pointers.Ptr("src"),
				envName1Enabled:      pointers.Ptr(false),
				expectedResult:       false,
			},
			{
				description:          "sourceFolder is not changed, disabled env sourceFolder is changed, checking another env",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         "noneexistingdir",
				checkChangesEnvName:  envDev2,
				envName1SourceFolder: pointers.Ptr("src"),
				envName1Enabled:      pointers.Ptr(false),
				expectedResult:       false,
			},
			{
				description:          "sourceFolder is src, env sourceFolder is dot",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         "src/",
				checkChangesEnvName:  envDev1,
				envName1SourceFolder: pointers.Ptr("."),
				expectedResult:       true,
			},
			{
				description:          "sourceFolder is dot, env sourceFolder is src",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         ".",
				checkChangesEnvName:  envDev1,
				envName1SourceFolder: pointers.Ptr("src/"),
				expectedResult:       true,
			},
			{
				description:          "sourceFolder is dot, no env sourceFolder",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         ".",
				checkChangesEnvName:  envDev2,
				envName1SourceFolder: pointers.Ptr("src/"),
				expectedResult:       true,
			},
			{
				description:          "no changes in the sourceFolder folder, env sourceFolder is empty",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         "nonexistingdir",
				checkChangesEnvName:  envDev1,
				envName1SourceFolder: pointers.Ptr(""),
				expectedResult:       false,
			},
			{
				description:          "changes in the sourceFolder folder, env sourceFolder changed",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         ".",
				checkChangesEnvName:  envDev2,
				envName1SourceFolder: pointers.Ptr("nonexistingdir"),
				expectedResult:       true,
			},
			{
				description:          "changes in the sourceFolder folder, disabled env sourceFolder not changed",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         ".",
				checkChangesEnvName:  envDev1,
				envName1SourceFolder: pointers.Ptr("nonexistingdir"),
				envName1Enabled:      pointers.Ptr(false),
				expectedResult:       false,
			},
			{
				description:          "no changes in the sourceFolder folder, env sourceFolder is changed",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         "nonexistingdir",
				checkChangesEnvName:  envDev1,
				envName1SourceFolder: pointers.Ptr("src"),
				expectedResult:       true,
			},
			{
				description:          "no changes in the sourceFolder folder, env sourceFolder is not changed",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         "nonexistingdir",
				checkChangesEnvName:  envDev2,
				envName1SourceFolder: pointers.Ptr("src"),
				expectedResult:       false,
			},
			{
				description:          "changes in the sourceFolder folder subfolder, env sourceFolder is same folder",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         "src",
				checkChangesEnvName:  envDev1,
				envName1SourceFolder: pointers.Ptr("src"),
				expectedResult:       true,
			},
			{
				description:          "sourceFolder is dot, env sourceFolder is disabled",
				changedFolders:       []string{"src/some/subdir", "src/subdir/business_logic", "notebooks", "tests"},
				sourceFolder:         ".",
				checkChangesEnvName:  envDev1,
				envName1SourceFolder: pointers.Ptr(""),
				expectedResult:       true,
			},
		}
	)

	var applicationComponent v1.RadixComponent

	for _, testScenario := range testScenarios {
		s.T().Run(testScenario.description, func(t *testing.T) {
			environmentConfigBuilder := utils.AnEnvironmentConfig().WithEnvironment(envDev1).WithImage(testScenario.envName1Image)
			if testScenario.envName1SourceFolder != nil {
				environmentConfigBuilder.WithSourceFolder(*testScenario.envName1SourceFolder)
			}
			if testScenario.envName1Enabled != nil {
				environmentConfigBuilder.WithEnabled(*testScenario.envName1Enabled)
			}
			applicationComponent =
				utils.AnApplicationComponent().
					WithName("client-component-1").
					WithEnvironmentConfigs(
						environmentConfigBuilder,
					).
					WithSourceFolder(testScenario.sourceFolder).
					BuildComponent()
			sourceHasChanged := componentHasChangedSource(testScenario.checkChangesEnvName, &applicationComponent, testScenario.changedFolders)
			assert.Equal(t, testScenario.expectedResult, sourceHasChanged)
		})
	}
}

func (s *stepTestSuite) Test_pipelineContext_createPipeline() {
	type fields struct {
		radixApplicationBuilder utils.ApplicationBuilder
		targetEnvironments      []string
		hash                    string
		ownerReference          *metav1.OwnerReference
	}
	type args struct {
		envName   string
		pipeline  *pipelinev1.Pipeline
		tasks     []pipelinev1.Task
		timestamp string
	}
	const (
		appName              = "test-app"
		envDev               = "dev"
		branchMain           = "main"
		radixImageTag        = "tag-123"
		radixPipelineJobName = "pipeline-job-123"
		hash                 = "some-hash"
	)

	scenarios := []struct {
		name           string
		fields         fields
		args           args
		wantErr        func(t *testing.T, err error)
		assertScenario func(t *testing.T, step *PreparePipelinesStepImplementation, pipelineName string)
	}{
		{
			name: "one default task",
			fields: fields{
				targetEnvironments: []string{envDev},
				hash:               hash,
				ownerReference:     &metav1.OwnerReference{Kind: "RadixApplication", Name: appName},
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
			assertScenario: func(t *testing.T, step *PreparePipelinesStepImplementation, pipelineName string) {
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
		{
			name: "set SecurityContexts in task step",
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
			assertScenario: func(t *testing.T, step *PreparePipelinesStepImplementation, pipelineName string) {
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
		{
			name: "set SecurityContexts in task sidecar",
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
			assertScenario: func(t *testing.T, step *PreparePipelinesStepImplementation, pipelineName string) {
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
		{
			name: "set SecurityContexts in task stepTemplate",
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
			assertScenario: func(t *testing.T, step *PreparePipelinesStepImplementation, pipelineName string) {
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
		{
			name: "allow in the SecurityContext in task step non-root RunAsUser and RunAsGroup",
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
			assertScenario: func(t *testing.T, step *PreparePipelinesStepImplementation, pipelineName string) {
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
		{
			name: "allow set SecurityContexts in task sidecar non-root RunAsUser and RunAsGroup",
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
			assertScenario: func(t *testing.T, step *PreparePipelinesStepImplementation, pipelineName string) {
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
		{
			name: "allow set SecurityContexts in task stepTemplate non-root RunAsUser and RunAsGroup",
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
			assertScenario: func(t *testing.T, step *PreparePipelinesStepImplementation, pipelineName string) {
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
		{
			name: "Test sanitizeAzureSkipContainersAnnotation in task stepTemplate",
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
			assertScenario: func(t *testing.T, step *PreparePipelinesStepImplementation, pipelineName string) {
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
		{
			name: "Test unknown steps is not allowed in sanitizeAzureSkipContainersAnnotation",
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
			assertScenario: func(t *testing.T, step *PreparePipelinesStepImplementation, pipelineName string) {},
		},
		{
			name: "Test illegal azure WI label value in task",
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
			assertScenario: func(t *testing.T, step *PreparePipelinesStepImplementation, pipelineName string) {},
		},
	}
	for _, scenario := range scenarios {
		s.T().Run(scenario.name, func(t *testing.T) {
			applicationBuilder := scenario.fields.radixApplicationBuilder
			if applicationBuilder == nil {
				applicationBuilder = getRadixApplicationBuilder(appName, envDev, branchMain)
			}
			rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
			_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.TODO(), rr, metav1.CreateOptions{})
			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					AppName:       appName,
					ImageTag:      radixImageTag,
					JobName:       radixPipelineJobName,
					Branch:        branchMain,
					PipelineType:  string(v1.Deploy),
					ToEnvironment: internalTest.Env1,
					DNSConfig:     &dnsalias.DNSConfig{},
				},
				RadixRegistration: rr,
				RadixApplication:  applicationBuilder.BuildRA(),
			}
			step := &PreparePipelinesStepImplementation{
				stepType: pipeline.PreparePipelinesStep,
			}
			ctx := context.Background()
			step.Init(ctx, s.kubeClient, s.radixClient, s.kubeUtil, s.promClient, s.tknClient, rr)
			step.Run(ctx, pipelineInfo)
			//err := pipelineCtx.createPipeline(scenario.args.envName, scenario.args.pipeline, scenario.args.tasks, scenario.args.timestamp)
			scenario.wantErr(t, err)
			scenario.assertScenario(t, step, scenario.args.pipeline.ObjectMeta.Name)
		})
	}
}

func getTestPipeline(modify func(pipeline *pipelinev1.Pipeline)) *pipelinev1.Pipeline {
	pipeline := &pipelinev1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline1"},
		Spec: pipelinev1.PipelineSpec{
			Tasks: []pipelinev1.PipelineTask{},
		},
	}
	if modify != nil {
		modify(pipeline)
	}
	return pipeline
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
