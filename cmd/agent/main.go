package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type AgentMessage struct {
	IssueID string `json:"issue_id"`
	Command string `json:"command"`
	Type    string `json:"type"`
}

type AgentResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

type Agent struct {
	issueID string
	port    string
}

func NewAgent() *Agent {
	return &Agent{
		issueID: os.Getenv("ISSUE_ID"),
		port:    getEnvOrDefault("AGENT_PORT", "8080"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (a *Agent) Start() error {
	http.HandleFunc("/health", a.healthHandler)
	http.HandleFunc("/exec", a.execHandler)
	
	addr := fmt.Sprintf(":%s", a.port)
	log.Printf("Agent started for issue %s, listening on %s", a.issueID, addr)
	
	return http.ListenAndServe(addr, nil)
}

func (a *Agent) healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":   "healthy",
		"issue_id": a.issueID,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (a *Agent) execHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg AgentMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate issue ID
	if msg.IssueID != a.issueID {
		resp := AgentResponse{
			Success: false,
			Error:   fmt.Sprintf("Issue ID mismatch: expected %s, got %s", a.issueID, msg.IssueID),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Execute command
	resp := a.executeCommand(msg.Command)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (a *Agent) executeCommand(command string) AgentResponse {
	log.Printf("Executing command: %s", command)

	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return AgentResponse{
			Success: false,
			Error:   "Empty command",
		}
	}

	// Execute command
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = "/workspace"
	output, err := cmd.CombinedOutput()

	resp := AgentResponse{
		Success: err == nil,
		Output:  string(output),
	}
	
	if err != nil {
		resp.Error = err.Error()
	}

	return resp
}

func main() {
	agent := NewAgent()
	
	if agent.issueID == "" {
		log.Fatal("ISSUE_ID environment variable not set")
	}

	log.Printf("Starting agent for issue: %s", agent.issueID)
	
	if err := agent.Start(); err != nil {
		log.Fatal("Failed to start agent:", err)
	}
}