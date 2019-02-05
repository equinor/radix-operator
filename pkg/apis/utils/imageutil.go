package utils

import "fmt"

// GetImagePath Returns a container image path given container registry, app, component and tag
func GetImagePath(containerRegistry, appName, componentName, imageTag string) string {
	imageName := fmt.Sprintf("%s/%s-%s:%s", containerRegistry, appName, componentName, imageTag)
	return imageName
}
