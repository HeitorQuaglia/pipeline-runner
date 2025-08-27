package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"basic-container-runtime/pkg/secrets"
	"basic-container-runtime/pkg/workflow"
	"basic-container-runtime/runtime"
)

func main() {
	var workflowFile string
	var logLevel string
	var secretsFile string

	flag.StringVar(&workflowFile, "f", "", "Path to workflow YAML file")
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&secretsFile, "secrets-file", "", "Path to secrets file (.env format)")
	flag.Parse()

	if workflowFile == "" {
		fmt.Println("Usage: runner -f <workflow-file.yaml> [--log-level=<level>] [--secrets-file=<secrets.env>]")
		fmt.Println("Example: runner -f hello-world.yaml --log-level=debug --secrets-file=secrets.env")
		fmt.Println("Log levels: debug, info, warn, error")
		os.Exit(1)
	}

	if err := configureLogging(logLevel); err != nil {
		log.Fatalf("Failed to configure logging: %v", err)
	}

	if err := runWorkflow(workflowFile, secretsFile); err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	fmt.Println("Workflow completed successfully!")
}

func configureLogging(levelStr string) error {
	level, err := logrus.ParseLevel(strings.ToLower(levelStr))
	if err != nil {
		return fmt.Errorf("invalid log level '%s': %w", levelStr, err)
	}

	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	logrus.Debugf("Log level set to: %s", level.String())
	return nil
}

func runWorkflow(filename, secretsFile string) error {
	spec, err := workflow.ParseSimpleWorkflow(filename)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}

	wf := workflow.ConvertToWorkflow(spec, filename)

	// Initialize secret manager
	logger := logrus.StandardLogger()
	secretManager := secrets.NewSecretManager(logger)
	
	// Load secrets if file provided
	if secretsFile != "" {
		if err := secretManager.LoadSecretsFromFile(secretsFile); err != nil {
			return fmt.Errorf("failed to load secrets: %w", err)
		}
		
		// Validate required secrets
		if err := secretManager.ValidateRequiredSecrets(wf.Secrets); err != nil {
			return fmt.Errorf("secret validation failed: %w", err)
		}
	} else if len(wf.Secrets) > 0 {
		return fmt.Errorf("workflow requires secrets but no secrets file provided. Use --secrets-file flag")
	}

	engine := workflow.NewEngine()
	if err := engine.ValidateWorkflow(wf); err != nil {
		return fmt.Errorf("workflow validation failed: %w", err)
	}

	executor, err := runtime.NewContainerExecutor()
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}
	defer func() {
		executor.PrintCacheStats()
		executor.Close()
	}()

	// Set secret manager in executor
	executor.SetSecretManager(secretManager)

	ctx := context.Background()
	return engine.ExecuteWorkflow(ctx, wf, executor)
}
