package utils

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// DeploymentBuilder Handles construction of RD
type DeploymentBuilder interface {
	WithRadixApplication(ApplicationBuilder) DeploymentBuilder
	WithRadixDeployment(*v1.RadixDeployment) DeploymentBuilder
	WithDeploymentName(string) DeploymentBuilder
	WithImageTag(string) DeploymentBuilder
	WithAppName(string) DeploymentBuilder
	WithLabel(label, value string) DeploymentBuilder
	WithEnvironment(string) DeploymentBuilder
	WithCreated(time.Time) DeploymentBuilder
	WithUID(types.UID) DeploymentBuilder
	WithCondition(v1.RadixDeployCondition) DeploymentBuilder
	WithActiveFrom(time.Time) DeploymentBuilder
	WithActiveTo(time.Time) DeploymentBuilder
	WithEmptyStatus() DeploymentBuilder
	WithComponent(DeployComponentBuilder) DeploymentBuilder
	WithComponents(...DeployComponentBuilder) DeploymentBuilder
	GetApplicationBuilder() ApplicationBuilder
	BuildRD() *v1.RadixDeployment
}

// DeploymentBuilderStruct Holds instance variables
type DeploymentBuilderStruct struct {
	applicationBuilder ApplicationBuilder
	DeploymentName     string
	AppName            string
	emptyStatus        bool
	Labels             map[string]string
	ImageTag           string
	Environment        string
	Created            time.Time
	Condition          v1.RadixDeployCondition
	ActiveFrom         metav1.Time
	ActiveTo           metav1.Time
	ResourceVersion    string
	UID                types.UID
	components         []DeployComponentBuilder
}

// WithDeploymentName Sets name of the deployment
func (db *DeploymentBuilderStruct) WithDeploymentName(name string) DeploymentBuilder {
	db.DeploymentName = name
	return db
}

// WithRadixApplication Links to RA builder
func (db *DeploymentBuilderStruct) WithRadixApplication(applicationBuilder ApplicationBuilder) DeploymentBuilder {
	db.applicationBuilder = applicationBuilder
	return db
}

// WithEmptyStatus Indicates that the RD has no reconciled status
func (db *DeploymentBuilderStruct) WithEmptyStatus() DeploymentBuilder {
	db.emptyStatus = true
	return db
}

// WithRadixDeployment Reverse engineers RD
func (db *DeploymentBuilderStruct) WithRadixDeployment(radixDeployment *v1.RadixDeployment) DeploymentBuilder {
	_, imageTag := GetAppAndTagPairFromName(radixDeployment.Name)

	db.WithImageTag(imageTag)
	db.WithAppName(radixDeployment.Spec.AppName)
	db.WithEnvironment(radixDeployment.Spec.Environment)
	db.WithCreated(radixDeployment.CreationTimestamp.Time)
	return db
}

// WithAppName Sets app name
func (db *DeploymentBuilderStruct) WithAppName(appName string) DeploymentBuilder {
	db.Labels["radix-app"] = appName

	if db.applicationBuilder != nil {
		db.applicationBuilder = db.applicationBuilder.WithAppName(appName)
	}

	db.AppName = appName
	return db
}

// WithLabel Appends label
func (db *DeploymentBuilderStruct) WithLabel(label, value string) DeploymentBuilder {
	db.Labels[label] = value
	return db
}

// WithImageTag Sets deployment tag to be appended to name
func (db *DeploymentBuilderStruct) WithImageTag(imageTag string) DeploymentBuilder {
	db.Labels[kube.RadixImageTagLabel] = imageTag
	db.ImageTag = imageTag
	return db
}

// WithEnvironment Sets environment name
func (db *DeploymentBuilderStruct) WithEnvironment(environment string) DeploymentBuilder {
	db.Labels[kube.RadixEnvLabel] = environment
	db.Environment = environment

	if db.applicationBuilder != nil {
		db.applicationBuilder = db.applicationBuilder.WithEnvironmentNoBranch(environment)
	}

	return db
}

// WithUID Sets UUID
func (db *DeploymentBuilderStruct) WithUID(uid types.UID) DeploymentBuilder {
	db.UID = uid
	return db
}

// WithCreated Sets timestamp
func (db *DeploymentBuilderStruct) WithCreated(created time.Time) DeploymentBuilder {
	db.Created = created
	return db
}

