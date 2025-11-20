package radixapplication

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func notificationValidator(_ context.Context, ra *radixv1.RadixApplication) ([]string, []error) {
	var errs []error
	for _, job := range ra.Spec.Jobs {
		if err := validateNotifications(ra, job.Notifications, job.GetName(), ""); err != nil {
			errs = append(errs, err)
		}
		for _, envConfig := range job.EnvironmentConfig {
			if err := validateNotifications(ra, envConfig.Notifications, job.GetName(), envConfig.Environment); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return nil, errs
}

func validateNotifications(ra *radixv1.RadixApplication, notifications *radixv1.Notifications, jobComponentName string, environment string) error {
	if notifications == nil || notifications.Webhook == nil || len(*notifications.Webhook) == 0 {
		return nil
	}
	webhook := strings.ToLower(strings.TrimSpace(*notifications.Webhook))
	webhookUrl, err := url.Parse(webhook)
	if err != nil {
		if environment != "" {
			return fmt.Errorf("job %s, environment %s: %w", jobComponentName, environment, ErrInvalidWebhookUrl)
		}
		return fmt.Errorf("job %s: %w", jobComponentName, ErrInvalidWebhookUrl)
	}
	if len(webhookUrl.Scheme) > 0 && webhookUrl.Scheme != "https" && webhookUrl.Scheme != "http" {
		if environment != "" {
			return fmt.Errorf("job %s, environment %s, scheme %s: %w", jobComponentName, environment, webhookUrl.Scheme, ErrNotAllowedSchemeInWebhookUrl)
		}
		return fmt.Errorf("job %s, scheme %s: %w", jobComponentName, webhookUrl.Scheme, ErrNotAllowedSchemeInWebhookUrl)
	}
	if len(webhookUrl.Port()) == 0 {
		if environment != "" {
			return fmt.Errorf("job %s, environment %s: %w", jobComponentName, environment, ErrMissingPortInWebhookUrl)
		}
		return fmt.Errorf("job %s: %w", jobComponentName, ErrMissingPortInWebhookUrl)
	}
	targetRadixComponent, targetRadixJobComponent := getRadixCommonComponentByName(ra, webhookUrl.Hostname())
	if targetRadixComponent == nil && targetRadixJobComponent == nil {
		if environment != "" {
			return fmt.Errorf("job %s, environment %s: %w", jobComponentName, environment, ErrOnlyAppComponentAllowedInWebhookUrl)
		}
		return fmt.Errorf("job %s: %w", jobComponentName, ErrOnlyAppComponentAllowedInWebhookUrl)
	}
	if targetRadixComponent != nil {
		componentPort := getComponentPort(targetRadixComponent, webhookUrl.Port())
		if componentPort == nil {
			if environment != "" {
				return fmt.Errorf("job %s, environment %s, port %s not found in component %s: %w", jobComponentName, environment, webhookUrl.Port(), targetRadixComponent.GetName(), ErrInvalidPortInWebhookUrl)
			}
			return fmt.Errorf("job %s, port %s not found in component %s: %w", jobComponentName, webhookUrl.Port(), targetRadixComponent.GetName(), ErrInvalidPortInWebhookUrl)
		}
		if strings.EqualFold(componentPort.Name, targetRadixComponent.PublicPort) {
			if environment != "" {
				return fmt.Errorf("job %s, environment %s, port %s is the public port of component %s: %w", jobComponentName, environment, webhookUrl.Port(), targetRadixComponent.GetName(), ErrInvalidUseOfPublicPortInWebhookUrl)
			}
			return fmt.Errorf("job %s, port %s is the public port of component %s: %w", jobComponentName, webhookUrl.Port(), targetRadixComponent.GetName(), ErrInvalidUseOfPublicPortInWebhookUrl)
		}
	} else if targetRadixJobComponent != nil {
		componentPort := getComponentPort(targetRadixJobComponent, webhookUrl.Port())
		if componentPort == nil {
			if environment != "" {
				return fmt.Errorf("job %s, environment %s, port %s not found in component %s: %w", jobComponentName, environment, webhookUrl.Port(), targetRadixJobComponent.GetName(), ErrInvalidPortInWebhookUrl)
			}
			return fmt.Errorf("job %s, port %s not found in component %s: %w", jobComponentName, webhookUrl.Port(), targetRadixJobComponent.GetName(), ErrInvalidPortInWebhookUrl)
		}
	}
	return nil
}
