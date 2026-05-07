package pods

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/api-server/api/utils/labelselector"
	sortUtils "github.com/equinor/radix-operator/api-server/api/utils/sort"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// PodHandler Instance variables
type PodHandler struct {
	client kubernetes.Interface
}

// Init Constructor
func Init(client kubernetes.Interface) PodHandler {
	return PodHandler{client}
}

// HandleGetAppPodLog Get logs from pod in app namespace
func (ph PodHandler) HandleGetAppPodLog(ctx context.Context, appName, podName, containerName string, sinceTime *time.Time, logLines *int64, follow bool) (io.ReadCloser, error) {
	appNs := crdUtils.GetAppNamespace(appName)
	return ph.getPodLog(ctx, appNs, podName, containerName, sinceTime, logLines, false, follow)
}

// HandleGetEnvironmentPodLog Get logs from pod in environment
func (ph PodHandler) HandleGetEnvironmentPodLog(ctx context.Context, appName, envName, podName, containerName string, sinceTime *time.Time, logLines *int64, previousLog, asStream bool) (io.ReadCloser, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	return ph.getPodLog(ctx, envNs, podName, containerName, sinceTime, logLines, previousLog, asStream)
}

// HandleGetEnvironmentScheduledJobLog Get logs from scheduled job in environment
func (ph PodHandler) HandleGetEnvironmentScheduledJobLog(ctx context.Context, appName, envName, scheduledJobName, replicaName, containerName string, sinceTime *time.Time, logLines *int64, follow bool) (io.ReadCloser, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	return ph.getScheduledJobLog(ctx, envNs, scheduledJobName, replicaName, containerName, sinceTime, logLines, follow)
}

// HandleGetEnvironmentAuxiliaryResourcePodLog Get logs from auxiliary resource pod in environment
func (ph PodHandler) HandleGetEnvironmentAuxiliaryResourcePodLog(ctx context.Context, appName, envName, componentName, auxType, podName string, sinceTime *time.Time, logLines *int64, follow bool) (io.ReadCloser, error) {
	envNs := crdUtils.GetEnvironmentNamespace(appName, envName)
	pods, err := ph.client.CoreV1().Pods(envNs).List(ctx, metav1.ListOptions{
		LabelSelector: labelselector.ForAuxiliaryResource(appName, componentName, auxType).String(),
		FieldSelector: getPodNameFieldSelector(podName),
	})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, PodNotFoundError(podName)
	}
	return ph.getPodLog(ctx, envNs, podName, "", sinceTime, logLines, false, follow)
}

func (ph PodHandler) getPodLog(ctx context.Context, namespace, podName, containerName string, sinceTime *time.Time, logLines *int64, previousLog, follow bool) (io.ReadCloser, error) {
	pod, err := ph.client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return ph.getPodLogFor(ctx, pod, containerName, sinceTime, logLines, previousLog, follow)
}

func (ph PodHandler) getScheduledJobLog(ctx context.Context, namespace, scheduledJobName, replicaName, containerName string, sinceTime *time.Time, logLines *int64, follow bool) (io.ReadCloser, error) {
	pods, err := ph.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", scheduledJobName),
	})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, PodNotFoundError(scheduledJobName)
	}
	if pod, ok := getPod(pods, replicaName); ok {
		return ph.getPodLogFor(ctx, pod, containerName, sinceTime, logLines, false, follow)
	}
	podNameForError := scheduledJobName
	if len(replicaName) > 0 {
		podNameForError = fmt.Sprintf("%s/%s", podNameForError, replicaName)
	}
	return nil, PodNotFoundError(podNameForError)
}

func getPod(pods *corev1.PodList, replicaName string) (*corev1.Pod, bool) {
	if len(pods.Items) == 0 {
		return nil, false
	}
	if len(replicaName) == 0 {
		sortUtils.Pods(pods.Items, sortUtils.ByPodCreationTimestamp, sortUtils.Descending)
		return &pods.Items[0], true
	}
	pod, ok := slice.FindFirst(pods.Items, func(pod corev1.Pod) bool {
		return strings.EqualFold(pod.GetName(), replicaName)
	})
	return &pod, ok
}

func (ph PodHandler) getPodLogFor(ctx context.Context, pod *corev1.Pod, containerName string, sinceTime *time.Time, logLines *int64, previousLog, follow bool) (io.ReadCloser, error) {
	req := getPodLogRequest(ph.client, pod, containerName, follow, sinceTime, logLines, previousLog)
	return req.Stream(ctx)
}

func getPodLogRequest(client kubernetes.Interface, pod *corev1.Pod, containerName string, follow bool, sinceTime *time.Time, logLines *int64, previousLog bool) *rest.Request {
	podLogOption := corev1.PodLogOptions{
		Follow:    follow,
		TailLines: logLines,
		Previous:  previousLog,
	}

	if sinceTime != nil {
		podLogOption.SinceTime = &metav1.Time{
			Time: *sinceTime,
		}
	}

	if containerName != "" {
		podLogOption.Container = containerName
	}

	return client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOption)
}

func getPodNameFieldSelector(podName string) string {
	return fmt.Sprintf("metadata.name=%s", podName)
}
