package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	vcs "github.com/ashupednekar/litefunctions/common/vcs/pkg"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

const (
	jsonRPCVersion = "2.0"
	serverName     = "litefunctions"
	serverVersion  = "0.1.0"
	maxBodyBytes   = 8 << 20
)

// Server is a minimal Streamable HTTP MCP server. It uses JSON-RPC over POST
// and exposes the LiteFunctions project tools described in tools.go.
type Server struct {
	settings Settings
	vcs      vcs.Config
	tools    map[string]tool
	repoMu   sync.Mutex
}

func NewServer(settings Settings, vcsCfg vcs.Config) *Server {
	s := &Server{settings: settings, vcs: vcsCfg}
	s.tools = s.buildTools()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.settings.Path != "" && r.URL.Path != s.settings.Path {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodPost:
		s.handleRPC(w, r)
	case http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	case http.MethodGet:
		w.Header().Set("Allow", "POST, DELETE")
		http.Error(w, "this MCP server uses Streamable HTTP POST", http.StatusMethodNotAllowed)
	default:
		w.Header().Set("Allow", "POST, DELETE")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	if s.settings.Token != "" && !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		writeRPC(w, rpcResponse{JSONRPC: jsonRPCVersion, Error: &rpcError{Code: -32700, Message: "failed to read body"}})
		return
	}

	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeRPC(w, rpcResponse{JSONRPC: jsonRPCVersion, Error: &rpcError{Code: -32700, Message: "parse error"}})
		return
	}
	if req.JSONRPC != "" && req.JSONRPC != jsonRPCVersion {
		writeRPC(w, rpcResponse{JSONRPC: jsonRPCVersion, ID: req.ID, Error: &rpcError{Code: -32600, Message: "invalid jsonrpc version"}})
		return
	}

	// Notifications have no id and must not receive a JSON-RPC response.
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	result, rpcErr := s.dispatch(r.Context(), req)
	if rpcErr != nil {
		writeRPC(w, rpcResponse{JSONRPC: jsonRPCVersion, ID: req.ID, Error: rpcErr})
		return
	}
	writeRPC(w, rpcResponse{JSONRPC: jsonRPCVersion, ID: req.ID, Result: result})
}

func (s *Server) dispatch(ctx context.Context, req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return s.initialize(req.Params), nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": s.toolList()}, nil
	case "tools/call":
		var call struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &call); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid tools/call params"}
		}
		t, ok := s.tools[call.Name]
		if !ok {
			return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("unknown tool %q", call.Name)}
		}
		args := call.Arguments
		if len(strings.TrimSpace(string(args))) == 0 {
			args = json.RawMessage(`{}`)
		}
		text, err := t.Handler(ctx, args)
		if err != nil {
			return map[string]any{
				"content": []map[string]string{{"type": "text", "text": err.Error()}},
				"isError": true,
			}, nil
		}
		return map[string]any{
			"content": []map[string]string{{"type": "text", "text": text}},
			"isError": false,
		}, nil
	default:
		return nil, &rpcError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}
}

func (s *Server) initialize(params json.RawMessage) map[string]any {
	version := "2025-06-18"
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(params) > 0 && json.Unmarshal(params, &init) == nil && init.ProtocolVersion != "" {
		version = init.ProtocolVersion
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities": map[string]any{
			"tools": map[string]any{"listChanged": false},
		},
		"serverInfo": map[string]any{
			"name":    serverName,
			"version": serverVersion,
		},
		"instructions": serverInstructions,
	}
}

func (s *Server) toolList() []map[string]any {
	out := make([]map[string]any, 0, len(s.tools))
	for _, name := range toolOrder {
		t, ok := s.tools[name]
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}
	return out
}

func (s *Server) authorized(r *http.Request) bool {
	if strings.TrimSpace(s.settings.Token) == "" {
		return true
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		token = r.Header.Get("X-MCP-Token")
	}
	return token == s.settings.Token
}

func writeRPC(w http.ResponseWriter, response rpcResponse) {
	response.JSONRPC = jsonRPCVersion
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("write MCP response: %v", err)
	}
}

var errNotFound = errors.New("not found")

const serverInstructions = `LiteFunctions MCP server.

Use these tools to build and edit functions in a LiteFunctions project.
Supported artefacts: sync HTTP functions, WebSocket/SSE functions, and async
NATS consumers. Databases, persistent streams, object storage, traffic
policies, and scheduled jobs are not supported yet; tell the user that instead
of inventing APIs.

Always pass the project name supplied by the portal. Keep function packages at
functions/<language>/<name>/handle.<ext>. Prefer create_function and
update_function so the portal can sync metadata after the turn.`
