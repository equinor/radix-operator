package models

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
)

// ApplicationRegistrationBuilder Handles construction of DTO
type ApplicationRegistrationBuilder interface {
	WithName(name string) ApplicationRegistrationBuilder
	WithAppID(appID string) ApplicationRegistrationBuilder
	WithOwner(owner string) ApplicationRegistrationBuilder
	WithCreator(creator string) ApplicationRegistrationBuilder
	WithRepository(string) ApplicationRegistrationBuilder
	WithSharedSecret(string) ApplicationRegistrationBuilder
	WithAdGroups([]string) ApplicationRegistrationBuilder
	WithAdUsers([]string) ApplicationRegistrationBuilder
	WithReaderAdGroups([]string) ApplicationRegistrationBuilder
	WithReaderAdUsers([]string) ApplicationRegistrationBuilder
	WithCloneURL(string) ApplicationRegistrationBuilder
	WithConfigBranch(string) ApplicationRegistrationBuilder
	WithConfigurationItem(string) ApplicationRegistrationBuilder
	WithRadixConfigFullName(string) ApplicationRegistrationBuilder
	WithAppRegistration(*ApplicationRegistration) ApplicationRegistrationBuilder
	WithRadixRegistration(*v1.RadixRegistration) ApplicationRegistrationBuilder
	Build() ApplicationRegistration
	BuildRR() (*v1.RadixRegistration, error)
}

type applicationBuilder struct {
	name                string
	appID               string
	owner               string
	creator             string
	repository          string
	sharedSecret        string
	adGroups            []string
	readerAdGroups      []string
	cloneURL            string
	configBranch        string
	configurationItem   string
	radixConfigFullName string
	adUsers             []string
	readerAdUsers       []string
}

func (rb *applicationBuilder) WithAppRegistration(appRegistration *ApplicationRegistration) ApplicationRegistrationBuilder {
	rb.WithName(appRegistration.Name)
	rb.WithAppID(appRegistration.AppID)
	rb.WithRepository(appRegistration.Repository)
	rb.WithSharedSecret(appRegistration.SharedSecret)
	rb.WithAdGroups(appRegistration.AdGroups)
	rb.WithAdUsers(appRegistration.AdUsers)
	rb.WithReaderAdGroups(appRegistration.ReaderAdGroups)
	rb.WithReaderAdUsers(appRegistration.ReaderAdUsers)
	rb.WithOwner(appRegistration.Owner)
	rb.WithConfigBranch(appRegistration.ConfigBranch)
	rb.WithRadixConfigFullName(appRegistration.RadixConfigFullName)
	rb.WithConfigurationItem(appRegistration.ConfigurationItem)
	return rb
}

func (rb *applicationBuilder) WithRadixRegistration(radixRegistration *v1.RadixRegistration) ApplicationRegistrationBuilder {
	rb.WithName(radixRegistration.Name)
	rb.WithAppID(radixRegistration.Spec.AppID.String())
	rb.WithCloneURL(radixRegistration.Spec.CloneURL)
	rb.WithSharedSecret(radixRegistration.Spec.SharedSecret)
	rb.WithAdGroups(radixRegistration.Spec.AdGroups)
	rb.WithAdUsers(radixRegistration.Spec.AdUsers)
	rb.WithReaderAdGroups(radixRegistration.Spec.ReaderAdGroups)
	rb.WithReaderAdUsers(radixRegistration.Spec.ReaderAdUsers)
	rb.WithOwner(radixRegistration.Spec.Owner)
	rb.WithCreator(radixRegistration.Spec.Creator)
	rb.WithConfigBranch(radixRegistration.Spec.ConfigBranch)
	rb.WithRadixConfigFullName(radixRegistration.Spec.RadixConfigFullName)
	rb.WithConfigurationItem(radixRegistration.Spec.ConfigurationItem)

	// Private part of key should never be returned
	return rb
}

func (rb *applicationBuilder) WithName(name string) ApplicationRegistrationBuilder {
	rb.name = name
	return rb
}

func (rb *applicationBuilder) WithAppID(appID string) ApplicationRegistrationBuilder {
	rb.appID = appID
	return rb
}

func (rb *applicationBuilder) WithOwner(owner string) ApplicationRegistrationBuilder {
	rb.owner = owner
	return rb
}

func (rb *applicationBuilder) WithCreator(creator string) ApplicationRegistrationBuilder {
	rb.creator = creator
	return rb
}

func (rb *applicationBuilder) WithRepository(repository string) ApplicationRegistrationBuilder {
	rb.repository = repository
	return rb
}

func (rb *applicationBuilder) WithCloneURL(cloneURL string) ApplicationRegistrationBuilder {
	rb.cloneURL = cloneURL
	return rb
}

func (rb *applicationBuilder) WithSharedSecret(sharedSecret string) ApplicationRegistrationBuilder {
	rb.sharedSecret = sharedSecret
	return rb
}

func (rb *applicationBuilder) WithAdGroups(adGroups []string) ApplicationRegistrationBuilder {
	rb.adGroups = adGroups
	return rb
}

func (rb *applicationBuilder) WithAdUsers(adUsers []string) ApplicationRegistrationBuilder {
	rb.adUsers = adUsers
	return rb
}

func (rb *applicationBuilder) WithReaderAdGroups(readerAdGroups []string) ApplicationRegistrationBuilder {
	rb.readerAdGroups = readerAdGroups
	return rb
}

func (rb *applicationBuilder) WithReaderAdUsers(readerAdUsers []string) ApplicationRegistrationBuilder {
	rb.readerAdUsers = readerAdUsers
	return rb
}

func (rb *applicationBuilder) WithConfigBranch(configBranch string) ApplicationRegistrationBuilder {
	rb.configBranch = configBranch
	return rb
}

func (rb *applicationBuilder) WithConfigurationItem(ci string) ApplicationRegistrationBuilder {
	rb.configurationItem = ci
	return rb
}

func (rb *applicationBuilder) WithRadixConfigFullName(fullName string) ApplicationRegistrationBuilder {
	rb.radixConfigFullName = fullName
	return rb
}

func (rb *applicationBuilder) Build() ApplicationRegistration {
	repository := rb.repository
	if repository == "" {
		repository = crdUtils.GetGithubRepositoryURLFromCloneURL(rb.cloneURL)
	}

	return ApplicationRegistration{
		Name:                rb.name,
		AppID:               rb.appID,
		Repository:          repository,
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
	}
}

func (rb *applicationBuilder) BuildRR() (*v1.RadixRegistration, error) {
	builder := crdUtils.NewRegistrationBuilder()

	radixRegistration := builder.
		WithName(rb.name).
		WithAppID(rb.appID).
		WithRepository(rb.repository).
		WithSharedSecret(rb.sharedSecret).
		WithAdGroups(rb.adGroups).
		WithAdUsers(rb.adUsers).
		WithReaderAdGroups(rb.readerAdGroups).
		WithReaderAdUsers(rb.readerAdUsers).
		WithOwner(rb.owner).
		WithCreator(rb.creator).
		WithConfigBranch(rb.configBranch).
		WithRadixConfigFullName(rb.radixConfigFullName).
		WithConfigurationItem(rb.configurationItem).
		BuildRR()

	return radixRegistration, nil
}

// NewApplicationRegistrationBuilder Constructor for application builder
func NewApplicationRegistrationBuilder() ApplicationRegistrationBuilder {
	return &applicationBuilder{}
}
