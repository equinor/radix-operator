package models

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/equinor/radix-common/utils/slice"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	"github.com/equinor/radix-operator/api-server/api/utils/event"
	"github.com/equinor/radix-operator/api-server/api/utils/predicate"
	"github.com/equinor/radix-operator/api-server/api/utils/tlsvalidation"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	certManagerCertificateRevisionAnnotation = "cert-manager.io/certificate-revision"
)

// BuildComponents builds a list of Component models.
func BuildComponents(
	ra *radixv1.RadixApplication, rd *radixv1.RadixDeployment, deploymentList []appsv1.Deployment, podList []corev1.Pod,
	hpaList []autoscalingv2.HorizontalPodAutoscaler, secretList []corev1.Secret, eventList []corev1.Event, certs []cmv1.Certificate,
	certRequests []cmv1.CertificateRequest, tlsValidator tlsvalidation.Validator, scaledObjects []v1alpha1.ScaledObject,
) []*deploymentModels.Component {
	lastEventWarnings := event.ConvertToEventWarnings(eventList)
	var components []*deploymentModels.Component
	for _, component := range rd.Spec.Components {
		components = append(components, buildComponent(&component, ra, rd, deploymentList, podList, hpaList, secretList, certs, certRequests, lastEventWarnings, tlsValidator, scaledObjects))
	}

	for _, job := range rd.Spec.Jobs {
		components = append(components, buildComponent(&job, ra, rd, deploymentList, podList, hpaList, secretList, certs, certRequests, lastEventWarnings, tlsValidator, scaledObjects))
	}

	return components
}

func buildComponent(
	radixComponent radixv1.RadixCommonDeployComponent, ra *radixv1.RadixApplication, rd *radixv1.RadixDeployment,
	deploymentList []appsv1.Deployment, podList []corev1.Pod, hpaList []autoscalingv2.HorizontalPodAutoscaler,
	secretList []corev1.Secret, certs []cmv1.Certificate, certRequests []cmv1.CertificateRequest,
	lastEventWarnings map[string]string, tlsValidator tlsvalidation.Validator, scaledObjects []v1alpha1.ScaledObject,
) *deploymentModels.Component {
	builder := deploymentModels.NewComponentBuilder().
		WithComponent(radixComponent).
		WithStatus(deploymentModels.ConsistentComponent).
		WithHorizontalScalingSummary(GetHpaSummary(ra.Name, radixComponent.GetName(), hpaList, scaledObjects)).
		WithExternalDNS(getComponentExternalDNS(ra.Name, radixComponent, secretList, certs, certRequests, tlsValidator))

	var kd *appsv1.Deployment
	if depl, ok := slice.FindFirst(deploymentList, predicate.IsDeploymentForComponent(ra.Name, radixComponent.GetName())); ok {
		kd = &depl
	}
	componentPods := append(slice.FindAll(podList, predicate.IsPodForComponent(ra.Name, radixComponent.GetName())),
		slice.FindAll(podList, predicate.IsPodForAuxComponent(ra.Name, radixComponent.GetName(), kube.RadixJobTypeManagerAux))...)

	if rd.Status.ActiveTo.IsZero() {
		builder.WithPodNames(slice.Map(componentPods, func(pod corev1.Pod) string { return pod.Name }))
		builder.WithRadixEnvironmentVariables(getRadixEnvironmentVariables(componentPods))
		builder.WithReplicaSummaryList(BuildReplicaSummaryList(componentPods, lastEventWarnings))
		builder.WithStatus(deploymentModels.ComponentStatusFromDeployment(radixComponent, kd, rd))
		builder.WithAuxiliaryResource(getAuxiliaryResources(rd, radixComponent, deploymentList, podList, lastEventWarnings))
	}

	//nolint:godox
	// TODO: Use radixComponent.GetType() instead?
	if jobComponent, ok := radixComponent.(*radixv1.RadixDeployJobComponent); ok {
		builder.WithSchedulerPort(&jobComponent.SchedulerPort)
		if jobComponent.Payload != nil {
			builder.WithScheduledJobPayloadPath(jobComponent.Payload.Path)
		}
		builder.WithNotifications(jobComponent.Notifications)
	}

	// The only error that can be returned from DeploymentBuilder is related to errors from github.com/imdario/mergo
	// This type of error will only happen if incorrect objects (e.g. incompatible structs) are sent as arguments to mergo,
	// and we should consider to panic the error in the code calling merge.
	// For now we will panic the error here.
	component, err := builder.BuildComponent()
	if err != nil {
		panic(err)
	}
	return component
}

