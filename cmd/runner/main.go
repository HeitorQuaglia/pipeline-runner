package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"basic-container-runtime/pkg/workflow"
	"basic-container-runtime/runtime"
)

func main() {
	var workflowFile string
	var logLevel string

	flag.StringVar(&workflowFile, "f", "", "Path to workflow YAML file")
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	if workflowFile == "" {
		fmt.Println("Usage: runner -f <workflow-file.yaml> [--log-level=<level>]")
		fmt.Println("Example: runner -f hello-world.yaml --log-level=debug")
		fmt.Println("Log levels: debug, info, warn, error")
		os.Exit(1)
	}

	if err := configureLogging(logLevel); err != nil {
		log.Fatalf("Failed to configure logging: %v", err)
	}

	if err := runWorkflow(workflowFile); err != nil {
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

func runWorkflow(filename string) error {
	spec, err := workflow.ParseSimpleWorkflow(filename)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}

	wf := workflow.ConvertToWorkflow(spec)

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

	ctx := context.Background()
	return engine.ExecuteWorkflow(ctx, wf, executor)
}
