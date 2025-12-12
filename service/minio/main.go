// Package main is the entry point for the Minio monitoring service.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/labring/sealos/service/minio/cmd"
	"github.com/labring/sealos/service/pkg/config"
)

func main() {
	// Setup logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Parse command-line flags
	configFile := flag.String("config", "/config/config.yml", "path to configuration file")
	flag.Parse()

	// Override with positional argument if provided
	if flag.NArg() > 0 {
		*configFile = flag.Arg(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Run server
	if err := cmd.Run(cfg); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