func getComponentExternalDNS(appName string, component radixv1.RadixCommonDeployComponent, secretList []corev1.Secret, certs []cmv1.Certificate, certRequests []cmv1.CertificateRequest, tlsValidator tlsvalidation.Validator) []deploymentModels.ExternalDNS {
	var externalDNSList []deploymentModels.ExternalDNS

	if tlsValidator == nil {
		tlsValidator = tlsvalidation.DefaultValidator()
	}

	for _, externalAlias := range component.GetExternalDNS() {
		var certData, keyData []byte
		status := deploymentModels.TLSStatusConsistent

		if secretValue, ok := slice.FindFirst(secretList, isSecretWithName(operatorutils.GetExternalDnsTlsSecretName(externalAlias))); ok {
			certData = secretValue.Data[corev1.TLSCertKey]
			keyData = secretValue.Data[corev1.TLSPrivateKeyKey]
			if certValue, keyValue := strings.TrimSpace(string(certData)), strings.TrimSpace(string(keyData)); len(certValue) == 0 || len(keyValue) == 0 || strings.EqualFold(certValue, secretDefaultData) || strings.EqualFold(keyValue, secretDefaultData) {
				status = deploymentModels.TLSStatusPending
			}
		} else {
			status = deploymentModels.TLSStatusPending
		}

		var x509Certs []deploymentModels.X509Certificate
		var statusMessages []string
		if status == deploymentModels.TLSStatusConsistent {
			x509Certs = append(x509Certs, deploymentModels.ParseX509CertificatesFromPEM(certData)...)

			if certIsValid, messages := tlsValidator.ValidateX509Certificate(certData, keyData, externalAlias.FQDN); !certIsValid {
				status = deploymentModels.TLSStatusInvalid
				statusMessages = append(statusMessages, messages...)
			}
		}

		externalDNSList = append(externalDNSList,
			deploymentModels.ExternalDNS{
				FQDN: externalAlias.FQDN,
				TLS: deploymentModels.TLS{
					UseAutomation:  externalAlias.UseCertificateAutomation,
					Automation:     getExternalDNSAutomationCondition(appName, externalAlias, certs, certRequests),
					Status:         status,
					StatusMessages: statusMessages,
					Certificates:   x509Certs,
				},
			},
		)
	}

	return externalDNSList
}

func getExternalDNSAutomationCondition(appName string, externalAlias radixv1.RadixDeployExternalDNS, certs []cmv1.Certificate, certRequests []cmv1.CertificateRequest) *deploymentModels.TLSAutomation {
	if !externalAlias.UseCertificateAutomation {
		return nil
	}

	// Find certificate belonging to the externalAlias.
	// A non-existing certificate most likely mean that radix-operator has not successfully
	// synced the RD that defines the externalAlias, and therefore not yet created the certificate object.
	certForAlias, found := slice.FindFirst(certs, certificateForExternalAliasPredicate(appName, externalAlias))
	if !found {
		return &deploymentModels.TLSAutomation{Status: deploymentModels.TLSAutomationPending}
	}

	// Check if the certificate Ready condition is True, which indicates that certificate issuance is successful
	// If False or not found we need to inspect the CertificateRequests belonging to the certificate
	if cond, found := slice.FindFirst(certForAlias.Status.Conditions, certificateConditionReady); found && cond.Status == cmmetav1.ConditionTrue {
		return &deploymentModels.TLSAutomation{Status: deploymentModels.TLSAutomationSuccess}
	}

	// Find all certificaterequests whos certificate revision is equal to or greater than the certificate's revision
	// Certificaterequests with lower revision values are obsolete.
	certRequestsForAlias := slice.FindAll(certRequests, certificateRequestNewOrCurrentForCertificate(&certForAlias))

	// An empty list indicates that cert-manager has not yet reconciled the certificate by creating a certificaterequest
	if len(certRequestsForAlias) == 0 {
		return &deploymentModels.TLSAutomation{Status: deploymentModels.TLSAutomationPending}
	}

	// Find the certificaterequest with the highest certificate revision value.
	slices.SortFunc(certRequestsForAlias, sortCertificateRequestByCertificateRevisionDesc)
	latestCertRequest := certRequestsForAlias[0]

	// Check if the certificate request has a condition of type Denied, InvalidRequest or a non-true Ready status with Reason=Failed, which indicates a failure.
	if cond, found := slice.FindFirst(latestCertRequest.Status.Conditions, certificateRequestConditionFailed); found {
		return &deploymentModels.TLSAutomation{Status: deploymentModels.TLSAutomationFailed, Message: cond.Message}
	}

	// Read message from the certificate's Ready condition
	var message string
	if cond, found := slice.FindFirst(latestCertRequest.Status.Conditions, certificateRequestConditionReady); found {
		message = cond.Message
	}

	return &deploymentModels.TLSAutomation{Status: deploymentModels.TLSAutomationPending, Message: message}

}

