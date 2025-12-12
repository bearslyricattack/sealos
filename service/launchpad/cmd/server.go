package cmd

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/sealos/service/launchpad/internal/handler"
	"github.com/labring/sealos/service/launchpad/internal/router"
	"github.com/labring/sealos/service/launchpad/internal/service"
	"github.com/labring/sealos/service/pkg/config"
)

// Run starts the launchpad monitoring service
func Run(cfg *config.ServerConfig) error {
	// Create launchpad service
	launchpadService, err := service.NewLaunchpadService(cfg)
	if err != nil {
		return err
	}
	log.Printf("Launchpad monitoring service using metrics host: %s", launchpadService.GetMetricsHost())
	defer launchpadService.Close()

	// Create handler
	launchpadHandler := handler.NewLaunchpadHandler(launchpadService)

	// Setup Gin engine
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Setup routes
	router.Setup(r, launchpadHandler)

	// Create HTTP server
	server := &http.Server{
		Addr:           cfg.ListenAddress,
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 21,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	}()
	// Wait for interrupt signal and graceful shutdown
	return gracefulShutdown(server)
}

// gracefulShutdown handles graceful shutdown
func gracefulShutdown(server *http.Server) error {
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
