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

type componentSpec struct {
	Name         string
	Dockerfile   string
	ImageName    string
	HelmValueKey string
}

var componentSpecs = map[string]componentSpec{
	"operator": {
		Name:         "radix-operator",
		Dockerfile:   "operator.Dockerfile",
		ImageName:    "local-kind-repo/radix-operator",
		HelmValueKey: "image",
	},
	"webhook": {
		Name:         "radix-webhook",
		Dockerfile:   "webhook.Dockerfile",
		ImageName:    "local-kind-repo/webhook",
		HelmValueKey: "radixWebhook.image",
	},
	"pipeline-runner": {
		Name:         "radix-pipeline-runner",
		Dockerfile:   "pipeline.Dockerfile",
		ImageName:    "local-kind-repo/pipeline-runner",
		HelmValueKey: "radixPipelineRunner.image",
	},
	"job-scheduler": {
		Name:         "radix-job-scheduler",
		Dockerfile:   "job-scheduler.Dockerfile",
		ImageName:    "local-kind-repo/job-scheduler",
		HelmValueKey: "radixJobScheduler.image",
	},
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

	// Build images in parallel
	var eg errgroup.Group
	for _, spec := range componentSpecs {
		spec := spec // capture loop variable
		eg.Go(func() error {
			imageName := fmt.Sprintf("%s:%s", spec.ImageName, imageTag)
			fmt.Printf("Building %s image...\n", spec.Name)

			// Build the Docker image from project root
			buildCmd := exec.CommandContext(ctx, "docker", "build",
				"-t", imageName,
				"-f", spec.Dockerfile,
				".",
			)
			buildCmd.Dir = projectRoot
			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr

			if err := buildCmd.Run(); err != nil {
				return err
			}

			fmt.Printf("Successfully built %s\n", spec.Name)

			return nil
		})
	}

	// Wait for all builds to complete
	if err := eg.Wait(); err != nil {
		return err
	}

	// Load images sequentially (kind load doesn't benefit much from parallelization)
	for _, spec := range componentSpecs {
		imageName := fmt.Sprintf("%s:%s", spec.ImageName, imageTag)
		fmt.Printf("Loading %s image into Kind cluster...\n", spec.Name)

		// Load the image into Kind
		loadCmd := exec.CommandContext(ctx, "kind", "load", "docker-image",
			imageName,
			"--name", clusterName,
		)
		loadCmd.Stdout = os.Stdout
		loadCmd.Stderr = os.Stderr

		if err := loadCmd.Run(); err != nil {
			return fmt.Errorf("failed to load %s into kind: %w", spec.Name, err)
		}

		fmt.Printf("Successfully loaded %s\n", spec.Name)
	}

	return nil
}

// GetImageValues returns Helm values for custom image tags
func GetImageValues(imageTag string) map[string]string {
	values := make(map[string]string)

	for _, spec := range componentSpecs {
		values[fmt.Sprintf("%s.repository", spec.HelmValueKey)] = spec.ImageName
		values[fmt.Sprintf("%s.tag", spec.HelmValueKey)] = imageTag
	}

	return values
}
