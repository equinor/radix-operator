package defaults

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/suite"
)

type oauth2DefaultConfigOptionsTestSuite struct {
	suite.Suite
}

func TestOAuth2ConfigFuncImplSuite(t *testing.T) {
	suite.Run(t, new(oauth2DefaultConfigOptionsTestSuite))
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_MergeWithDoesNotMutateOriginal() {
	original := s.oauthConfig()
	originalClientId := original.ClientID
	actual, err := MergeOAuth2(original, v1.OAuth2{ClientID: "newclientid"})
	s.Nil(err)
	expected := s.oauthConfig()
	expected.ClientID = "newclientid"
	s.Equal(expected, actual)
	s.Equal(originalClientId, original.ClientID)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_ClientId() {
	expected := s.oauthConfig()
	expected.ClientID = "newclientid"
	actual, err := MergeOAuth2(expected, v1.OAuth2{ClientID: "newclientid"})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_LoginURL() {
	expected := s.oauthConfig()
	expected.LoginURL = "newloginurl"
	actual, err := MergeOAuth2(expected, v1.OAuth2{LoginURL: "newloginurl"})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_ProxyPrefix() {
	expected := s.oauthConfig()
	expected.ProxyPrefix = "newprefix"
	actual, err := MergeOAuth2(expected, v1.OAuth2{ProxyPrefix: "newprefix"})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_RedeemURL() {
	expected := s.oauthConfig()
	expected.RedeemURL = "newredeemurl"
	actual, err := MergeOAuth2(expected, v1.OAuth2{RedeemURL: "newredeemurl"})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_Scope() {
	expected := s.oauthConfig()
	expected.Scope = "newscope"
	actual, err := MergeOAuth2(expected, v1.OAuth2{Scope: "newscope"})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_SessionStoreType() {
	expected := s.oauthConfig()
	expected.SessionStoreType = v1.SessionStoreType("newsessionstore")
	actual, err := MergeOAuth2(expected, v1.OAuth2{SessionStoreType: v1.SessionStoreType("newsessionstore")})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_SetAuthorizationHeader() {
	expected := s.oauthConfig()
	expected.SetAuthorizationHeader = new(true)
	actual, err := MergeOAuth2(expected, v1.OAuth2{SetAuthorizationHeader: new(true)})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_SetXAuthRequestHeaders() {
	expected := s.oauthConfig()
	expected.SetXAuthRequestHeaders = new(true)
	actual, err := MergeOAuth2(expected, v1.OAuth2{SetXAuthRequestHeaders: new(true)})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_Cookie_Expire() {
	expected := s.oauthConfig()
	expected.Cookie.Expire = "newexpire"
	actual, err := MergeOAuth2(expected, v1.OAuth2{Cookie: &v1.OAuth2Cookie{Expire: "newexpire"}})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_Cookie_Name() {
	expected := s.oauthConfig()
	expected.Cookie.Name = "newcookiename"
	actual, err := MergeOAuth2(expected, v1.OAuth2{Cookie: &v1.OAuth2Cookie{Name: "newcookiename"}})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_Cookie_Refresh() {
	expected := s.oauthConfig()
	expected.Cookie.Refresh = "newrefresh"
	actual, err := MergeOAuth2(expected, v1.OAuth2{Cookie: &v1.OAuth2Cookie{Refresh: "newrefresh"}})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_Cookie_SameSite() {
	expected := s.oauthConfig()
	expected.Cookie.SameSite = v1.CookieSameSiteType("newsamesite")
	actual, err := MergeOAuth2(expected, v1.OAuth2{Cookie: &v1.OAuth2Cookie{SameSite: v1.CookieSameSiteType("newsamesite")}})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_CookieStore_Minimal() {
	expected := s.oauthConfig()
	expected.CookieStore.Minimal = new(true)
	actual, err := MergeOAuth2(expected, v1.OAuth2{CookieStore: &v1.OAuth2CookieStore{Minimal: new(true)}})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_OIDC_InsecureSkipVerifyNonce() {
	expected := s.oauthConfig()
	expected.OIDC.InsecureSkipVerifyNonce = new(true)
	actual, err := MergeOAuth2(expected, v1.OAuth2{OIDC: &v1.OAuth2OIDC{InsecureSkipVerifyNonce: new(true)}})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_OIDC_IssuerURL() {
	expected := s.oauthConfig()
	expected.OIDC.IssuerURL = "newissuerurl"
	actual, err := MergeOAuth2(expected, v1.OAuth2{OIDC: &v1.OAuth2OIDC{IssuerURL: "newissuerurl"}})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_OIDC_JWKSURL() {
	expected := s.oauthConfig()
	expected.OIDC.JWKSURL = "newjwksurl"
	actual, err := MergeOAuth2(expected, v1.OAuth2{OIDC: &v1.OAuth2OIDC{JWKSURL: "newjwksurl"}})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_OIDC_SkipDiscovery() {
	expected := s.oauthConfig()
	expected.OIDC.SkipDiscovery = new(true)
	actual, err := MergeOAuth2(expected, v1.OAuth2{OIDC: &v1.OAuth2OIDC{SkipDiscovery: new(true)}})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *oauth2DefaultConfigOptionsTestSuite) Test_RedisStore_ConnectionURL() {
	expected := s.oauthConfig()
	expected.RedisStore = &v1.OAuth2RedisStore{ConnectionURL: "newconnectionurl"}
	actual, err := MergeOAuth2(expected, v1.OAuth2{RedisStore: &v1.OAuth2RedisStore{ConnectionURL: "newconnectionurl"}})
	s.Nil(err)
	s.Equal(expected, actual)
}

func (*oauth2DefaultConfigOptionsTestSuite) oauthConfig() v1.OAuth2 {
	return v1.OAuth2{
		ClientID:               "expectedclientid",
		Scope:                  "expectedscope",
		SetXAuthRequestHeaders: new(false),
		SetAuthorizationHeader: new(false),
		ProxyPrefix:            "expectedprefix",
		LoginURL:               "expectedloginurl",
		RedeemURL:              "expectedredeemurl",
		SessionStoreType:       v1.SessionStoreType("expectedsessionstoretype"),
		OIDC: &v1.OAuth2OIDC{
			IssuerURL:               "expectedissuerurl",
			JWKSURL:                 "expectedjwksurl",
			SkipDiscovery:           new(false),
			InsecureSkipVerifyNonce: new(false),
		},
		Cookie: &v1.OAuth2Cookie{
			Name:     "expectedname",
			Expire:   "expectedexpire",
			Refresh:  "expectedrefresh",
			SameSite: v1.CookieSameSiteType("expectedsamesite"),
		},
		CookieStore: &v1.OAuth2CookieStore{
			Minimal: new(false),
		},
		RedisStore: &v1.OAuth2RedisStore{
			ConnectionURL: "expectedconnectionurl",
		},
	}
}
