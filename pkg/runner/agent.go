package runner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	
	"basic-container-runtime/pkg/server"
	"basic-container-runtime/pkg/workflow"
	"basic-container-runtime/runtime"
)

// Agent represents a runner agent that connects to the server
type Agent struct {
	client          *Client
	executor        *runtime.Executor
	logger          *logrus.Logger
	heartbeatTicker *time.Ticker
	pollTicker      *time.Ticker
	running         bool
	currentJob      *server.Job
	mutex           sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewAgent creates a new runner agent
func NewAgent(serverURL, runnerID, runnerName string, logger *logrus.Logger) *Agent {
	client := NewClient(serverURL, runnerID, runnerName, logger)
	executor := runtime.NewExecutor(logger)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Agent{
		client:   client,
		executor: executor,
		logger:   logger,
		running:  false,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start starts the runner agent
func (a *Agent) Start() error {
	a.logger.Info("Starting runner agent...")
	
	// Register with server
	if err := a.client.Register(); err != nil {
		return fmt.Errorf("failed to register with server: %w", err)
	}
	
	a.mutex.Lock()
	a.running = true
	a.mutex.Unlock()
	
	// Start heartbeat timer (every 15 seconds)
	a.heartbeatTicker = time.NewTicker(15 * time.Second)
	go a.heartbeatLoop()
	
	// Start job polling timer (every 5 seconds)  
	a.pollTicker = time.NewTicker(5 * time.Second)
	go a.jobPollLoop()
	
	// Send initial heartbeat
	if err := a.client.SendHeartbeat(server.RunnerStatusOnline, ""); err != nil {
		a.logger.Warnf("Failed to send initial heartbeat: %v", err)
	}
	
	a.logger.Info("Runner agent started successfully")
	return nil
}

// Stop stops the runner agent
func (a *Agent) Stop() error {
	a.logger.Info("Stopping runner agent...")
	
	a.mutex.Lock()
	a.running = false
	a.mutex.Unlock()
	
	// Cancel context
	a.cancel()
	
	// Stop tickers
	if a.heartbeatTicker != nil {
		a.heartbeatTicker.Stop()
	}
	if a.pollTicker != nil {
		a.pollTicker.Stop()
	}
	
	// Unregister from server
	if err := a.client.Unregister(); err != nil {
		a.logger.Warnf("Failed to unregister from server: %v", err)
	}
	
	a.logger.Info("Runner agent stopped")
	return nil
}

// Wait blocks until the agent is stopped
func (a *Agent) Wait() {
	<-a.ctx.Done()
}

// heartbeatLoop sends periodic heartbeats to the server
func (a *Agent) heartbeatLoop() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.heartbeatTicker.C:
			a.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends a heartbeat with current status
func (a *Agent) sendHeartbeat() {
	a.mutex.RLock()
	isRunning := a.running
	currentJob := a.currentJob
	a.mutex.RUnlock()
	
	if !isRunning {
		return
	}
	
	status := server.RunnerStatusOnline
	currentJobID := ""
	
	if currentJob != nil {
		status = server.RunnerStatusBusy
		currentJobID = currentJob.ID
	}
	
	if err := a.client.SendHeartbeat(status, currentJobID); err != nil {
		a.logger.Warnf("Failed to send heartbeat: %v", err)
	}
}

// jobPollLoop polls for new jobs from the server
func (a *Agent) jobPollLoop() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.pollTicker.C:
			a.pollForJob()
		}
	}
}

// pollForJob checks for new jobs and executes them
func (a *Agent) pollForJob() {
	a.mutex.RLock()
	isRunning := a.running
	hasBusy := a.currentJob != nil
	a.mutex.RUnlock()
	
	if !isRunning || hasBusy {
		return // Not running or already busy
	}
	
	// Request next job from server
	assignment, err := a.client.GetNextJob()
	if err != nil {
		a.logger.Errorf("Failed to get next job: %v", err)
		return
	}
	
	if assignment == nil {
		// No jobs available
		return
	}
	
	a.logger.Infof("Received job assignment: %s (%s)", assignment.Job.ID, assignment.Job.Name)
	
	// Execute the job
	go a.executeJob(assignment.Job)
}

