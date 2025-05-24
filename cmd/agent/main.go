package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
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
	issueID    string
	socketPath string
}

func NewAgent() *Agent {
	return &Agent{
		issueID:    os.Getenv("ISSUE_ID"),
		socketPath: os.Getenv("SOCKET_PATH"),
	}
}

func (a *Agent) Start() error {
	// Remove existing socket if it exists
	os.Remove(a.socketPath)

	// Create Unix socket
	listener, err := net.Listen("unix", a.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	defer listener.Close()

	log.Printf("Agent started for issue %s, listening on %s", a.issueID, a.socketPath)

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go a.handleConnection(conn)
	}
}

func (a *Agent) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read message
	var msg AgentMessage
	if err := json.NewDecoder(conn).Decode(&msg); err != nil {
		log.Printf("Failed to decode message: %v", err)
		return
	}

	// Validate issue ID
	if msg.IssueID != a.issueID {
		resp := AgentResponse{
			Success: false,
			Error:   fmt.Sprintf("Issue ID mismatch: expected %s, got %s", a.issueID, msg.IssueID),
		}
		json.NewEncoder(conn).Encode(resp)
		return
	}

	// Handle command
	switch msg.Type {
	case "exec":
		a.handleExec(conn, msg.Command)
	default:
		resp := AgentResponse{
			Success: false,
			Error:   fmt.Sprintf("Unknown message type: %s", msg.Type),
		}
		json.NewEncoder(conn).Encode(resp)
	}
}

func (a *Agent) handleExec(conn net.Conn, command string) {
	log.Printf("Executing command: %s", command)

	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		resp := AgentResponse{
			Success: false,
			Error:   "Empty command",
		}
		json.NewEncoder(conn).Encode(resp)
		return
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

	// Send response
	if err := json.NewEncoder(conn).Encode(resp); err != nil {
		log.Printf("Failed to send response: %v", err)
	}
}

func main() {
	agent := NewAgent()
	
	if agent.issueID == "" {
		log.Fatal("ISSUE_ID environment variable not set")
	}
	
	if agent.socketPath == "" {
		agent.socketPath = "/tmp/claude-agent.sock"
	}

	if err := agent.Start(); err != nil {
		log.Fatal("Failed to start agent:", err)
	}
}