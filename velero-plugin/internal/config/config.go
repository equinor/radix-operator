package config

import (
	"flag"
)

type Config struct {
	LogLevel       string
	LogPrettyPrint bool
}

func ParseConfig() Config {
	var c Config
	flag.StringVar(&c.LogLevel, "log-level", "info", "Log level for the plugin. Possible values: trace, debug, info, warning, error, fatal, panic")
	flag.BoolVar(&c.LogPrettyPrint, "log-pretty", false, "Enable pretty print for logs. If set to true, the logs will be formatted in a human-readable way.")
	flag.Parse()
	return c
}
