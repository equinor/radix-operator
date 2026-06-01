package config

type GatewayConfig struct {
	Name        string `envconfig:"RADIXOPERATOR_INGRESS_GATEWAY_NAME" required:"true"`
	Namespace   string `envconfig:"RADIXOPERATOR_INGRESS_GATEWAY_NAMESPACE" required:"true"`
	SectionName string `envconfig:"RADIXOPERATOR_INGRESS_GATEWAY_SECTION_NAME" required:"true"`
}
