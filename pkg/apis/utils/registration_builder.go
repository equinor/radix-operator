package utils

import (
	"time"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/oklog/ulid/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// RegistrationBuilder Handles construction of RR or applicationRegistration
type RegistrationBuilder interface {
	WithUID(types.UID) RegistrationBuilder
	WithAppID(string) RegistrationBuilder
	WithName(name string) RegistrationBuilder
	WithRepository(string) RegistrationBuilder
	WithSharedSecret(string) RegistrationBuilder
	WithAdGroups([]string) RegistrationBuilder
	WithAdUsers([]string) RegistrationBuilder
	WithCloneURL(string) RegistrationBuilder
	WithOwner(string) RegistrationBuilder
	WithCreator(string) RegistrationBuilder
	WithEmptyStatus() RegistrationBuilder
	WithConfigBranch(string) RegistrationBuilder
	WithRadixConfigFullName(string) RegistrationBuilder
	WithConfigurationItem(string) RegistrationBuilder
	WithRadixRegistration(*v1.RadixRegistration) RegistrationBuilder
	WithReaderAdGroups([]string) RegistrationBuilder
	WithReaderAdUsers([]string) RegistrationBuilder
	BuildRR() *v1.RadixRegistration
}

// RegistrationBuilderStruct Instance variables
type RegistrationBuilderStruct struct {
	uid                 types.UID
	appID               string
	name                string
	repository          string
	sharedSecret        string
	adGroups            []string
	adUsers             []string
	cloneURL            string
	owner               string
	creator             string
	emptyStatus         bool
	configBranch        string
	radixConfigFullName string
	configurationItem   string
	readerAdGroups      []string
	readerAdUsers       []string
}

// WithRadixRegistration Re-enginers a builder from a registration
func (rb *RegistrationBuilderStruct) WithRadixRegistration(radixRegistration *v1.RadixRegistration) RegistrationBuilder {
	rb.WithAppID(radixRegistration.Spec.AppID.String())
	rb.WithName(radixRegistration.Name)
	rb.WithCloneURL(radixRegistration.Spec.CloneURL)
	rb.WithSharedSecret(radixRegistration.Spec.SharedSecret)
	rb.WithAdGroups(radixRegistration.Spec.AdGroups)
	rb.WithAdUsers(radixRegistration.Spec.AdUsers)
	rb.WithReaderAdGroups(radixRegistration.Spec.ReaderAdGroups)
	rb.WithReaderAdUsers(radixRegistration.Spec.ReaderAdUsers)
	rb.WithOwner(radixRegistration.Spec.Owner)
	rb.WithCreator(radixRegistration.Spec.Creator)
	rb.WithRadixConfigFullName(radixRegistration.Spec.RadixConfigFullName)
	rb.WithConfigurationItem(radixRegistration.Spec.ConfigurationItem)
	return rb
}

// WithUID Sets UID
func (rb *RegistrationBuilderStruct) WithUID(uid types.UID) RegistrationBuilder {
	rb.uid = uid
	return rb
}

// WithAppID Sets Radix AppID, if "" it will generate a new ULID
func (rb *RegistrationBuilderStruct) WithAppID(id string) RegistrationBuilder {
	rb.appID = id
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

// WithAdUsers Sets ad user
func (rb *RegistrationBuilderStruct) WithAdUsers(adUsers []string) RegistrationBuilder {
	rb.adUsers = adUsers
	return rb
}

// WithReaderAdGroups Sets reader ad group
func (rb *RegistrationBuilderStruct) WithReaderAdGroups(readerAdGroups []string) RegistrationBuilder {
	rb.readerAdGroups = readerAdGroups
	return rb
}

// WithReaderAdUsers Sets reader ad user
func (rb *RegistrationBuilderStruct) WithReaderAdUsers(readerAdUsers []string) RegistrationBuilder {
	rb.readerAdUsers = readerAdUsers
	return rb
}

// WithEmptyStatus Indicates that the RR has no reconciled status
func (rb *RegistrationBuilderStruct) WithEmptyStatus() RegistrationBuilder {
	rb.emptyStatus = true
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

	appId := rb.appID
	if appId == "" {
		appId = ulid.Make().String()
	}

	status := v1.RadixRegistrationStatus{}
	if !rb.emptyStatus {
		status = v1.RadixRegistrationStatus{
			Reconciled: metav1.NewTime(time.Now().UTC()),
		}
	}

	radixRegistration := &v1.RadixRegistration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.Identifier(),
			Kind:       v1.KindRadixRegistration,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: rb.name,
			UID:  rb.uid,
		},
		Spec: v1.RadixRegistrationSpec{
			AppID:               v1.ULID{ULID: ulid.MustParse(appId)},
			CloneURL:            cloneURL,
			SharedSecret:        rb.sharedSecret,
			AdGroups:            rb.adGroups,
			AdUsers:             rb.adUsers,
			ReaderAdGroups:      rb.readerAdGroups,
			ReaderAdUsers:       rb.readerAdUsers,
			Owner:               rb.owner,
			Creator:             rb.creator,
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
		WithAppID("00000000000000000000000001").
		WithName("anyapp").
		WithCloneURL("git@github.com:equinor/anyapp").
		WithSharedSecret("NotSoSecret").
		WithUID("1234-5678").
		WithAdGroups([]string{"604bad73-c53b-4a95-ab17-d7953f75c8c3"}).
		WithReaderAdGroups([]string{"40edc80d-0047-450d-b71a-970e6bb61d64"}).
		WithOwner("radix@equinor.com").
		WithCreator("radix@equinor.com").
		WithConfigBranch("main")

	return builder
}
