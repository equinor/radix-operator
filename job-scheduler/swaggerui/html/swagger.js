window.onload = () => {
	window.ui = SwaggerUIBundle({
		url: "./swagger.json",
		dom_id: "#swagger-ui",
		deepLinking: true,
		presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
		layout: "StandaloneLayout",
	});

	console.info("Swagger UI loaded");
};