func certificateForExternalAliasPredicate(appName string, externalAlias radixv1.RadixDeployExternalDNS) func(cr cmv1.Certificate) bool {
	selector := radixlabels.ForExternalDNSResource(appName, externalAlias).AsSelector()
	return func(cr cmv1.Certificate) bool {
		return selector.Matches(labels.Set(cr.Labels))
	}
}

// Predicate function for filtering certificaterequests where annotation "cert-manager.io/certificate-revision" has a value
// equal to or greater that the corresponding certificates Status.Revision value. Ref https://cert-manager.io/docs/reference/api-docs/
func certificateRequestNewOrCurrentForCertificate(cert *cmv1.Certificate) func(cr cmv1.CertificateRequest) bool {
	var certRevision int
	if cert.Status.Revision != nil {
		certRevision = *cert.Status.Revision
	}

	return func(cr cmv1.CertificateRequest) bool {
		certRequestForRevision, err := strconv.Atoi(cr.Annotations[certManagerCertificateRevisionAnnotation])
		if err != nil {
			return false
		}
		return certRequestForRevision >= certRevision && metav1.IsControlledBy(&cr, cert)
	}
}

func sortCertificateRequestByCertificateRevisionDesc(a cmv1.CertificateRequest, b cmv1.CertificateRequest) int {
	// CertificateRequest should always have this annotation, and the value should be convertible to int, ref docs: https://cert-manager.io/docs/reference/api-docs/
	// We therefore ignore any errors returnd by Atoi since a parsing error will return 0 and thus sort in "lower" than any valid values
	revisionA, _ := strconv.Atoi(a.Annotations[certManagerCertificateRevisionAnnotation])
	revisionB, _ := strconv.Atoi(b.Annotations[certManagerCertificateRevisionAnnotation])
	return cmp.Compare(revisionB, revisionA)
}

func certificateRequestConditionFailed(condition cmv1.CertificateRequestCondition) bool {
	return (condition.Type == cmv1.CertificateRequestConditionDenied) ||
		(condition.Type == cmv1.CertificateRequestConditionInvalidRequest && condition.Status == cmmetav1.ConditionTrue) ||
		(condition.Type == cmv1.CertificateRequestConditionReady && condition.Status != cmmetav1.ConditionTrue && condition.Reason == "Failed")
}

func certificateConditionReady(condition cmv1.CertificateCondition) bool {
	return condition.Type == cmv1.CertificateConditionReady
}

func certificateRequestConditionReady(condition cmv1.CertificateRequestCondition) bool {
	return condition.Type == cmv1.CertificateRequestConditionReady
}

func getRadixEnvironmentVariables(pods []corev1.Pod) map[string]string {
	radixEnvironmentVariables := make(map[string]string)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			for _, envVariable := range container.Env {
				if operatorutils.IsRadixEnvVar(envVariable.Name) {
					radixEnvironmentVariables[envVariable.Name] = envVariable.Value
				}
			}
		}
	}

	return radixEnvironmentVariables
}