// WithCondition Sets the condition of the deployment
func (db *DeploymentBuilderStruct) WithCondition(condition v1.RadixDeployCondition) DeploymentBuilder {
	db.Condition = condition
	return db
}

// WithActiveFrom Sets active from
func (db *DeploymentBuilderStruct) WithActiveFrom(activeFrom time.Time) DeploymentBuilder {
	db.ActiveFrom = metav1.NewTime(activeFrom.UTC())
	return db
}

// WithActiveTo Sets active to
func (db *DeploymentBuilderStruct) WithActiveTo(activeTo time.Time) DeploymentBuilder {
	db.ActiveTo = metav1.NewTime(activeTo.UTC())
	return db
}

// WithComponent Appends component to list of components
func (db *DeploymentBuilderStruct) WithComponent(component DeployComponentBuilder) DeploymentBuilder {
	db.components = append(db.components, component)
	return db
}

// WithComponents Sets list of components
func (db *DeploymentBuilderStruct) WithComponents(components ...DeployComponentBuilder) DeploymentBuilder {
	if db.applicationBuilder != nil {
		applicationComponents := make([]RadixApplicationComponentBuilder, 0)

		for _, comp := range components {
			applicationComponents = append(applicationComponents, NewApplicationComponentBuilder().
				WithName(comp.BuildComponent().Name))
		}

		db.applicationBuilder = db.applicationBuilder.WithComponents(applicationComponents...)
	}

	db.components = components
	return db
}

// GetApplicationBuilder Obtains the builder for the corresponding RA, if exists (used for testing)
func (db *DeploymentBuilderStruct) GetApplicationBuilder() ApplicationBuilder {
	if db.applicationBuilder != nil {
		return db.applicationBuilder
	}

	return nil
}

// BuildRD Builds RD structure based on set variables
func (db *DeploymentBuilderStruct) BuildRD() *v1.RadixDeployment {
	components := make([]v1.RadixDeployComponent, 0)
	for _, comp := range db.components {
		components = append(components, comp.BuildComponent())
	}
	deployName := db.DeploymentName
	if deployName == "" {
		deployName = GetDeploymentName(db.AppName, db.Environment, db.ImageTag)
	}
	status := v1.RadixDeployStatus{}
	if !db.emptyStatus {
		status = v1.RadixDeployStatus{
			Condition:  db.Condition,
			ActiveFrom: db.ActiveFrom,
			ActiveTo:   db.ActiveTo,
		}
	}

	radixDeployment := &v1.RadixDeployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixDeployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              deployName,
			Namespace:         GetEnvironmentNamespace(db.AppName, db.Environment),
			Labels:            db.Labels,
			CreationTimestamp: metav1.Time{Time: db.Created},
			ResourceVersion:   db.ResourceVersion,
			UID:               db.UID,
		},
		Spec: v1.RadixDeploymentSpec{
			AppName:     db.AppName,
			Components:  components,
			Environment: db.Environment,
		},
		Status: status,
	}
	return radixDeployment
}

// NewDeploymentBuilder Constructor for deployment builder
func NewDeploymentBuilder() DeploymentBuilder {
	return &DeploymentBuilderStruct{
		Labels:          make(map[string]string),
		Created:         time.Now().UTC(),
		Condition:       v1.DeploymentActive,
		ActiveFrom:      metav1.NewTime(time.Now().UTC()),
		ResourceVersion: strconv.Itoa(rand.Intn(100)),
	}
}

// ARadixDeployment Constructor for deployment builder containing test data
func ARadixDeployment() DeploymentBuilder {
	replicas := 1
	builder := NewDeploymentBuilder().
		WithRadixApplication(ARadixApplication()).
		WithAppName("someapp").
		WithImageTag("imagetag").
		WithEnvironment("test").
		WithComponent(NewDeployComponentBuilder().
			WithImage("radixdev.azurecr.io/some-image:imagetag").
			WithName("app").
			WithPort("http", 8080).
			WithPublicPort("http").
			WithReplicas(&replicas))

	return builder
}

