package utils

import (
	"fmt"
	"strings"
	"time"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO : Separate out into library functions
func getAppNamespace(appName string) string {
	return fmt.Sprintf("%s-app", appName)
}

func getNamespaceForApplicationEnvironment(appName, environment string) string {
	return fmt.Sprintf("%s-%s", appName, environment)
}

func getDeploymentName(appName, imageTag string) string {
	return fmt.Sprintf("%s-%s", appName, imageTag)
}

func getAppAndImagePairFromName(name string) (string, string) {
	runes := []rune(name)
	lastIndex := strings.LastIndex(name, "-")
	return string(runes[0:lastIndex]), string(runes[(lastIndex + 1):len(runes)])
}

// DeploymentBuilder Handles construction of RD
type DeploymentBuilder interface {
	WithRadixApplication(ApplicationBuilder) DeploymentBuilder
	WithRadixDeployment(*v1.RadixDeployment) DeploymentBuilder
	WithImageTag(string) DeploymentBuilder
	WithAppName(string) DeploymentBuilder
	WithEnvironment(string) DeploymentBuilder
	WithCreated(time.Time) DeploymentBuilder
	WithComponent(DeployComponentBuilder) DeploymentBuilder
	WithComponents([]DeployComponentBuilder) DeploymentBuilder
	GetApplicationBuilder() ApplicationBuilder
	BuildRD() *v1.RadixDeployment
}

type deploymentBuilder struct {
	applicationBuilder ApplicationBuilder
	appName            string
	labels             map[string]string
	imageTag           string
	environment        string
	created            time.Time
	components         []DeployComponentBuilder
}

func (db *deploymentBuilder) WithRadixApplication(applicationBuilder ApplicationBuilder) DeploymentBuilder {
	db.applicationBuilder = applicationBuilder
	return db
}

func (db *deploymentBuilder) WithRadixDeployment(radixDeployment *v1.RadixDeployment) DeploymentBuilder {
	_, imageTag := getAppAndImagePairFromName(radixDeployment.Name)

	db.WithImageTag(imageTag)
	db.WithAppName(radixDeployment.Spec.AppName)
	db.WithEnvironment(radixDeployment.Spec.Environment)
	db.WithCreated(radixDeployment.CreationTimestamp.Time)
	return db
}

func (db *deploymentBuilder) WithAppName(appName string) DeploymentBuilder {
	db.labels["radixApp"] = appName

	if db.applicationBuilder != nil {
		db.applicationBuilder = db.applicationBuilder.WithAppName(appName)
	}

	db.appName = appName
	return db
}

func (db *deploymentBuilder) WithLabel(label, value string) DeploymentBuilder {
	db.labels[label] = value
	return db
}

func (db *deploymentBuilder) WithImageTag(imageTag string) DeploymentBuilder {
	db.imageTag = imageTag
	return db
}

func (db *deploymentBuilder) WithEnvironment(environment string) DeploymentBuilder {
	db.labels["env"] = environment
	db.environment = environment
	return db
}

func (db *deploymentBuilder) WithCreated(created time.Time) DeploymentBuilder {
	db.created = created
	return db
}

func (db *deploymentBuilder) WithComponent(component DeployComponentBuilder) DeploymentBuilder {
	db.components = append(db.components, component)
	return db
}

func (db *deploymentBuilder) WithComponents(components []DeployComponentBuilder) DeploymentBuilder {
	if db.applicationBuilder != nil {
		applicationComponents := make([]RadixApplicationComponentBuilder, 0)

		for _, comp := range components {
			applicationComponents = append(applicationComponents, NewApplicationComponentBuilder().
				WithName(comp.BuildComponent().Name))
		}

		db.applicationBuilder = db.applicationBuilder.WithComponents(applicationComponents)
	}

	db.components = components
	return db
}

func (db *deploymentBuilder) GetApplicationBuilder() ApplicationBuilder {
	if db.applicationBuilder != nil {
		return db.applicationBuilder
	}

	return nil
}

func (db *deploymentBuilder) BuildRD() *v1.RadixDeployment {
	var components = make([]v1.RadixDeployComponent, 0)
	for _, comp := range db.components {
		components = append(components, comp.BuildComponent())
	}

	radixDeployment := &v1.RadixDeployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixDeployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              getDeploymentName(db.appName, db.imageTag),
			Namespace:         getNamespaceForApplicationEnvironment(db.appName, db.environment),
			Labels:            db.labels,
			CreationTimestamp: metav1.Time{Time: db.created},
		},
		Spec: v1.RadixDeploymentSpec{
			AppName:     db.appName,
			Components:  components,
			Environment: db.environment,
		},
	}
	return radixDeployment
}

