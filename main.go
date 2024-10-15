package main

import (
	"log"

	"github.com/dymensionxyz/eibc-client/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		log.Fatalf("failed to execute root command: %v", err)
	}
}
