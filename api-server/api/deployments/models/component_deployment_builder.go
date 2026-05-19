package models

import (
	"fmt"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// DeploymentItemBuilder Builds DTOs
type DeploymentItemBuilder interface {
	WithRadixDeployment(*v1.RadixDeployment) DeploymentItemBuilder
	Build() (*DeploymentItem, error)
}

type deploymentItemBuilder struct {
	radixDeployment *v1.RadixDeployment
}

// NewDeploymentItemBuilder Constructor for application DeploymentItemBuilder
func NewDeploymentItemBuilder() DeploymentItemBuilder {
	return &deploymentItemBuilder{}
}

func (b *deploymentItemBuilder) WithRadixDeployment(rd *v1.RadixDeployment) DeploymentItemBuilder {
	b.radixDeployment = rd
	return b
}

func (b *deploymentItemBuilder) Build() (*DeploymentItem, error) {
	if b.radixDeployment == nil {
		return nil, fmt.Errorf("RadixDeployment is empty")
	}
	var activeTo *time.Time
	if !b.radixDeployment.Status.ActiveTo.IsZero() {
		activeTo = &b.radixDeployment.Status.ActiveTo.Time
	}

	return &DeploymentItem{
		Name:          b.radixDeployment.GetName(),
		ActiveFrom:    b.radixDeployment.Status.ActiveFrom.Time,
		ActiveTo:      activeTo,
		GitCommitHash: b.radixDeployment.GetLabels()[kube.RadixCommitLabel],
	}, nil
}
