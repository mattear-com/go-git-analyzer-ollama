package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
	"github.com/arturoeanton/go-git-analyzer-ollama/internal/service"
)

// Server implements the Model Context Protocol (MCP) server.
// It exposes tools for external AI agents to interact with CodeLens AI.
type Server struct {
	ragService      *service.RAGService
	analysisService *service.AnalysisService
	port            string
}

// NewServer creates a new MCP server.
func NewServer(ragService *service.RAGService, analysisService *service.AnalysisService, port string) *Server {
	return &Server{
		ragService:      ragService,
		analysisService: analysisService,
		port:            port,
	}
}

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Start begins the MCP server on the configured port.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleRPC)
	mux.HandleFunc("/mcp/sse", s.handleSSE)

	slog.Info("MCP server starting", "port", s.port)
	return http.ListenAndServe(":"+s.port, mux)
}

func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, nil, -32700, "parse error")
		return
	}

	var result interface{}
	var err error

	switch req.Method {
	case "tools/list":
		result = s.listTools()
	case "tools/call":
		result, err = s.callTool(r.Context(), req.Params)
	case "initialize":
		result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "codelens-ai",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{"listChanged": false},
			},
		}
	default:
		writeError(w, req.ID, -32601, "method not found")
		return
	}

	if err != nil {
		writeError(w, req.ID, -32603, err.Error())
		return
	}

	writeResult(w, req.ID, result)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial endpoint message
	fmt.Fprintf(w, "event: endpoint\ndata: /mcp\n\n")
	w.(http.Flusher).Flush()

	// Keep connection alive
	<-r.Context().Done()
}

func (s *Server) listTools() map[string]interface{} {
	tools := []Tool{
		{
			Name:        "search_code",
			Description: "Search code in a repository using semantic similarity",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"repo_id": {"type": "string", "description": "Repository ID"},
					"query": {"type": "string", "description": "Search query"}
				},
				"required": ["repo_id", "query"]
			}`),
		},
		{
			Name:        "analyze_repo",
			Description: "Run an analysis strategy on a repository",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"repo_id": {"type": "string", "description": "Repository ID"},
					"strategy": {"type": "string", "description": "Strategy name: architecture, code_quality, functionality, devops"}
				},
				"required": ["repo_id", "strategy"]
			}`),
		},
		{
			Name:        "list_strategies",
			Description: "List available analysis strategies",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
	}
	return map[string]interface{}{"tools": tools}
}

func (s *Server) callTool(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var req struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	switch req.Name {
	case "search_code":
		var args struct {
			RepoID string `json:"repo_id"`
			Query  string `json:"query"`
		}
		json.Unmarshal(req.Arguments, &args)

		answer, chunks, err := s.ragService.Query(ctx, args.RepoID, args.Query)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": answer},
			},
			"sources": chunks,
		}, nil

	case "analyze_repo":
		var args struct {
			RepoID   string `json:"repo_id"`
			Strategy string `json:"strategy"`
		}
		json.Unmarshal(req.Arguments, &args)

		result, err := s.analysisService.RunStrategy(ctx, args.Strategy, port.AnalysisRequest{
			RepoID: args.RepoID,
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": result.Summary},
			},
		}, nil

	case "list_strategies":
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("Available strategies: %v", s.analysisService.ListStrategies())},
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", req.Name)
	}
}

func writeResult(w http.ResponseWriter, id interface{}, result interface{}) {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: message}}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
