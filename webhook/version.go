package main

import (
	"runtime/debug"
)

var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}

	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return
	}

	Version = info.Main.Version
}
