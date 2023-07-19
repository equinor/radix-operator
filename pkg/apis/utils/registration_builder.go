package utils

import (
	"strings"
	"time"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// RegistrationBuilder Handles construction of RR or applicationRegistration
type RegistrationBuilder interface {
	WithUID(types.UID) RegistrationBuilder
	WithName(name string) RegistrationBuilder
	WithRepository(string) RegistrationBuilder
	WithSharedSecret(string) RegistrationBuilder
	WithAdGroups([]string) RegistrationBuilder
	WithPublicKey(string) RegistrationBuilder
	WithPrivateKey(string) RegistrationBuilder
	WithCloneURL(string) RegistrationBuilder
	WithOwner(string) RegistrationBuilder
	WithCreator(string) RegistrationBuilder
	WithEmptyStatus() RegistrationBuilder
	WithMachineUser(bool) RegistrationBuilder
	WithWBS(string) RegistrationBuilder
	WithConfigBranch(string) RegistrationBuilder
	WithRadixConfigFullName(string) RegistrationBuilder
	WithConfigurationItem(string) RegistrationBuilder
	WithRadixRegistration(*v1.RadixRegistration) RegistrationBuilder
	WithReaderAdGroups([]string) RegistrationBuilder
	BuildRR() *v1.RadixRegistration
}

// RegistrationBuilderStruct Instance variables
type RegistrationBuilderStruct struct {
	uid                 types.UID
	name                string
	repository          string
	sharedSecret        string
	adGroups            []string
	publicKey           string
	privateKey          string
	cloneURL            string
	owner               string
	creator             string
	emptyStatus         bool
	machineUser         bool
	wbs                 string
	configBranch        string
	radixConfigFullName string
	configurationItem   string
	readerAdGroups      []string
}

// WithRadixRegistration Re-enginers a builder from a registration
func (rb *RegistrationBuilderStruct) WithRadixRegistration(radixRegistration *v1.RadixRegistration) RegistrationBuilder {
	rb.WithName(radixRegistration.Name)
	rb.WithCloneURL(radixRegistration.Spec.CloneURL)
	rb.WithSharedSecret(radixRegistration.Spec.SharedSecret)
	rb.WithAdGroups(radixRegistration.Spec.AdGroups)
	rb.WithReaderAdGroups(radixRegistration.Spec.ReaderAdGroups)
	rb.WithPublicKey(radixRegistration.Spec.DeployKeyPublic)
	rb.WithPrivateKey(radixRegistration.Spec.DeployKey)
	rb.WithOwner(radixRegistration.Spec.Owner)
	rb.WithCreator(radixRegistration.Spec.Creator)
	rb.WithMachineUser(radixRegistration.Spec.MachineUser)
	rb.WithWBS(radixRegistration.Spec.WBS)
	rb.WithRadixConfigFullName(radixRegistration.Spec.RadixConfigFullName)
	rb.WithConfigurationItem(radixRegistration.Spec.ConfigurationItem)
	return rb
}

// WithUID Sets UID
func (rb *RegistrationBuilderStruct) WithUID(uid types.UID) RegistrationBuilder {
	rb.uid = uid
	return rb
}

// WithName Sets name
func (rb *RegistrationBuilderStruct) WithName(name string) RegistrationBuilder {
	rb.name = name
	return rb
}

// WithOwner set owner
func (rb *RegistrationBuilderStruct) WithOwner(owner string) RegistrationBuilder {
	rb.owner = owner
	return rb
}

// WithCreator set creator
func (rb *RegistrationBuilderStruct) WithCreator(creator string) RegistrationBuilder {
	rb.creator = creator
	return rb
}

// WithRepository Sets repository
func (rb *RegistrationBuilderStruct) WithRepository(repository string) RegistrationBuilder {
	rb.repository = repository
	return rb
}

// WithCloneURL Sets clone url
func (rb *RegistrationBuilderStruct) WithCloneURL(cloneURL string) RegistrationBuilder {
	rb.cloneURL = cloneURL
	return rb
}

// WithSharedSecret Sets shared secret
func (rb *RegistrationBuilderStruct) WithSharedSecret(sharedSecret string) RegistrationBuilder {
	rb.sharedSecret = sharedSecret
	return rb
}

// WithAdGroups Sets ad group
func (rb *RegistrationBuilderStruct) WithAdGroups(adGroups []string) RegistrationBuilder {
	rb.adGroups = adGroups
	return rb
}

// WithReaderAdGroups Sets reader ad group
func (rb *RegistrationBuilderStruct) WithReaderAdGroups(readerAdGroups []string) RegistrationBuilder {
	rb.readerAdGroups = readerAdGroups
	return rb
}

// WithPublicKey Sets public key
func (rb *RegistrationBuilderStruct) WithPublicKey(publicKey string) RegistrationBuilder {
	rb.publicKey = strings.TrimSuffix(publicKey, "\n")
	return rb
}

// WithPrivateKey Sets private key
func (rb *RegistrationBuilderStruct) WithPrivateKey(privateKey string) RegistrationBuilder {
	rb.privateKey = privateKey
	return rb
}

// WithEmptyStatus Indicates that the RR has no reconciled status
func (rb *RegistrationBuilderStruct) WithEmptyStatus() RegistrationBuilder {
	rb.emptyStatus = true
	return rb
}

// WithMachineUser Indicates that a machine user service account should be created for this application
func (rb *RegistrationBuilderStruct) WithMachineUser(machineUser bool) RegistrationBuilder {
	rb.machineUser = machineUser
	return rb
}

// WithWBS Sets WBS
func (rb *RegistrationBuilderStruct) WithWBS(wbs string) RegistrationBuilder {
	rb.wbs = wbs
	return rb
}

// WithConfigBranch Sets ConfigBranch
func (rb *RegistrationBuilderStruct) WithConfigBranch(configBranch string) RegistrationBuilder {
	rb.configBranch = configBranch
	return rb
}

// WithRadixConfigFullName Sets RadixConfigFullName
func (rb *RegistrationBuilderStruct) WithRadixConfigFullName(fullName string) RegistrationBuilder {
	rb.radixConfigFullName = fullName
	return rb
}

// WithConfigBranch Sets ApplicationId
func (rb *RegistrationBuilderStruct) WithConfigurationItem(ci string) RegistrationBuilder {
	rb.configurationItem = ci
	return rb
}

// BuildRR Builds the radix registration
func (rb *RegistrationBuilderStruct) BuildRR() *v1.RadixRegistration {
	cloneURL := rb.cloneURL
	if cloneURL == "" {
		cloneURL = GetGithubCloneURLFromRepo(rb.repository)
	}

	status := v1.RadixRegistrationStatus{}
	if !rb.emptyStatus {
		status = v1.RadixRegistrationStatus{
			Reconciled: metav1.NewTime(time.Now().UTC()),
		}
	}

	radixRegistration := &v1.RadixRegistration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixRegistration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: rb.name,
			UID:  rb.uid,
		},
		Spec: v1.RadixRegistrationSpec{
			CloneURL:            cloneURL,
			SharedSecret:        rb.sharedSecret,
			DeployKey:           rb.privateKey,
			DeployKeyPublic:     rb.publicKey,
			AdGroups:            rb.adGroups,
			ReaderAdGroups:      rb.readerAdGroups,
			Owner:               rb.owner,
			Creator:             rb.creator,
			MachineUser:         rb.machineUser,
			WBS:                 rb.wbs,
			ConfigBranch:        rb.configBranch,
			RadixConfigFullName: rb.radixConfigFullName,
			ConfigurationItem:   rb.configurationItem,
		},
		Status: status,
	}
	return radixRegistration
}

// NewRegistrationBuilder Constructor for registration builder
func NewRegistrationBuilder() RegistrationBuilder {
	return &RegistrationBuilderStruct{}
}

// ARadixRegistration Constructor for registration builder containing test data
func ARadixRegistration() RegistrationBuilder {
	builder := NewRegistrationBuilder().
		WithName("anyapp").
		WithCloneURL("git@github.com:equinor/anyapp").
		WithSharedSecret("NotSoSecret").
		WithUID("1234-5678").
		WithAdGroups([]string{"604bad73-c53b-4a95-ab17-d7953f75c8c3"}).
		WithReaderAdGroups([]string{"40edc80d-0047-450d-b71a-970e6bb61d64"}).
		WithOwner("radix@equinor.com").
		WithCreator("radix@equinor.com").
		WithWBS("A.BCD.00.999").
		WithConfigBranch("main")

	return builder
}
