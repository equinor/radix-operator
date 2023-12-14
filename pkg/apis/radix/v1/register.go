package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion provides the group version
var SchemeGroupVersion = schema.GroupVersion{
	Group:   "radix.equinor.com",
	Version: "v1",
}
var (
	// SchemeBuilder builds a scheme
	SchemeBuilder      runtime.SchemeBuilder
	localSchemeBuilder = &SchemeBuilder
	// AddToScheme adds to scheme
	AddToScheme = localSchemeBuilder.AddToScheme
)

func init() {
	localSchemeBuilder.Register(addKnownTypes)
}

// Resource does things to resource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// addKnownTypes adds our types to the API scheme by registering
// RadixApplication, RadixApplicationList, RadixDeployment, RadixDeploymentList, etc.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		SchemeGroupVersion,
		&RadixApplication{},
		&RadixApplicationList{},
		&RadixDeployment{},
		&RadixDeploymentList{},
		&RadixRegistration{},
		&RadixRegistrationList{},
		&RadixJob{},
		&RadixJobList{},
		&RadixEnvironment{},
		&RadixEnvironmentList{},
		&RadixAlert{},
		&RadixAlertList{},
		&RadixBatch{},
		&RadixBatchList{},
		&RadixDNSAlias{},
		&RadixDNSAliasList{},
	)

	// register the type in the scheme
	meta_v1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

const (
	// KindRadixRegistration RadixRegistration object Kind
	KindRadixRegistration = "RadixRegistration"
	// KindRadixEnvironment RadixEnvironment object Kind
	KindRadixEnvironment = "RadixEnvironment"
	// KindRadixApplication RadixApplication object Kind
	KindRadixApplication = "RadixApplication"
	// KindRadixDeployment RadixDeployment object Kind
	KindRadixDeployment = "RadixDeployment"
	// KindRadixJob RadixJob object Kind
	KindRadixJob = "RadixJob"
	// KindRadixBatch RadixBatch object Kind
	KindRadixBatch = "RadixBatch"
	// KindRadixAlert RadixAlert object Kind
	KindRadixAlert = "RadixAlert"
	// KindRadixDNSAlias RadixDNSAlias object Kind
	KindRadixDNSAlias = "RadixDNSAlias"
	// ResourceRadixRegistrations RadixRegistrations API resource
	ResourceRadixRegistrations = "radixregistrations"
	// ResourceRadixDNSAliases RadixDNSAliases API resource
	ResourceRadixDNSAliases = "radixdnsaliases"
	// ResourceRadixDeployment RadixDeployment API resource
	ResourceRadixDeployment = "radixdeployment"
)