// NewDeploymentBuilder Constructor for deployment builder
func NewDeploymentBuilder() DeploymentBuilder {
	return &deploymentBuilder{
		labels:  make(map[string]string),
		created: time.Now(),
	}
}

// ARadixDeployment Constructor for deployment builder containing test data
func ARadixDeployment() DeploymentBuilder {
	builder := NewDeploymentBuilder().
		WithRadixApplication(ARadixApplication()).
		WithAppName("someapp").
		WithImageTag("imagetag").
		WithEnvironment("test").
		WithComponent(NewDeployComponentBuilder().
			WithImage("radixdev.azurecr.io/some-image:imagetag").
			WithName("app").
			WithPort("http", 8080).
			WithPublic(true).
			WithReplicas(1))

	return builder
}

// DeployComponentBuilder Handles construction of RD component
type DeployComponentBuilder interface {
	WithName(string) DeployComponentBuilder
	WithImage(string) DeployComponentBuilder
	WithPort(string, int32) DeployComponentBuilder
	WithEnvironmentVariable(string, string) DeployComponentBuilder
	WithPublic(bool) DeployComponentBuilder
	WithReplicas(int) DeployComponentBuilder
	WithSecrets([]string) DeployComponentBuilder
	BuildComponent() v1.RadixDeployComponent
}

type deployComponentBuilder struct {
	name                 string
	image                string
	ports                map[string]int32
	environmentVariables map[string]string
	public               bool
	replicas             int
	secrets              []string
}

func (dcb *deployComponentBuilder) WithName(name string) DeployComponentBuilder {
	dcb.name = name
	return dcb
}

func (dcb *deployComponentBuilder) WithImage(image string) DeployComponentBuilder {
	dcb.image = image
	return dcb
}

func (dcb *deployComponentBuilder) WithPort(name string, port int32) DeployComponentBuilder {
	dcb.ports[name] = port
	return dcb
}

func (dcb *deployComponentBuilder) WithPublic(public bool) DeployComponentBuilder {
	dcb.public = public
	return dcb
}

func (dcb *deployComponentBuilder) WithReplicas(replicas int) DeployComponentBuilder {
	dcb.replicas = replicas
	return dcb
}

func (dcb *deployComponentBuilder) WithEnvironmentVariable(name string, value string) DeployComponentBuilder {
	dcb.environmentVariables[name] = value
	return dcb
}

func (dcb *deployComponentBuilder) WithSecrets(secrets []string) DeployComponentBuilder {
	dcb.secrets = secrets
	return dcb
}

func (dcb *deployComponentBuilder) BuildComponent() v1.RadixDeployComponent {
	componentPorts := make([]v1.ComponentPort, 0)
	for key, value := range dcb.ports {
		componentPorts = append(componentPorts, v1.ComponentPort{Name: key, Port: value})
	}

	return v1.RadixDeployComponent{
		Image:                dcb.image,
		Name:                 dcb.name,
		Ports:                componentPorts,
		Public:               dcb.public,
		Replicas:             dcb.replicas,
		Secrets:              dcb.secrets,
		EnvironmentVariables: dcb.environmentVariables,
	}
}

// NewDeployComponentBuilder Constructor for component builder
func NewDeployComponentBuilder() DeployComponentBuilder {
	return &deployComponentBuilder{
		ports:                make(map[string]int32),
		environmentVariables: make(map[string]string),
	}
}
