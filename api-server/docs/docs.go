// Package docs classification Radix API.
//
// This is the API Server for the Radix platform.
//
//	Schemes: https, http
//	BasePath: /api/v1
//	Version: 1.23.0
//	Contact: https://equinor.slack.com/messages/CBKM6N2JY
//
//	Consumes:
//	- application/json
//
//	Produces:
//	- application/json
//
//	Security:
//	- bearer:
//	- token:
//
//	SecurityDefinitions:
//		bearer:
//		    type: oauth2
//		    flow: accessCode
//		    scopes:
//		      openid: OpenID
//		      profile: User Profile
//		      6dae42f8-4368-4678-94ff-3960e28e3630/.default: Access Radix API
//		    authorizationUrl: https://login.microsoftonline.com/3aa4a235-b6e2-48d5-9195-7fcf05b459b0/oauth2/v2.0/authorize
//		    tokenUrl: https://login.microsoftonline.com/3aa4a235-b6e2-48d5-9195-7fcf05b459b0/oauth2/v2.0/token
//		token:
//	        type: apiKey
//	        name: Authorization
//	        in: header
//
// swagger:meta
package docs
