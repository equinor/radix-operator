window.onload = () => {
	window.ui = SwaggerUIBundle({
		url: "./swagger.json",
		dom_id: "#swagger-ui",
		deepLinking: true,
		presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
		layout: "StandaloneLayout",
	});

	window.ui.initOAuth({
		clientId: "6e96429a-3ad5-40ee-b961-6de864d878fc",
		realm: "3aa4a235-b6e2-48d5-9195-7fcf05b459b0",
		appName: "radix-ar-swaggerui",
		scopeSeparator: " ",
		scopes: "openid profile 6dae42f8-4368-4678-94ff-3960e28e3630/.default",
		usePkceWithAuthorizationCodeGrant: true,
	});

	console.info("Swagger UI loaded");
};
