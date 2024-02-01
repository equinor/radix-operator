package radixvalidators

const (
	maxPortNameLength    = 15
	minimumPortNumber    = 1024
	maximumPortNumber    = 65535
	cpuRegex             = "^[0-9]+m$"
	resourceNameTemplate = `^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$`
)
