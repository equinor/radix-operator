// file deepcode ignore HardcodedPassword: does not contain a secret value
package suffix

const (
	//ClientCertificate Client certificate
	ClientCertificate = "-clientcertca"
	//OAuth2ClientSecret Client secret of OAuth2
	OAuth2ClientSecret = "-oauth2proxy-clientsecret"
	//OAuth2CookieSecret Cookie secret of OAuth2
	OAuth2CookieSecret = "-oauth2proxy-cookiesecret"
	//OAuth2RedisPassword Password of OAuth2 Redis
	OAuth2RedisPassword = "-oauth2proxy-redispassword"
)
