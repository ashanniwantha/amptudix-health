package main

import (
	"fmt"
	"os"

	"github.com/ashanniwantha/amptudix-health/internal/config"
	"github.com/ashanniwantha/amptudix-health/internal/logger"
)

func main() {
	config, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
	}

	logger := logger.NewLogger(config.AppEnv)
	logger.Info("starting application", "env", config.AppEnv, "port", config.AppPort)
}
