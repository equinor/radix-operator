package radixapplication

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

var (
	storageAccountNameRegExp = regexp.MustCompile(`^[a-z0-9]{3,24}$`)
)

func createVolumeMountValidator() validatorFunc {
	return func(ctx context.Context, app *radixv1.RadixApplication) (string, error) {
		var errs []error
		var wrns []string

		for _, component := range app.Spec.Components {
			hasComponentIdentityAzureClientId := len(component.Identity.GetAzure().GetClientId()) > 0
			wrn, err := validateVolumeMounts(component.VolumeMounts, hasComponentIdentityAzureClientId)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed volumeMount validation for component %s. %w", component.Name, err))
			}
			if wrn != "" {
				wrns = append(wrns, fmt.Sprintf("component %s: %s", component.Name, wrn))
			}
			for _, envConfig := range component.EnvironmentConfig {
				hasEnvIdentityAzureClientId := hasComponentIdentityAzureClientId || len(envConfig.GetIdentity().GetAzure().GetClientId()) > 0
				wrn, err := validateVolumeMounts(envConfig.VolumeMounts, hasEnvIdentityAzureClientId)
				if err != nil {
					errs = append(errs, fmt.Errorf("failed volumeMount validation for component %s in environment %s. %w", component.Name, envConfig.Environment, err))
				}
				if wrn != "" {
					wrns = append(wrns, fmt.Sprintf("component %s in env %s: %s", component.Name, envConfig.Environment, wrn))
				}
			}
		}
		for _, job := range app.Spec.Jobs {
			hasJobIdentityAzureClientId := len(job.Identity.GetAzure().GetClientId()) > 0
			wrn, err := validateVolumeMounts(job.VolumeMounts, hasJobIdentityAzureClientId)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed volumeMount validation for job %s. %w", job.Name, err))
			}
			if wrn != "" {
				wrns = append(wrns, fmt.Sprintf("component %s: %s", job.Name, wrn))
			}
			for _, envConfig := range job.EnvironmentConfig {
				hasEnvIdentityAzureClientId := hasJobIdentityAzureClientId || len(envConfig.GetIdentity().GetAzure().GetClientId()) > 0
				wrn, err := validateVolumeMounts(envConfig.VolumeMounts, hasEnvIdentityAzureClientId)
				if err != nil {
					errs = append(errs, fmt.Errorf("failed volumeMount validation for job %s in environment %s. %w", job.Name, envConfig.Environment, err))
				}
				if wrn != "" {
					wrns = append(wrns, fmt.Sprintf("component %s in env %s: %s", job.Name, envConfig.Environment, wrn))
				}
			}
		}

		return strings.Join(wrns, "\n"), errors.Join(errs...)
	}
}

func validateVolumeMounts(volumeMounts []radixv1.RadixVolumeMount, hasIdentityAzureClientId bool) (string, error) {
	if len(volumeMounts) == 0 {
		return "", nil
	}
	wrns := []string{}

	for _, v := range volumeMounts {
		if len(strings.TrimSpace(v.Name)) == 0 {
			return "", ErrVolumeMountMissingName
		}

		if len(strings.TrimSpace(v.Path)) == 0 {
			return "", fmt.Errorf("volumeMount %s failed validation. %w", v.Name, ErrVolumeMountMissingPath)
		}

		if len(slice.FindAll(volumeMounts, func(rvm radixv1.RadixVolumeMount) bool { return rvm.Name == v.Name })) > 1 {
			return "", fmt.Errorf("volumeMount %s failed validation. %w", v.Name, ErrVolumeMountDuplicateName)
		}

		if len(slice.FindAll(volumeMounts, func(rvm radixv1.RadixVolumeMount) bool { return rvm.Path == v.Path })) > 1 {
			return "", fmt.Errorf("volumeMount %s failed validation. %w", v.Name, ErrVolumeMountDuplicatePath)
		}

		volumeSourceCount := len(slice.FindAll(
			[]bool{v.HasDeprecatedVolume(), v.HasBlobFuse2(), v.HasEmptyDir()},
			func(b bool) bool { return b }),
		)
		if volumeSourceCount > 1 {
			return "", fmt.Errorf("volumeMount %s failed validation. %w", v.Name, ErrVolumeMountMultipleTypes)
		}
		if volumeSourceCount == 0 {
			return "", fmt.Errorf("volumeMount %s failed validation. %w", v.Name, ErrVolumeMountMissingType)
		}

		switch {
		case v.HasDeprecatedVolume():
			if wrn := validateVolumeMountDeprecatedSource(&v); wrn != "" {
				wrns = append(wrns, fmt.Sprintf("volumeMount %s failed validation. %s", v.Name, wrn))
			}
		case v.HasBlobFuse2():
			if err := validateVolumeMountBlobFuse2(v.BlobFuse2, hasIdentityAzureClientId); err != nil {
				return "", fmt.Errorf("volumeMount %s failed validation. %w", v.Name, err)
			}
		case v.HasEmptyDir():
			if err := validateVolumeMountEmptyDir(v.EmptyDir); err != nil {
				return "", fmt.Errorf("volumeMount %s failed validation. %w", v.Name, err)
			}
		}
	}

	return strings.Join(wrns, "\n"), nil
}

