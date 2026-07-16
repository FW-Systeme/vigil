package main

import (
	"os"

	"github.com/FW-Systeme/Virgil/internal/cli"
)

var version = "dev"

func main() {
	cli.SetVersion(version)

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
