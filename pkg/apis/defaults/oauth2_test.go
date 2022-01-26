package defaults

import (
	"testing"

	commonUtils "github.com/equinor/radix-common/utils"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/suite"
)

type oauth2DefaultConfigOptionsTestSuite struct {
	suite.Suite
}

func TestOAuth2ConfigFuncImplSuite(t *testing.T) {
	suite.Run(t, new(oauth2DefaultConfigOptionsTestSuite))
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_NewDefaultValues() {
	expected := oauth2Default()
	sut := NewOAuth2Config(WithOAuth2Defaults())
	actual := sut.(*oauth2Config)
	s.Equal(expected, actual.OAuth2)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_NewWithIssuerURL() {
	expected := v1.OAuth2{OIDC: &v1.OAuth2OIDC{IssuerURL: "anyissuerurl"}}
	sut := NewOAuth2Config(WithOIDCIssuerURL("anyissuerurl"))
	actual := sut.(*oauth2Config)
	s.Equal(expected, actual.OAuth2)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_NewWithMultplieOptions() {
	expected := oauth2Default()
	if expected.OIDC == nil {
		expected.OIDC = &v1.OAuth2OIDC{}
	}
	expected.OIDC.IssuerURL = "issuerurl"
	sut := NewOAuth2Config(WithOAuth2Defaults(), WithOIDCIssuerURL("issuerurl"))
	actual := sut.(*oauth2Config)
	s.Equal(expected, actual.OAuth2)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_ClientId() {
	expected := s.oauthConfig()
	expected.ClientID = "newclientid"
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{ClientID: "newclientid"})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_LoginURL() {
	expected := s.oauthConfig()
	expected.LoginURL = "newloginurl"
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{LoginURL: "newloginurl"})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_ProxyPrefix() {
	expected := s.oauthConfig()
	expected.ProxyPrefix = "newprefix"
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{ProxyPrefix: "newprefix"})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_RedeemURL() {
	expected := s.oauthConfig()
	expected.RedeemURL = "newredeemurl"
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{RedeemURL: "newredeemurl"})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_Scope() {
	expected := s.oauthConfig()
	expected.Scope = "newscope"
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{Scope: "newscope"})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_SessionStoreType() {
	expected := s.oauthConfig()
	expected.SessionStoreType = v1.SessionStoreType("newsessionstore")
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{SessionStoreType: v1.SessionStoreType("newsessionstore")})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_SetAuthorizationHeader() {
	expected := s.oauthConfig()
	expected.SetAuthorizationHeader = commonUtils.BoolPtr(true)
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{SetAuthorizationHeader: commonUtils.BoolPtr(true)})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_SetXAuthRequestHeaders() {
	expected := s.oauthConfig()
	expected.SetXAuthRequestHeaders = commonUtils.BoolPtr(true)
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{SetXAuthRequestHeaders: commonUtils.BoolPtr(true)})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_Cookie_Expire() {
	expected := s.oauthConfig()
	expected.Cookie.Expire = "newexpire"
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{Cookie: &v1.OAuth2Cookie{Expire: "newexpire"}})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_Cookie_Name() {
	expected := s.oauthConfig()
	expected.Cookie.Name = "newcookiename"
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{Cookie: &v1.OAuth2Cookie{Name: "newcookiename"}})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_Cookie_Refresh() {
	expected := s.oauthConfig()
	expected.Cookie.Refresh = "newrefresh"
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{Cookie: &v1.OAuth2Cookie{Refresh: "newrefresh"}})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_Cookie_SameSite() {
	expected := s.oauthConfig()
	expected.Cookie.SameSite = v1.CookieSameSiteType("newsamesite")
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{Cookie: &v1.OAuth2Cookie{SameSite: v1.CookieSameSiteType("newsamesite")}})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_CookieStore_Minimal() {
	expected := s.oauthConfig()
	expected.CookieStore.Minimal = commonUtils.BoolPtr(true)
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{CookieStore: &v1.OAuth2CookieStore{Minimal: commonUtils.BoolPtr(true)}})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_OIDC_InsecureSkipVerifyNonce() {
	expected := s.oauthConfig()
	expected.OIDC.InsecureSkipVerifyNonce = commonUtils.BoolPtr(true)
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{OIDC: &v1.OAuth2OIDC{InsecureSkipVerifyNonce: commonUtils.BoolPtr(true)}})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_OIDC_IssuerURL() {
	expected := s.oauthConfig()
	expected.OIDC.IssuerURL = "newissuerurl"
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{OIDC: &v1.OAuth2OIDC{IssuerURL: "newissuerurl"}})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_OIDC_JWKSURL() {
	expected := s.oauthConfig()
	expected.OIDC.JWKSURL = "newjwksurl"
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{OIDC: &v1.OAuth2OIDC{JWKSURL: "newjwksurl"}})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_OIDC_SkipDiscovery() {
	expected := s.oauthConfig()
	expected.OIDC.SkipDiscovery = commonUtils.BoolPtr(true)
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{OIDC: &v1.OAuth2OIDC{SkipDiscovery: commonUtils.BoolPtr(true)}})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_RedisStore_ConnectionURL() {
	expected := s.oauthConfig()
	expected.RedisStore = &v1.OAuth2RedisStore{ConnectionURL: "newconnectionurl"}
	sut := oauth2Config{OAuth2: s.oauthConfig()}
	actual, err := sut.MergeWith(&v1.OAuth2{RedisStore: &v1.OAuth2RedisStore{ConnectionURL: "newconnectionurl"}})
	s.Nil(err)
	s.Equal(expected, *actual)
}

func (*oauth2DefaultConfigOptionsTestSuite) oauthConfig() v1.OAuth2 {
	return v1.OAuth2{
		ClientID:               "expectedclientid",
		Scope:                  "expectedscope",
		SetXAuthRequestHeaders: commonUtils.BoolPtr(false),
		SetAuthorizationHeader: commonUtils.BoolPtr(false),
		ProxyPrefix:            "expectedprefix",
		LoginURL:               "expectedloginurl",
		RedeemURL:              "expectedredeemurl",
		SessionStoreType:       v1.SessionStoreType("expectedsessionstoretype"),
		OIDC: &v1.OAuth2OIDC{
			IssuerURL:               "expectedissuerurl",
			JWKSURL:                 "expectedjwksurl",
			SkipDiscovery:           commonUtils.BoolPtr(false),
			InsecureSkipVerifyNonce: commonUtils.BoolPtr(false),
		},
		Cookie: &v1.OAuth2Cookie{
			Name:     "expectedname",
			Expire:   "expectedexpire",
			Refresh:  "expectedrefresh",
			SameSite: v1.CookieSameSiteType("expectedsamesite"),
		},
		CookieStore: &v1.OAuth2CookieStore{
			Minimal: commonUtils.BoolPtr(false),
		},
		RedisStore: &v1.OAuth2RedisStore{
			ConnectionURL: "expectedconnectionurl",
		},
	}
}
