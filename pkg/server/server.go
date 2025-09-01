package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	
	"basic-container-runtime/pkg/workflow"
)

// Server represents the workflow server
type Server struct {
	queue    *JobQueue
	registry *RunnerRegistry
	logger   *logrus.Logger
	port     int
	version  string
}

// NewServer creates a new workflow server
func NewServer(port int, logger *logrus.Logger) *Server {
	return &Server{
		queue:    NewJobQueue(logger),
		registry: NewRunnerRegistry(logger),
		logger:   logger,
		port:     port,
		version:  "1.0.0",
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	router := s.setupRoutes()
	
	s.logger.Infof("Starting workflow server on port %d", s.port)
	return http.ListenAndServe(fmt.Sprintf(":%d", s.port), router)
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() *mux.Router {
	router := mux.NewRouter()
	
	// Enable CORS for all routes
	router.Use(s.corsMiddleware)
	router.Use(s.loggingMiddleware)
	
	// API routes
	api := router.PathPrefix("/api").Subrouter()
	
	// Workflow management
	api.HandleFunc("/workflows", s.submitWorkflow).Methods("POST")
	api.HandleFunc("/workflows/{id}/cancel", s.cancelWorkflow).Methods("POST")
	
	// Job management
	api.HandleFunc("/jobs", s.listJobs).Methods("GET")
	api.HandleFunc("/jobs/{id}", s.getJob).Methods("GET")
	api.HandleFunc("/jobs/{id}/logs", s.getJobLogs).Methods("GET")
	api.HandleFunc("/jobs/next", s.getNextJob).Methods("POST") // For runners
	api.HandleFunc("/jobs/{id}/complete", s.completeJob).Methods("POST") // For runners
	api.HandleFunc("/jobs/{id}/logs", s.submitLogs).Methods("POST") // For runners
	
	// Runner management
	api.HandleFunc("/runners/register", s.registerRunner).Methods("POST")
	api.HandleFunc("/runners/{id}/heartbeat", s.updateHeartbeat).Methods("POST")
	api.HandleFunc("/runners", s.listRunners).Methods("GET")
	api.HandleFunc("/runners/{id}", s.getRunner).Methods("GET")
	api.HandleFunc("/runners/{id}/unregister", s.unregisterRunner).Methods("DELETE")
	
	// Server status and stats
	api.HandleFunc("/status", s.getServerStatus).Methods("GET")
	api.HandleFunc("/stats", s.getStats).Methods("GET")
	
	// Health check
	api.HandleFunc("/health", s.healthCheck).Methods("GET")
	
	return router
}

// Middleware for CORS
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// Middleware for request logging
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Call the next handler
		next.ServeHTTP(w, r)
		
		// Log the request
		s.logger.Infof("%s %s - %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// submitWorkflow handles POST /api/workflows
func (s *Server) submitWorkflow(w http.ResponseWriter, r *http.Request) {
	var submission WorkflowSubmission
	if err := json.NewDecoder(r.Body).Decode(&submission); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	
	// Validate workflow YAML
	_, err := workflow.ParseSimpleWorkflowFromString(submission.YAML)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid workflow YAML: "+err.Error())
		return
	}
	
	// Add job to queue
	job, err := s.queue.AddJob(&submission)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to add job: "+err.Error())
		return
	}
	
	s.sendSuccess(w, job)
}

// getNextJob handles POST /api/jobs/next (for runners)
func (s *Server) getNextJob(w http.ResponseWriter, r *http.Request) {
	var request struct {
		RunnerID string `json:"runner_id"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	
	// Verify runner exists
	runner, err := s.registry.GetRunner(request.RunnerID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "Runner not found: "+err.Error())
		return
	}
	
	// Get next job
	job, err := s.queue.GetNextJob(request.RunnerID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, err.Error())
		return
	}
	
	// Create job assignment
	assignment := JobAssignment{
		Job:       job,
		ServerURL: fmt.Sprintf("http://localhost:%d", s.port), // TODO: Make this configurable
	}
	
	// Update runner status to busy
	err = s.registry.UpdateHeartbeat(request.RunnerID, RunnerStatusBusy, job.ID)
	if err != nil {
		s.logger.Warnf("Failed to update runner status: %v", err)
	}
	
	s.logger.Infof("Assigned job %s to runner %s (%s)", job.ID, runner.ID, runner.Name)
	s.sendSuccess(w, assignment)
}

// completeJob handles POST /api/jobs/{id}/complete (for runners)
func (s *Server) completeJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]
	
	var request JobCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	
	// Complete the job
	err := s.queue.CompleteJob(jobID, request.Result)
	if err != nil {
		s.sendError(w, http.StatusNotFound, err.Error())
		return
	}
	
	// Update runner status back to online
	err = s.registry.UpdateHeartbeat(request.RunnerID, RunnerStatusOnline, "")
	if err != nil {
		s.logger.Warnf("Failed to update runner status: %v", err)
	}
	
	s.sendSuccess(w, map[string]string{"status": "completed"})
}

// listJobs handles GET /api/jobs
func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	page, pageSize := s.getPaginationParams(r)
	
	jobs, total, err := s.queue.ListJobs(page, pageSize)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	response := JobListResponse{
		Jobs: jobs,
		Pagination: &PaginationParams{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}
	
	s.sendSuccess(w, response)
}

// getJob handles GET /api/jobs/{id}
func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]
	
	job, err := s.queue.GetJob(jobID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, err.Error())
		return
	}
	
	s.sendSuccess(w, job)
}

// getJobLogs handles GET /api/jobs/{id}/logs
func (s *Server) getJobLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]
	
	job, err := s.queue.GetJob(jobID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, err.Error())
		return
	}
	
	s.sendSuccess(w, map[string]interface{}{
		"job_id": jobID,
		"logs":   job.Logs,
	})
}

// submitLogs handles POST /api/jobs/{id}/logs (for runners)
func (s *Server) submitLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]
	
	var submission LogSubmission
	if err := json.NewDecoder(r.Body).Decode(&submission); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	
	err := s.queue.AddLogs(jobID, submission.Logs)
	if err != nil {
		s.sendError(w, http.StatusNotFound, err.Error())
		return
	}
	
	s.sendSuccess(w, map[string]string{"status": "logs_added"})
}

// registerRunner handles POST /api/runners/register
func (s *Server) registerRunner(w http.ResponseWriter, r *http.Request) {
	var registration RunnerRegistration
	if err := json.NewDecoder(r.Body).Decode(&registration); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	
	runner, err := s.registry.RegisterRunner(&registration)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	s.sendSuccess(w, runner)
}

// updateHeartbeat handles POST /api/runners/{id}/heartbeat
func (s *Server) updateHeartbeat(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	runnerID := vars["id"]
	
	var heartbeat HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&heartbeat); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	
	err := s.registry.UpdateHeartbeat(runnerID, heartbeat.Status, heartbeat.CurrentJobID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, err.Error())
		return
	}
	
	s.sendSuccess(w, map[string]string{"status": "heartbeat_updated"})
}

// listRunners handles GET /api/runners
func (s *Server) listRunners(w http.ResponseWriter, r *http.Request) {
	page, pageSize := s.getPaginationParams(r)
	
	runners, total, err := s.registry.ListRunners(page, pageSize)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	response := RunnerListResponse{
		Runners: runners,
		Pagination: &PaginationParams{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}
	
	s.sendSuccess(w, response)
}

// getRunner handles GET /api/runners/{id}
func (s *Server) getRunner(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	runnerID := vars["id"]
	
	runner, err := s.registry.GetRunner(runnerID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, err.Error())
		return
	}
	
	s.sendSuccess(w, runner)
}

// unregisterRunner handles DELETE /api/runners/{id}
func (s *Server) unregisterRunner(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	runnerID := vars["id"]
	
	err := s.registry.UnregisterRunner(runnerID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, err.Error())
		return
	}
	
	s.sendSuccess(w, map[string]string{"status": "unregistered"})
}

// getServerStatus handles GET /api/status
func (s *Server) getServerStatus(w http.ResponseWriter, r *http.Request) {
	queueStats := s.queue.GetStats()
	runnerStats := s.registry.GetStats()
	
	status := ServerStatus{
		Version:        s.version,
		StartTime:      time.Now(), // TODO: Track actual start time
		JobsTotal:      queueStats["total"],
		JobsPending:    queueStats["pending"],
		JobsRunning:    queueStats["running"],
		JobsCompleted:  queueStats["completed"],
		JobsFailed:     queueStats["failed"],
		RunnersOnline:  runnerStats["online"],
		RunnersOffline: runnerStats["offline"],
		RunnersBusy:    runnerStats["busy"],
	}
	
	s.sendSuccess(w, status)
}

// getStats handles GET /api/stats
func (s *Server) getStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"queue":   s.queue.GetStats(),
		"runners": s.registry.GetStats(),
	}
	
	s.sendSuccess(w, stats)
}

// healthCheck handles GET /api/health
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	s.sendSuccess(w, map[string]string{
		"status":  "ok",
		"version": s.version,
	})
}

// cancelWorkflow handles POST /api/workflows/{id}/cancel
func (s *Server) cancelWorkflow(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement workflow cancellation
	s.sendError(w, http.StatusNotImplemented, "Workflow cancellation not yet implemented")
}

// Helper methods

// getPaginationParams extracts pagination parameters from request
func (s *Server) getPaginationParams(r *http.Request) (int, int) {
	page := 1
	pageSize := 20
	
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}
	
	return page, pageSize
}

// sendSuccess sends a successful API response
func (s *Server) sendSuccess(w http.ResponseWriter, data interface{}) {
	response := APIResponse{
		Success: true,
		Data:    data,
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// sendError sends an error API response
func (s *Server) sendError(w http.ResponseWriter, statusCode int, message string) {
	response := APIResponse{
		Success: false,
		Error:   message,
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}