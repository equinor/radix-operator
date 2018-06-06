package brigade

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	radix_v1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var projectPrefix = "Statoil/"

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

func (b *BrigadeGateway) EnsureProject(app *radix_v1.RadixApplication) error {
	if b.client == nil {
		return fmt.Errorf("No k8s client available")
	}

	log.Infof("Creating/Updating application %s", app.ObjectMeta.Name)
	trueVar := true
	secretsJson, _ := json.Marshal(app.Spec.Secrets)
	project := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("brigade-%s", shortSHA(projectPrefix+app.Name)),
			Labels: map[string]string{
				"app":       "brigade",
				"component": "project",
				"radixApp":  app.Name,
			},
			Annotations: map[string]string{
				"projectName": projectPrefix + app.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
					Kind:       "RadixApplication",
					Name:       app.Name,
					UID:        app.UID,
					Controller: &trueVar,
				},
			},
		},
		Type: "brigade.sh/project",
		StringData: map[string]string{
			"repository":        app.Spec.Repository,
			"sharedSecret":      app.Spec.SharedSecret,
			"cloneURL":          app.Spec.CloneURL,
			"sshKey":            strings.Replace(app.Spec.SshKey, "\n", "$", -1),
			"initGitSubmodules": "false",
			"defaultScript":     app.Spec.DefaultScript,
			"defaultScriptName": "",
			"secrets":           string(secretsJson),
		},
	}

	createdSecret, err := b.client.CoreV1().Secrets("default").Create(project)

	if err != nil {
		return fmt.Errorf("Failed to create Brigade project: %v", err)
	}
	projectCounter.Inc()

	log.Infof("Created: %s", createdSecret.Name)
	return nil
}

func (b *BrigadeGateway) DeleteProject(appName, namespace string) error {
	return nil
}

func shortSHA(input string) string {
	sum := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", sum)[0:54]
}