// executeJob executes a job and reports results
func (a *Agent) executeJob(job *server.Job) {
	a.mutex.Lock()
	a.currentJob = job
	a.mutex.Unlock()
	
	// Send heartbeat to update status
	a.sendHeartbeat()
	
	defer func() {
		a.mutex.Lock()
		a.currentJob = nil
		a.mutex.Unlock()
		// Send heartbeat to clear status
		a.sendHeartbeat()
	}()
	
	a.logger.Infof("Starting execution of job %s", job.ID)
	
	// Parse workflow from YAML
	workflowSpec, err := workflow.ParseSimpleWorkflowFromString(job.YAML)
	if err != nil {
		a.reportJobFailure(job.ID, fmt.Sprintf("Failed to parse workflow: %v", err))
		return
	}
	
	// Convert to internal workflow format
	wf := workflow.ConvertToWorkflow(workflowSpec, "")
	
	// Set variables and secrets from job
	if job.Variables != nil {
		if wf.Variables == nil {
			wf.Variables = make(map[string]string)
		}
		for k, v := range job.Variables {
			wf.Variables[k] = v
		}
	}
	
	if job.Secrets != nil {
		if wf.Secrets == nil {
			wf.Secrets = make([]string, 0)
		}
		for secretName := range job.Secrets {
			wf.Secrets = append(wf.Secrets, secretName)
		}
	}
	
	// Create execution context
	executionCtx := &runtime.ExecutionContext{
		WorkflowID: wf.ID,
		Variables:  wf.Variables,
		Secrets:    job.Secrets, // Pass actual secret values
		Logger:     a.logger,
	}
	
	// Create log collector for real-time submission
	logCollector := &AgentLogCollector{
		client: a.client,
		jobID:  job.ID,
		logger: a.logger,
	}
	
	// Execute workflow
	startTime := time.Now()
	result, err := a.executor.ExecuteWorkflowWithCollector(wf, executionCtx, logCollector)
	duration := time.Since(startTime)
	
	if err != nil {
		a.reportJobFailure(job.ID, fmt.Sprintf("Workflow execution failed: %v", err))
		return
	}
	
	// Report success
	jobResult := &server.JobResult{
		Success:   result.Success,
		Error:     result.ErrorMessage,
		ExitCode:  result.ExitCode,
		Duration:  duration,
		Workflow:  wf,
	}
	
	if err := a.client.CompleteJob(job.ID, jobResult); err != nil {
		a.logger.Errorf("Failed to report job completion: %v", err)
	} else {
		a.logger.Infof("Successfully completed job %s", job.ID)
	}
}

// reportJobFailure reports a job failure to the server
func (a *Agent) reportJobFailure(jobID, errorMsg string) {
	a.logger.Errorf("Job %s failed: %s", jobID, errorMsg)
	
	jobResult := &server.JobResult{
		Success:  false,
		Error:    errorMsg,
		ExitCode: 1,
		Duration: 0,
	}
	
	if err := a.client.CompleteJob(jobID, jobResult); err != nil {
		a.logger.Errorf("Failed to report job failure: %v", err)
	}
}

// AgentLogCollector collects logs and submits them to the server in real-time
type AgentLogCollector struct {
	client *Client
	jobID  string
	logger *logrus.Logger
	buffer []server.LogEntry
	mutex  sync.Mutex
}

// CollectLog adds a log entry to the buffer
func (c *AgentLogCollector) CollectLog(timestamp time.Time, level, message, source string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	logEntry := server.LogEntry{
		Timestamp: timestamp,
		Level:     level,
		Message:   message,
		Source:    source,
	}
	
	c.buffer = append(c.buffer, logEntry)
	
	// Submit logs when buffer reaches 10 entries or immediately for errors
	if len(c.buffer) >= 10 || strings.ToLower(level) == "error" {
		c.flushLogs()
	}
}

// flushLogs submits all buffered logs to the server
func (c *AgentLogCollector) flushLogs() {
	if len(c.buffer) == 0 {
		return
	}
	
	logs := make([]server.LogEntry, len(c.buffer))
	copy(logs, c.buffer)
	c.buffer = c.buffer[:0] // Clear buffer
	
	if err := c.client.SubmitLogs(c.jobID, logs); err != nil {
		c.logger.Warnf("Failed to submit logs: %v", err)
	}
}

// Flush ensures all logs are submitted
func (c *AgentLogCollector) Flush() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.flushLogs()
}