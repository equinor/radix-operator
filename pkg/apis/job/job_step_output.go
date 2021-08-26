package job

import (
	"context"
	"encoding/json"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	ScanStatusReasonNotRequested     = "Pipeline did not request scan job to output results"
	ScanStatusReasonOutputDeleted    = "Output from scan job deleted"
	ScanStatusReasonResultMissing    = "Scan results not found in output from scan job"
	ScanStatusReasonResultParseError = "Unable to parse output from scan job"
)

func getJobStepOutput(kubeClient kubernetes.Interface, jobType, containerOutputName, namespace string, containerStatus corev1.ContainerStatus) *v1.RadixJobStepOutput {
	switch jobType {
	case kube.RadixJobTypeScan:
		return getScanJobStepOutput(kubeClient, containerOutputName, namespace, containerStatus)
	}

	return nil
}

func getScanJobStepOutput(kubeClient kubernetes.Interface, outputConfigMapName, namespace string, containerStatus corev1.ContainerStatus) *v1.RadixJobStepOutput {
	// Wait for completion of container before processing scan step output
	if containerStatus.State.Terminated == nil {
		return nil
	}

	scanOutput := getScanJobOutput(kubeClient, outputConfigMapName, namespace)
	return &v1.RadixJobStepOutput{
		Scan: scanOutput,
	}
}

func getScanJobOutput(kubeClient kubernetes.Interface, configMapName, namespace string) *v1.RadixJobStepScanOutput {
	scanMissing := v1.RadixJobStepScanOutput{Status: v1.ScanMissing}
	if configMapName == "" {
		scanMissing.Reason = ScanStatusReasonNotRequested
		return &scanMissing
	}

	cm, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		scanMissing.Reason = ScanStatusReasonOutputDeleted
		return &scanMissing
	}

	vulnerabilityCountJson, exists := cm.Data[defaults.RadixPipelineScanStepVulnerabilityCountKey]
	if !exists || len(vulnerabilityCountJson) == 0 {
		scanMissing.Reason = ScanStatusReasonResultMissing
		return &scanMissing
	}

	vulnerabilityCountMap := make(v1.VulnerabilityMap)
	if err := json.Unmarshal([]byte(vulnerabilityCountJson), &vulnerabilityCountMap); err != nil {
		scanMissing.Reason = ScanStatusReasonResultParseError
		return &scanMissing
	}

	scanOutput := v1.RadixJobStepScanOutput{
		Status:                     v1.ScanSuccess,
		Vulnerabilities:            vulnerabilityCountMap,
		VulnerabilityListKey:       defaults.RadixPipelineScanStepVulnerabilityListKey,
		VulnerabilityListConfigMap: configMapName,
	}

	return &scanOutput
}
