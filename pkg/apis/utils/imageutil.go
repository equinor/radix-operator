package utils

import "fmt"

// GetImagePath Returns a container image path given container registry, app, component and tag
func GetImagePath(containerRegistry, appName, componentName, imageTag string) string {
	repositoryName := GetRepositoryName(appName, componentName)
	imageName := fmt.Sprintf("%s/%s:%s", containerRegistry, repositoryName, imageTag)
	return imageName
}

// GetRepositoryName Returns a container registry repository name given
func GetRepositoryName(appName, componentName string) string {
	repositoryName := fmt.Sprintf("%s-%s", appName, componentName)
	return repositoryName
}