package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	
	"basic-container-runtime/pkg/runner"
)

func main() {
	var (
		serverURL  = flag.String("server", "http://localhost:8080", "Server URL")
		runnerID   = flag.String("id", "", "Runner ID (auto-generated if not provided)")
		runnerName = flag.String("name", "", "Runner name")
		logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	)
	flag.Parse()

	// Setup logger
	logger := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid log level: %s\n", *logLevel)
		os.Exit(1)
	}
	logger.SetLevel(level)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Generate runner ID if not provided
	if *runnerID == "" {
		*runnerID = "runner-" + uuid.New().String()[:8]
	}
	
	// Use hostname as default name if not provided
	if *runnerName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			*runnerName = *runnerID
		} else {
			*runnerName = fmt.Sprintf("%s (%s)", hostname, *runnerID)
		}
	}

	logger.Infof("Starting Simple Workflow Runner Agent")
	logger.Infof("Runner ID: %s", *runnerID)
	logger.Infof("Runner Name: %s", *runnerName)
	logger.Infof("Server URL: %s", *serverURL)

	// Create and start agent
	agent := runner.NewAgent(*serverURL, *runnerID, *runnerName, logger)
	
	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		logger.Info("Shutting down runner agent...")
		if err := agent.Stop(); err != nil {
			logger.Errorf("Error stopping agent: %v", err)
		}
		os.Exit(0)
	}()

	// Start the agent
	if err := agent.Start(); err != nil {
		logger.Fatalf("Failed to start runner agent: %v", err)
	}
	
	// Wait for shutdown
	agent.Wait()
	logger.Info("Runner agent stopped")
}