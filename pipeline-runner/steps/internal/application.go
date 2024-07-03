package internal

import (
	"context"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	validate "github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// CreateRadixApplication Create RadixApplication from radixconfig.yaml content
func CreateRadixApplication(ctx context.Context, radixClient radixclient.Interface, dnsConfig *dnsalias.DNSConfig, configFileContent string) (*radixv1.RadixApplication, error) {
	ra := &radixv1.RadixApplication{}

	// Important: Must use sigs.k8s.io/yaml decoder to correctly unmarshal Kubernetes objects.
	// This package supports encoding and decoding of yaml for CRD struct types using the json tag.
	// The gopkg.in/yaml.v3 package requires the yaml tag.
	if err := yaml.Unmarshal([]byte(configFileContent), ra); err != nil {
		return nil, err
	}
	correctRadixApplication(ctx, ra)

	// Validate RA
	if validate.RAContainsOldPublic(ra) {
		log.Ctx(ctx).Warn().Msg("component.public is deprecated, please use component.publicPort instead")
	}
	if err := validate.CanRadixApplicationBeInserted(ctx, radixClient, ra, dnsConfig); err != nil {
		log.Ctx(ctx).Error().Msg("Radix config not valid")
		return nil, err
	}
	return ra, nil
}

func correctRadixApplication(ctx context.Context, ra *radixv1.RadixApplication) {
	if isAppNameLowercase, err := validate.IsApplicationNameLowercase(ra.Name); !isAppNameLowercase {
		log.Ctx(ctx).Warn().Err(err).Msg("%s Converting name to lowercase")
		ra.Name = strings.ToLower(ra.Name)
	}
	for i := 0; i < len(ra.Spec.Components); i++ {
		ra.Spec.Components[i].Resources = buildResource(&ra.Spec.Components[i])
	}
	for i := 0; i < len(ra.Spec.Jobs); i++ {
		ra.Spec.Jobs[i].Resources = buildResource(&ra.Spec.Jobs[i])
	}
}

func buildResource(component radixv1.RadixCommonComponent) radixv1.ResourceRequirements {
	memoryReqName := corev1.ResourceMemory.String()
	resources := component.GetResources()
	delete(resources.Limits, memoryReqName)

	if requestsMemory, ok := resources.Requests[memoryReqName]; ok {
		if resources.Limits == nil {
			resources.Limits = radixv1.ResourceList{}
		}
		resources.Limits[memoryReqName] = requestsMemory
	}
	return resources
}
