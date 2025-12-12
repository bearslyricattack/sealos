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
	"github.com/labring/sealos/service/minio/internal/handler"
	"github.com/labring/sealos/service/minio/internal/router"
	"github.com/labring/sealos/service/minio/internal/service"
	"github.com/labring/sealos/service/pkg/config"
)

// Run starts the minio monitoring service
func Run(cfg *config.ServerConfig) error {
	// Create minio service
	minioService, err := service.NewMinioService(cfg)
	if err != nil {
		return err
	}
	log.Printf("Minio monitoring service using metrics host: %s", minioService.GetMetricsHost())
	log.Printf("Minio instance: %s", minioService.GetMinioInstance())
	defer minioService.Close()

	// Create handler
	minioHandler := handler.NewMinioHandler(minioService)

	// Setup Gin engine
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Setup routes
	router.Setup(r, minioHandler)

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
