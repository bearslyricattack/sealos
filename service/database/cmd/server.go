package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labring/sealos/service/database/internal/router"
	"github.com/labring/sealos/service/database/internal/service"
	"github.com/labring/sealos/service/pkg/config"
	pkgHandler "github.com/labring/sealos/service/pkg/handler"
)

// Run starts the database monitoring service
func Run(cfg *config.ServerConfig) error {
	// Create server
	server, err := pkgHandler.NewServer(cfg)
	if err != nil {
		return err
	}

	// Create database service
	dbService, err := service.NewDatabaseService(cfg)
	if err != nil {
		return err
	}
	defer dbService.Close()

	log.Printf("Database monitoring service using metrics host: %s", dbService.GetMetricsHost())

	// Setup routes
	router.SetupRoutes(server, dbService)

	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	return gracefulShutdown(server)
}

// gracefulShutdown handles graceful shutdown
func gracefulShutdown(server *pkgHandler.Server) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
		return err
	}

	log.Println("Server exited")
	return nil
}
