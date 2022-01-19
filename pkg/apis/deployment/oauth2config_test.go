package deployment

import (
	// "os"
	// "testing"

	// "github.com/equinor/radix-operator/pkg/apis/defaults"
	// v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	// "github.com/equinor/radix-operator/pkg/apis/test"
	// "github.com/equinor/radix-operator/pkg/apis/utils"
	// "github.com/stretchr/testify/suite"

	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/stretchr/testify/suite"
)

type oauth2ConfigFuncImplSuite struct {
	suite.Suite
}

func TestOAuth2ConfigFuncImplSuite(t *testing.T) {
	suite.Run(t, new(oauth2ConfigFuncImplSuite))
}

func (*oauth2ConfigFuncImplSuite) SetupSuite() {
	defaultIssuerUrl := test.RandomString(20)
	os.Setenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable, defaultIssuerUrl)
}

func (*oauth2ConfigFuncImplSuite) TearDownSuite() {
	defer os.Unsetenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable)
}

// func (s *oauth2ConfigFuncImplSuite) Test_DefaultValues() {
// 	expected := s.expectedDefaultConfig()
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_ClientId() {
// 	expected := s.expectedDefaultConfig()
// 	expected.ClientID = "1234"
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{ClientID: "1234"})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_LoginURL() {
// 	expected := s.expectedDefaultConfig()
// 	expected.LoginURL = "login"
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{LoginURL: "login"})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_ProxyPrefix() {
// 	expected := s.expectedDefaultConfig()
// 	expected.ProxyPrefix = "prefix"
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{ProxyPrefix: "prefix"})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_RedeemURL() {
// 	expected := s.expectedDefaultConfig()
// 	expected.RedeemURL = "redeem"
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{RedeemURL: "redeem"})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_Scope() {
// 	expected := s.expectedDefaultConfig()
// 	expected.Scope = "offline"
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{Scope: "offline"})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_SessionStoreType() {
// 	expected := s.expectedDefaultConfig()
// 	expected.SessionStoreType = "any"
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{SessionStoreType: "any"})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_SetAuthorizationHeader() {
// 	expected := s.expectedDefaultConfig()
// 	expected.SetAuthorizationHeader = utils.BoolPtr(true)
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{SetAuthorizationHeader: utils.BoolPtr(true)})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_SetXAuthRequestHeaders() {
// 	expected := s.expectedDefaultConfig()
// 	expected.SetXAuthRequestHeaders = utils.BoolPtr(true)
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{SetXAuthRequestHeaders: utils.BoolPtr(true)})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_Cookie_Expire() {
// 	expected := s.expectedDefaultConfig()
// 	expected.Cookie.Expire = "expire"
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{Cookie: &v1.OAuth2Cookie{Expire: "expire"}})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_Cookie_Name() {
// 	expected := s.expectedDefaultConfig()
// 	expected.Cookie.Name = "cookiename"
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{Cookie: &v1.OAuth2Cookie{Name: "cookiename"}})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_Cookie_Refresh() {
// 	expected := s.expectedDefaultConfig()
// 	expected.Cookie.Refresh = "refresh"
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{Cookie: &v1.OAuth2Cookie{Refresh: "refresh"}})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_Cookie_SameSite() {
// 	expected := s.expectedDefaultConfig()
// 	expected.Cookie.SameSite = "samesite"
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{Cookie: &v1.OAuth2Cookie{SameSite: "samesite"}})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_CookieStore_Minimal() {
// 	expected := s.expectedDefaultConfig()
// 	expected.CookieStore = &v1.OAuth2CookieStore{Minimal: utils.BoolPtr(true)}
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{CookieStore: &v1.OAuth2CookieStore{Minimal: utils.BoolPtr(true)}})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_OIDC_InsecureSkipVerifyNonce() {
// 	expected := s.expectedDefaultConfig()
// 	expected.OIDC.InsecureSkipVerifyNonce = utils.BoolPtr(true)
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{OIDC: &v1.OAuth2OIDC{InsecureSkipVerifyNonce: utils.BoolPtr(true)}})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_OIDC_IssuerURL() {
// 	expected := s.expectedDefaultConfig()
// 	expected.OIDC.IssuerURL = "issuerurl"
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{OIDC: &v1.OAuth2OIDC{IssuerURL: "issuerurl"}})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_OIDC_JWKSURL() {
// 	expected := s.expectedDefaultConfig()
// 	expected.OIDC.JWKSURL = "jwksurl"
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{OIDC: &v1.OAuth2OIDC{JWKSURL: "jwksurl"}})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_OIDC_SkipDiscovery() {
// 	expected := s.expectedDefaultConfig()
// 	expected.OIDC.SkipDiscovery = utils.BoolPtr(true)
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{OIDC: &v1.OAuth2OIDC{SkipDiscovery: utils.BoolPtr(true)}})
// 	s.Equal(expected, actual)
// }

// func (s *oauth2ConfigFuncImplSuite) Test_RedisStore_ConnectionURL() {
// 	expected := s.expectedDefaultConfig()
// 	expected.RedisStore = &v1.OAuth2RedisStore{ConnectionURL: "redisurl"}
// 	actual := oauth2ConfigFuncImpl(&v1.OAuth2{RedisStore: &v1.OAuth2RedisStore{ConnectionURL: "redisurl"}})
// 	s.Equal(expected, actual)
// }

// func (*oauth2ConfigFuncImplSuite) expectedDefaultConfig() *v1.OAuth2 {
// 	return &v1.OAuth2{
// 		Scope:                  "openid profile email",
// 		SetXAuthRequestHeaders: utils.BoolPtr(false),
// 		SetAuthorizationHeader: utils.BoolPtr(false),
// 		ProxyPrefix:            "/oauth2",
// 		SessionStoreType:       v1.SessionStoreCookie,
// 		OIDC: &v1.OAuth2OIDC{
// 			IssuerURL:               os.Getenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable),
// 			InsecureSkipVerifyNonce: utils.BoolPtr(false),
// 		},
// 		Cookie: &v1.OAuth2Cookie{
// 			Name:     "_oauth2_proxy",
// 			Expire:   "168h0m0s",
// 			Refresh:  "60m0s",
// 			SameSite: v1.SameSiteLax,
// 		},
// 	}
// }
