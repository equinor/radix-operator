package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// ImageConfig holds configuration for building and loading images
type ImageConfig struct {
	Registry   string
	Repository string
	Tag        string
}

// GenerateImageTag generates a unique tag for the current build
func GenerateImageTag() string {
	// Get current timestamp for uniqueness
	timestamp := time.Now().Format("20060102-150405")

	// Try to get git commit hash
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err := cmd.Output()
	var gitHash string
	if err == nil {
		gitHash = strings.TrimSpace(string(output))
	} else {
		gitHash = "local"
	}

	return fmt.Sprintf("e2e-%s-%s", timestamp, gitHash)
}

// BuildAndLoadImages builds Docker images in parallel and loads them into the Kind cluster
func BuildAndLoadImages(ctx context.Context, clusterName string, imageTag string) error {
	// Get the project root directory (parent of e2e directory)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	projectRoot := filepath.Dir(cwd)

	images := []struct {
		name       string
		dockerfile string
		imageName  string
	}{
		{
			name:       "radix-operator",
			dockerfile: "operator.Dockerfile",
			imageName:  fmt.Sprintf("ghcr.io/equinor/radix/radix-operator:%s", imageTag),
		},
		{
			name:       "radix-webhook",
			dockerfile: "webhook.Dockerfile",
			imageName:  fmt.Sprintf("ghcr.io/equinor/radix/webhook:%s", imageTag),
		},
		{
			name:       "radix-job-scheduler",
			dockerfile: "job-scheduler.Dockerfile",
			imageName:  fmt.Sprintf("ghcr.io/equinor/radix/job-scheduler:%s", imageTag),
		},
	}

	// Build images in parallel
	var eg errgroup.Group
	for _, img := range images {
		eg.Go(func() error {
			fmt.Printf("Building %s image...\n", img.name)

			// Build the Docker image from project root
			buildCmd := exec.CommandContext(ctx, "docker", "build",
				"-t", img.imageName,
				"-f", img.dockerfile,
				".",
			)
			buildCmd.Dir = projectRoot
			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr

			if err := buildCmd.Run(); err != nil {
				return err
			}

			fmt.Printf("Successfully built %s\n", img.name)

			return nil
		})
	}

	// Wait for all builds to complete
	if err := eg.Wait(); err != nil {
		return err
	}

	// Load images sequentially (kind load doesn't benefit much from parallelization)
	for _, img := range images {
		fmt.Printf("Loading %s image into Kind cluster...\n", img.name)

		// Load the image into Kind
		loadCmd := exec.CommandContext(ctx, "kind", "load", "docker-image",
			img.imageName,
			"--name", clusterName,
		)
		loadCmd.Stdout = os.Stdout
		loadCmd.Stderr = os.Stderr

		if err := loadCmd.Run(); err != nil {
			return fmt.Errorf("failed to load %s into kind: %w", img.name, err)
		}

		fmt.Printf("Successfully loaded %s\n", img.name)
	}

	return nil
}

// GetImageValues returns Helm values for custom image tags
func GetImageValues(imageTag string) map[string]string {
	return map[string]string{
		"radixOperator.image.repository": "ghcr.io/equinor/radix/radix-operator",
		"radixOperator.image.tag":        imageTag,
		"radixWebhook.image.repository":  "ghcr.io/equinor/radix/webhook",
		"radixWebhook.image.tag":         imageTag,
		"jobScheduler.image.repository":  "ghcr.io/equinor/radix/job-scheduler",
		"jobScheduler.image.tag":         imageTag,
	}
}
