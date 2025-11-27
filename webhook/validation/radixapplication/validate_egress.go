package radixapplication

import (
	"context"
	"fmt"
	"net"
	"strings"

	commonUtils "github.com/equinor/radix-common/utils"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	maximumPortNumber          = 65535
	maximumNumberOfEgressRules = 1000
)

func environmentEgressValidator(ctx context.Context, ra *radixv1.RadixApplication) ([]string, []error) {

	var errs []error
	for _, env := range ra.Spec.Environments {
		if len(env.Egress.Rules) > maximumNumberOfEgressRules {
			errs = append(errs, fmt.Errorf("environment %s: %w (max: %d)", env.Name, ErrEgressRulesExceedMaxNumber, maximumNumberOfEgressRules))
			continue
		}
		for _, egressRule := range env.Egress.Rules {
			if len(egressRule.Destinations) < 1 {
				errs = append(errs, fmt.Errorf("environment %s: %w", env.Name, ErrEgressRuleMustContainAtLeastOneDestination))
			}
			for _, ipMask := range egressRule.Destinations {
				err := validateEgressRuleIpMask(string(ipMask))
				if err != nil {
					errs = append(errs, fmt.Errorf("environment %s: %w", env.Name, err))
				}
			}
			for _, port := range egressRule.Ports {
				err := validateEgressRulePortProtocol(port.Protocol)
				if err != nil {
					errs = append(errs, fmt.Errorf("environment %s: %w", env.Name, err))
				}
				err = validateEgressRulePort(port.Port)
				if err != nil {
					errs = append(errs, fmt.Errorf("environment %s: %w", env.Name, err))
				}
			}
		}
	}

	return nil, errs
}

func validateEgressRulePort(port int32) error {
	if port < 1 || port > maximumPortNumber {
		return ErrEgressRulePortOutOfRange
	}
	return nil
}

func validateEgressRulePortProtocol(protocol string) error {
	upperCaseProtocol := strings.ToUpper(protocol)
	validProtocols := []string{string(corev1.ProtocolTCP), string(corev1.ProtocolUDP)}
	if commonUtils.ContainsString(validProtocols, upperCaseProtocol) {
		return nil
	} else {
		return fmt.Errorf("protocol %s (valid: %v): %w", protocol, validProtocols, ErrInvalidEgressPortProtocol)
	}
}

func validateEgressRuleIpMask(ipMask string) error {
	ipAddr, _, err := net.ParseCIDR(ipMask)
	if err != nil {
		return fmt.Errorf("%s: %w", ipMask, ErrNotValidCidr)
	}
	ipV4Addr := ipAddr.To4()
	if ipV4Addr == nil {
		return fmt.Errorf("%s: %w", ipMask, ErrNotValidIPv4Cidr)
	}

	return nil
}
