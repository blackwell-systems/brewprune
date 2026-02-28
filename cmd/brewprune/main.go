package main

import (
	"os"

	"github.com/blackwell-systems/brewprune/internal/app"
)

func main() {
	if err := app.Execute(); err != nil {
		// Error already printed by Execute()
		os.Exit(1)
	}
}
