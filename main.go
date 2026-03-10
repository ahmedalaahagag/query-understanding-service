package main

import (
	"os"

	"github.com/hellofresh/qus/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
