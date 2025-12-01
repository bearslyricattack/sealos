// Package main is the entry point for the Launchpad monitoring service.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/sealos/service/launchpad/handler"
	"github.com/labring/sealos/service/pkg/config"
	pkgHandler "github.com/labring/sealos/service/pkg/handler"
)

func main() {
	// Setup logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Parse command-line flags
	configFile := flag.String("config", "/config/config.yml", "path to configuration file")
	flag.Parse()

	// Override with positional argument if provided (backward compatibility)
	if flag.NArg() > 0 {
		*configFile = flag.Arg(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create server
	server, err := pkgHandler.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Create Launchpad handler
	launchpadHandler, err := handler.NewLaunchpadHandler(cfg)
	if err != nil {
		log.Fatalf("Failed to create launchpad handler: %v", err)
	}
	defer launchpadHandler.Close()

	log.Printf("Launchpad monitoring service using metrics host: %s", launchpadHandler.GetMetricsHost())

	// Register routes
	// POST /query - Main query endpoint (maintains API compatibility)
	server.RegisterQueryHandler("/query", launchpadHandler.HandleQuery)

	// Health check endpoint
	server.Router().GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "launchpad",
		})
	})

	// Readiness check endpoint
	server.Router().GET("/readyz", func(c *gin.Context) {
		c.String(200, "ok")
	})

	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with 5-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
