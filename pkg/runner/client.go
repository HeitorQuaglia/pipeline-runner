package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	
	"basic-container-runtime/pkg/server"
)

// Client represents a runner client that communicates with the server
type Client struct {
	serverURL    string
	runnerID     string
	runnerName   string
	httpClient   *http.Client
	logger       *logrus.Logger
	capabilities []string
	tags         []string
	metadata     map[string]string
}

// NewClient creates a new runner client
func NewClient(serverURL, runnerID, runnerName string, logger *logrus.Logger) *Client {
	return &Client{
		serverURL:  serverURL,
		runnerID:   runnerID,
		runnerName: runnerName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:       logger,
		capabilities: []string{"docker", "basic-workflow"},
		tags:         []string{},
		metadata:     make(map[string]string),
	}
}

// Register registers this runner with the server
func (c *Client) Register() error {
	registration := server.RunnerRegistration{
		ID:           c.runnerID,
		Name:         c.runnerName,
		Version:      "1.0.0",
		Capabilities: c.capabilities,
		Tags:         c.tags,
		Metadata:     c.metadata,
	}
	
	data, err := json.Marshal(registration)
	if err != nil {
		return fmt.Errorf("failed to marshal registration: %w", err)
	}
	
	resp, err := c.httpClient.Post(
		c.serverURL+"/api/runners/register",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return fmt.Errorf("failed to register runner: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registration failed with status: %d", resp.StatusCode)
	}
	
	c.logger.Infof("Successfully registered runner %s (%s) with server", c.runnerID, c.runnerName)
	return nil
}

// SendHeartbeat sends a heartbeat to the server
func (c *Client) SendHeartbeat(status server.RunnerStatus, currentJobID string) error {
	heartbeat := server.HeartbeatRequest{
		RunnerID:     c.runnerID,
		Status:       status,
		CurrentJobID: currentJobID,
	}
	
	data, err := json.Marshal(heartbeat)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %w", err)
	}
	
	resp, err := c.httpClient.Post(
		c.serverURL+"/api/runners/"+c.runnerID+"/heartbeat",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed with status: %d", resp.StatusCode)
	}
	
	return nil
}

// GetNextJob requests the next job from the server
func (c *Client) GetNextJob() (*server.JobAssignment, error) {
	request := map[string]string{
		"runner_id": c.runnerID,
	}
	
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job request: %w", err)
	}
	
	resp, err := c.httpClient.Post(
		c.serverURL+"/api/jobs/next",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to request next job: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // No jobs available
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("job request failed with status: %d", resp.StatusCode)
	}
	
	var apiResp server.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	if !apiResp.Success {
		return nil, fmt.Errorf("job request failed: %s", apiResp.Error)
	}
	
	// Convert the data back to JobAssignment
	assignmentData, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal assignment data: %w", err)
	}
	
	var assignment server.JobAssignment
	if err := json.Unmarshal(assignmentData, &assignment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job assignment: %w", err)
	}
	
	return &assignment, nil
}

// CompleteJob reports job completion to the server
func (c *Client) CompleteJob(jobID string, result *server.JobResult) error {
	request := server.JobCompletionRequest{
		JobID:    jobID,
		RunnerID: c.runnerID,
		Result:   result,
	}
	
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal completion request: %w", err)
	}
	
	resp, err := c.httpClient.Post(
		c.serverURL+"/api/jobs/"+jobID+"/complete",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("job completion failed with status: %d", resp.StatusCode)
	}
	
	return nil
}

// SubmitLogs sends log entries to the server
func (c *Client) SubmitLogs(jobID string, logs []server.LogEntry) error {
	submission := server.LogSubmission{
		JobID:    jobID,
		RunnerID: c.runnerID,
		Logs:     logs,
	}
	
	data, err := json.Marshal(submission)
	if err != nil {
		return fmt.Errorf("failed to marshal log submission: %w", err)
	}
	
	resp, err := c.httpClient.Post(
		c.serverURL+"/api/jobs/"+jobID+"/logs",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return fmt.Errorf("failed to submit logs: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("log submission failed with status: %d", resp.StatusCode)
	}
	
	return nil
}

// Unregister removes this runner from the server
func (c *Client) Unregister() error {
	req, err := http.NewRequest("DELETE", c.serverURL+"/api/runners/"+c.runnerID, nil)
	if err != nil {
		return fmt.Errorf("failed to create unregister request: %w", err)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to unregister runner: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unregister failed with status: %d", resp.StatusCode)
	}
	
	c.logger.Infof("Successfully unregistered runner %s", c.runnerID)
	return nil
}