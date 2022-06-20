package alert

const (
	radixApplicationNameLabel = "label_radix_app"
	radixEnvironmentNameLabel = "label_radix_env"
	radixComponentNameLabel   = "label_radix_component"
	radixReplicaNameLabel     = "radix_replica_name"
	radixJobNameLabel         = "job_name"
	radixPipelineJobNameLabel = "label_radix_job_name"
)

// AlertScope defines scope for an alert
type AlertScope int

const (
	// ApplicationScope contains alerts for failed pipeline jobs
	ApplicationScope AlertScope = iota
	// EnvironmentScope contains alerts for components and jobs in a specific environment
	EnvironmentScope
)

var (
	defaultSlackMessageTemplate slackMessageTemplate = slackMessageTemplate{
		title:     "{{ template \"radix-slack-alert-title\" .}}",
		titleLink: "{{ template \"radix-slack-alert-titlelink\" .}}",
		text:      "{{ template \"radix-slack-alert-text\" .}}",
	}
	defaultAlertConfigs AlertConfigs = AlertConfigs{
		"RadixAppComponentCrashLooping": {
			GroupBy:    []string{radixApplicationNameLabel, radixEnvironmentNameLabel, radixComponentNameLabel, radixReplicaNameLabel},
			Resolvable: true,
			Scope:      EnvironmentScope,
		},
		"RadixAppComponentNotReady": {
			GroupBy:    []string{radixApplicationNameLabel, radixEnvironmentNameLabel, radixComponentNameLabel},
			Resolvable: true,
			Scope:      EnvironmentScope,
		},
		"RadixAppJobNotReady": {
			GroupBy:    []string{radixApplicationNameLabel, radixEnvironmentNameLabel, radixJobNameLabel},
			Resolvable: true,
			Scope:      EnvironmentScope,
		},
		"RadixAppJobFailed": {
			GroupBy:    []string{radixApplicationNameLabel, radixEnvironmentNameLabel, radixJobNameLabel},
			Resolvable: false,
			Scope:      EnvironmentScope,
		},
		"RadixAppPipelineJobFailed": {
			GroupBy:    []string{radixApplicationNameLabel, radixPipelineJobNameLabel},
			Resolvable: false,
			Scope:      ApplicationScope,
		},
	}
)

type AlertConfig struct {
	GroupBy    []string
	Resolvable bool
	Scope      AlertScope
}

type AlertConfigs map[string]AlertConfig

type slackMessageTemplate struct {
	title     string
	titleLink string
	text      string
}

// GetDefaultAlertConfigs returns list of alert names defined for scope
func GetDefaultAlertConfigs() AlertConfigs {
	return defaultAlertConfigs
}
