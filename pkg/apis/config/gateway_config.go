package config

type GatewayConfig struct {
	Name        string `json:"name" required:"true"`
	Namespace   string `json:"namespace" required:"true"`
	SectionName string `json:"sectionName" required:"true"`
}
