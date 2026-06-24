package application

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// ParseRadixApplication Parse and validate RadixApplication from configFileContent
func ParseRadixApplication(ctx context.Context, radixClient radixclient.Interface, appName string, configFileContent []byte) (*radixv1.RadixApplication, error) {
	ra := &radixv1.RadixApplication{}

	// Important: Must use sigs.k8s.io/yaml decoder to correctly unmarshal Kubernetes objects.
	// This package supports encoding and decoding of yaml for CRD struct types using the json tag.
	// The gopkg.in/yaml.v3 package requires the yaml tag.
	if err := yaml.Unmarshal(configFileContent, ra); err != nil {
		return nil, err
	}
	if ra.GetName() != appName {
		return nil, fmt.Errorf("the application name %s in the radixconfig file does not match the registered application name %s", ra.GetName(), appName)
	}
	correctRadixApplication(ctx, ra)
	return ra, nil
}

func correctRadixApplication(ctx context.Context, ra *radixv1.RadixApplication) {
	if !isApplicationNameLowercase(ra.Name) {
		log.Ctx(ctx).Warn().Msgf("Converting name %s to lowercase ", ra.Name)
		ra.Name = strings.ToLower(ra.Name)
	}
	for i := 0; i < len(ra.Spec.Components); i++ {
		ra.Spec.Components[i].Resources = buildResource(&ra.Spec.Components[i])
	}
	for i := 0; i < len(ra.Spec.Jobs); i++ {
		ra.Spec.Jobs[i].Resources = buildResource(&ra.Spec.Jobs[i])
	}
}

func isApplicationNameLowercase(appName string) bool {
	for _, r := range appName {
		if unicode.IsUpper(r) && unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func buildResource(component radixv1.RadixCommonComponent) radixv1.ResourceRequirements {
	memoryReqName := corev1.ResourceMemory.String()
	resources := component.GetResources()

	memReq, hasMemReq := resources.Requests[memoryReqName]
	memLimit, hasMemLimit := resources.Limits[memoryReqName]

	if hasMemReq && !hasMemLimit {
		if resources.Limits == nil {
			resources.Limits = radixv1.ResourceList{}
		}
		resources.Limits[memoryReqName] = memReq
	}

	if !hasMemReq && hasMemLimit {
		if resources.Requests == nil {
			resources.Requests = radixv1.ResourceList{}
		}
		resources.Requests[memoryReqName] = memLimit
	}
	return resources
}
