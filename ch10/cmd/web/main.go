package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aws-mcp-server/internal/config"
	"aws-mcp-server/internal/logging"
	"aws-mcp-server/pkg/aws"
	"aws-mcp-server/pkg/web"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logging using config
	logger := logging.NewLogger(cfg.Logging.Level, cfg.Logging.Format)

	logger.Info("Starting AI Infrastructure Agent Web UI")

	// Initialize AWS client
	awsClient, err := aws.NewClient(cfg.AWS.Region, cfg.AWS.Profile, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create AWS client")
	}

	// Create web server with shared infrastructure
	webServer := web.NewWebServer(cfg, awsClient, logger)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received shutdown signal, stopping server...")
		cancel()
	}()

	// Start web server
	webPort := cfg.GetWebPort()
	logger.WithField("port", webPort).Info("Starting web server")
	fmt.Printf("\n🚀 AI Infrastructure Agent Web UI\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("🌐 Open your browser and go to: http://localhost:%d\n", webPort)
	fmt.Printf("📊 Dashboard: http://localhost:%d/dashboard\n", webPort)
	fmt.Printf("🔗 API Base: http://localhost:%d/api\n", webPort)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("💡 Features:\n")
	fmt.Printf("   • Natural language infrastructure requests\n")
	fmt.Printf("   • Real-time infrastructure state monitoring\n")
	fmt.Printf("   • Dependency graph visualization\n")
	fmt.Printf("   • Conflict detection and resolution\n")
	fmt.Printf("   • Deployment planning with dependency ordering\n")
	fmt.Printf("   • State export/import capabilities\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("🔑 Keyboard Shortcuts:\n")
	fmt.Printf("   • Ctrl+Enter: Process AI agent request\n")
	fmt.Printf("   • F5: Refresh current tab\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := webServer.Start(webPort); err != nil {
			serverErr <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		logger.Info("Shutting down gracefully...")
		// Give time for cleanup
		time.Sleep(2 * time.Second)
	case err := <-serverErr:
		logger.WithError(err).Error("Web server failed")
	}

	logger.Info("AI Infrastructure Agent Web UI stopped")
}
