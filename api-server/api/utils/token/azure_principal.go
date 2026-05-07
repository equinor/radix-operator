package token

import (
	"context"
	"fmt"

	"github.com/auth0/go-jwt-middleware/v2/validator"
)

type azureClaims struct {
	ObjectId       string `json:"oid,omitempty"`
	Upn            string `json:"upn,omitempty"`
	AppDisplayName string `json:"app_displayname,omitempty"`
	AppId          string `json:"appid,omitempty"`
}

func (c *azureClaims) Validate(_ context.Context) error {
	return nil
}

type azurePrincipal struct {
	token       string
	claims      validator.RegisteredClaims
	azureClaims azureClaims
}

func (p *azurePrincipal) Token() string {
	return p.token
}
func (p *azurePrincipal) IsAuthenticated() bool {
	return true
}
func (p *azurePrincipal) Id() string {
	if p.azureClaims.ObjectId != "" {
		return fmt.Sprintf("oid:%s", p.azureClaims.ObjectId)
	}

	return fmt.Sprintf("sub:%s", p.claims.Subject)
}

func (p *azurePrincipal) Name() string {
	if p.azureClaims.Upn != "" {
		return p.azureClaims.Upn
	}

	if p.azureClaims.AppDisplayName != "" {
		return p.azureClaims.AppDisplayName
	}

	if p.azureClaims.AppId != "" {
		return p.azureClaims.AppId
	}

	if p.azureClaims.ObjectId != "" {
		return p.azureClaims.ObjectId
	}

	return p.claims.Subject
}
