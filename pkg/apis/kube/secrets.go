package kube

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DockerConfigJson struct {
	Authentication map[string]interface{} `json:"auths"`
}

type ContainerRegistryCredentials struct {
	Server   string `json:""`
	User     string
	Password string
}

//RetrieveContainerRegistryCredentials retrieves the default container registry credentials from Kubernetes secret
func (k *Kube) RetrieveContainerRegistryCredentials() (*ContainerRegistryCredentials, error) {
	secret, err := k.kubeClient.CoreV1().Secrets("default").Get("radix-docker", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve container credentials: %v", err)
	}
	credsJSON, ok := secret.Data[".dockerconfigjson"]
	if !ok {
		return nil, fmt.Errorf("Failed to read docker config from radix-docker")
	}

	var config DockerConfigJson
	err = json.Unmarshal(credsJSON, &config)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal docker config: %v", err)
	}

	creds := &ContainerRegistryCredentials{}
	for key := range config.Authentication {
		creds.Server = key
		values := config.Authentication[key].(map[string]interface{})
		creds.User = values["username"].(string)
		creds.Password = values["password"].(string)
	}

	return creds, nil
}