func validateVolumeMountDeprecatedSource(v *radixv1.RadixVolumeMount) string {
	//nolint:staticcheck
	if v.Type != radixv1.MountTypeBlobFuse2FuseCsiAzure {
		return fmt.Sprintf("deprecated arguments failed validation. %s", ErrVolumeMountInvalidType)
	}
	//nolint:staticcheck
	if v.Type == radixv1.MountTypeBlobFuse2FuseCsiAzure && len(v.Storage) == 0 {
		return fmt.Sprintf("deprecated arguments failed validation. %s", ErrVolumeMountMissingStorage)
	}
	return ""
}

func validateVolumeMountBlobFuse2(fuse2 *radixv1.RadixBlobFuse2VolumeMount, hasIdentityAzureClientId bool) error {
	if !slices.Contains([]radixv1.BlobFuse2Protocol{radixv1.BlobFuse2ProtocolFuse2, ""}, fuse2.Protocol) {
		return fmt.Errorf("blobFuse2 failed validation. %w", ErrVolumeMountInvalidProtocol)
	}

	if len(fuse2.Container) == 0 {
		return fmt.Errorf("blobFuse2 failed validation. %w", ErrVolumeMountMissingContainer)
	}

	if len(fuse2.StorageAccount) > 0 && !storageAccountNameRegExp.Match([]byte(fuse2.StorageAccount)) {
		return fmt.Errorf("blobFuse2 failed validation. %w", ErrVolumeMountInvalidStorageAccount)
	}
	if fuse2.UseAzureIdentity != nil && *fuse2.UseAzureIdentity {
		if !hasIdentityAzureClientId {
			return fmt.Errorf("blobFuse2 failed validation. %w", ErrVolumeMountMissingAzureIdentity)
		}
		if fuse2.SubscriptionId == "" {
			return fmt.Errorf("blobFuse2 failed validation. %w", ErrVolumeMountWithUseAzureIdentityMissingSubscriptionId)
		}
		if fuse2.ResourceGroup == "" {
			return fmt.Errorf("blobFuse2 failed validation. %w", ErrVolumeMountWithUseAzureIdentityMissingResourceGroup)
		}
		if fuse2.StorageAccount == "" {
			return fmt.Errorf("blobFuse2 failed validation. %w", ErrVolumeMountWithUseAzureIdentityMissingStorageAccount)
		}
	}

	if err := validateBlobFuse2BlockCache(fuse2.BlockCacheOptions); err != nil {
		return fmt.Errorf("invalid blockCache configuration: %w", err)
	}

	return nil
}

func validateBlobFuse2BlockCache(blockCache *radixv1.BlobFuse2BlockCacheOptions) error {
	if blockCache == nil {
		return nil
	}

	if prefetchCount := blockCache.PrefetchCount; prefetchCount != nil && !(*prefetchCount == 0 || *prefetchCount > 10) {
		return ErrInvalidBlobFuse2BlockCachePrefetchCount
	}

	return nil
}

func validateVolumeMountEmptyDir(emptyDir *radixv1.RadixEmptyDirVolumeMount) error {
	if emptyDir.SizeLimit.IsZero() {
		return fmt.Errorf("emptyDir failed validation. %w", ErrVolumeMountMissingSizeLimit)
	}
	return nil
}
