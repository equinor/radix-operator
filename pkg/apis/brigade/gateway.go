package brigade

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	radix_v1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const namespace = "default"

var projectCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "project_created",
	Help: "Number of projects created by the Radix Operator",
})

type BrigadeGateway struct {
	client kubernetes.Interface
}

func init() {
	prometheus.MustRegister(projectCounter)
}

func New(clientset kubernetes.Interface) (*BrigadeGateway, error) {
	if clientset == nil {
		return nil, fmt.Errorf("Missing client")
	}

	gw := &BrigadeGateway{
		client: clientset,
	}
	return gw, nil
}

//EnsureProject will create a Brigade project if it doesn't exist or update existing one
func (b *BrigadeGateway) EnsureProject(appRegistration *radix_v1.RadixRegistration) error {
	create := false
	if b.client == nil {
		return fmt.Errorf("No k8s client available")
	}

	user, repo, err := getProjectName(appRegistration)
	brigadeProjectName := fmt.Sprintf("%s/%s", user, repo)
	if err != nil {
		return err
	}
	secretName := fmt.Sprintf("brigade-%s", shortSHA(brigadeProjectName))
	log.Infof("Creating/Updating application %s", appRegistration.ObjectMeta.Name)
	project, _ := b.getExistingBrigadeProject(secretName)
	if project == nil {
		project = createNewProject(secretName, appRegistration.Name, brigadeProjectName, appRegistration.UID)
		create = true
	}

	kubeutil, err := kube.New(b.client)
	if err != nil {
		return fmt.Errorf("Failed to get k8s util: %v", err)
	}

	var creds *kube.ContainerRegistryCredentials
	if creds, err = kubeutil.RetrieveContainerRegistryCredentials(); err != nil {
		return err
	}

	if appRegistration.Spec.Secrets == nil {
		appRegistration.Spec.Secrets = radix_v1.SecretsMap{}
	}
	appRegistration.Spec.Secrets["APP_NAME"] = appRegistration.Name
	appRegistration.Spec.Secrets["DOCKER_USER"] = creds.User
	appRegistration.Spec.Secrets["DOCKER_PASS"] = creds.Password
	appRegistration.Spec.Secrets["DOCKER_REGISTRY"] = creds.Server

	secretsJSON, _ := json.Marshal(appRegistration.Spec.Secrets)
	project.StringData = map[string]string{
		"repository":        appRegistration.Spec.Repository,
		"sharedSecret":      appRegistration.Spec.SharedSecret,
		"cloneURL":          appRegistration.Spec.CloneURL,
		"sshKey":            strings.Replace(appRegistration.Spec.DeployKey, "\n", "$", -1),
		"initGitSubmodules": "false",
		"defaultScript":     appRegistration.Spec.DefaultScript,
		"defaultScriptName": appRegistration.Spec.DefaultScriptName,
		"secrets":           string(secretsJSON),
	}

	if create {
		createdSecret, err := b.client.CoreV1().Secrets(namespace).Create(project)
		if err != nil {
			return fmt.Errorf("Failed to create Brigade project: %v", err)
		}

		log.Infof("Created: %s", createdSecret.Name)
		projectCounter.Inc()
	} else {
		_, err := b.client.CoreV1().Secrets(namespace).Update(project)
		if err != nil {
			log.Errorf("Failed to update registration: %v", err)
			return err
		}
	}

	return nil
}

func (b *BrigadeGateway) DeleteProject(appName, namespace string) error {
	return nil
}

func getProjectName(registration *radix_v1.RadixRegistration) (user string, repo string, err error) {
	if registration.Spec.Repository == "" {
		return "", "", fmt.Errorf("No repository defined")
	}

	segments := strings.Split(registration.Spec.Repository, "/")
	user = segments[3]
	repo = segments[4]
	return user, repo, nil
}

func shortSHA(input string) string {
	sum := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", sum)[0:54]
}

func (b *BrigadeGateway) getExistingBrigadeProject(name string) (*v1.Secret, error) {
	secret, err := b.client.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if secret != nil {
		secret.ObjectMeta.Name = name
	}

	if errors.IsNotFound(err) {
		return nil, nil
	}

	return secret, err
}

func createNewProject(name, appName, brigadeProjectName string, ownerID types.UID) *v1.Secret {
	trueVar := true

	project := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app":       "brigade",
				"component": "project",
				"radixApp":  appName,
			},
			Annotations: map[string]string{
				"projectName": brigadeProjectName,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
					Kind:       "RadixRegistration",
					Name:       appName,
					UID:        ownerID,
					Controller: &trueVar,
				},
			},
		},
		Type: "brigade.sh/project",
	}
	return project
}