// DeployComponentBuilder Handles construction of RD component
type DeployComponentBuilder interface {
	WithName(string) DeployComponentBuilder
	WithImage(string) DeployComponentBuilder
	WithPort(string, int32) DeployComponentBuilder
	WithEnvironmentVariable(string, string) DeployComponentBuilder
	WithEnvironmentVariables(map[string]string) DeployComponentBuilder
	// Deprecated: For backwards comptibility WithPublic is still supported, new code should use WithPublicPort instead
	WithPublic(bool) DeployComponentBuilder
	WithPublicPort(string) DeployComponentBuilder
	WithMonitoring(bool) DeployComponentBuilder
	WithReplicas(*int) DeployComponentBuilder
	WithResourceRequestsOnly(map[string]string) DeployComponentBuilder
	WithResource(map[string]string, map[string]string) DeployComponentBuilder
	WithIngressConfiguration(...string) DeployComponentBuilder
	WithSecrets([]string) DeployComponentBuilder
	WithDNSAppAlias(bool) DeployComponentBuilder
	WithDNSExternalAlias(string) DeployComponentBuilder
	BuildComponent() v1.RadixDeployComponent
}

type deployComponentBuilder struct {
	name                 string
	image                string
	ports                map[string]int32
	environmentVariables map[string]string
	// Deprecated: For backwards comptibility public is still supported, new code should use publicPort instead
	public               bool
	publicPort           string
	monitoring           bool
	replicas             *int
	ingressConfiguration []string
	secrets              []string
	dnsappalias          bool
	externalAppAlias     []string
	resources            v1.ResourceRequirements
}

func (dcb *deployComponentBuilder) WithResourceRequestsOnly(request map[string]string) DeployComponentBuilder {
	dcb.resources = v1.ResourceRequirements{
		Requests: request,
	}
	return dcb
}

func (dcb *deployComponentBuilder) WithResource(request map[string]string, limit map[string]string) DeployComponentBuilder {
	dcb.resources = v1.ResourceRequirements{
		Limits:   limit,
		Requests: request,
	}
	return dcb
}

func (dcb *deployComponentBuilder) WithName(name string) DeployComponentBuilder {
	dcb.name = name
	return dcb
}

func (dcb *deployComponentBuilder) WithDNSAppAlias(createDNSAppAlias bool) DeployComponentBuilder {
	dcb.dnsappalias = createDNSAppAlias
	return dcb
}

func (dcb *deployComponentBuilder) WithDNSExternalAlias(alias string) DeployComponentBuilder {
	if dcb.externalAppAlias == nil {
		dcb.externalAppAlias = make([]string, 0)
	}

	dcb.externalAppAlias = append(dcb.externalAppAlias, alias)
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

// Deprecated: For backwards comptibility WithPublic is still supported, new code should use WithPublicPort instead
func (dcb *deployComponentBuilder) WithPublic(public bool) DeployComponentBuilder {
	dcb.public = public
	return dcb
}

func (dcb *deployComponentBuilder) WithPublicPort(publicPort string) DeployComponentBuilder {
	dcb.publicPort = publicPort
	return dcb
}

func (dcb *deployComponentBuilder) WithMonitoring(monitoring bool) DeployComponentBuilder {
	dcb.monitoring = monitoring
	return dcb
}

func (dcb *deployComponentBuilder) WithReplicas(replicas *int) DeployComponentBuilder {
	dcb.replicas = replicas
	return dcb
}

func (dcb *deployComponentBuilder) WithEnvironmentVariable(name string, value string) DeployComponentBuilder {
	dcb.environmentVariables[name] = value
	return dcb
}

func (dcb *deployComponentBuilder) WithEnvironmentVariables(environmentVariables map[string]string) DeployComponentBuilder {
	dcb.environmentVariables = environmentVariables
	return dcb
}

func (dcb *deployComponentBuilder) WithSecrets(secrets []string) DeployComponentBuilder {
	dcb.secrets = secrets
	return dcb
}

func (dcb *deployComponentBuilder) WithIngressConfiguration(ingressConfiguration ...string) DeployComponentBuilder {
	dcb.ingressConfiguration = ingressConfiguration
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
		PublicPort:           dcb.publicPort,
		Monitoring:           dcb.monitoring,
		Replicas:             dcb.replicas,
		Secrets:              dcb.secrets,
		IngressConfiguration: dcb.ingressConfiguration,
		EnvironmentVariables: dcb.environmentVariables,
		DNSAppAlias:          dcb.dnsappalias,
		DNSExternalAlias:     dcb.externalAppAlias,
		Resources:            dcb.resources,
	}
}

// NewDeployComponentBuilder Constructor for component builder
func NewDeployComponentBuilder() DeployComponentBuilder {
	return &deployComponentBuilder{
		ports:                make(map[string]int32),
		environmentVariables: make(map[string]string),
	}
}
