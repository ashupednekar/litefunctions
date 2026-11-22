package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	vcs "github.com/ashupednekar/litefunctions/common/vcs/pkg"
)

func TestInitializeAndListTools(t *testing.T) {
	server := httptest.NewServer(NewServer(Settings{Path: "/mcp"}, vcs.Config{}))
	defer server.Close()

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
	initRes := postMCP(t, server.URL+"/mcp", initBody)
	if initRes["jsonrpc"] != "2.0" || initRes["id"].(float64) != 1 {
		t.Fatalf("unexpected initialize response: %#v", initRes)
	}
	result := initRes["result"].(map[string]any)
	if result["protocolVersion"] != "2025-06-18" {
		t.Fatalf("protocol version = %#v", result["protocolVersion"])
	}
	if result["instructions"] == "" {
		t.Fatalf("missing instructions")
	}

	listRes := postMCP(t, server.URL+"/mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	tools := listRes["result"].(map[string]any)["tools"].([]any)
	names := map[string]bool{}
	for _, raw := range tools {
		names[raw.(map[string]any)["name"].(string)] = true
	}
	for _, want := range toolOrder {
		if !names[want] {
			t.Fatalf("missing tool %q in %#v", want, names)
		}
	}
}

func TestToolsCallUnknownTool(t *testing.T) {
	server := httptest.NewServer(NewServer(Settings{Path: "/mcp"}, vcs.Config{}))
	defer server.Close()

	res := postMCP(t, server.URL+"/mcp", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)
	rpcErr, ok := res["error"].(map[string]any)
	if !ok || rpcErr["code"].(float64) != -32602 {
		t.Fatalf("expected invalid params error, got %#v", res)
	}
}

func TestTokenAuth(t *testing.T) {
	server := httptest.NewServer(NewServer(Settings{Path: "/mcp", Token: "secret"}, vcs.Config{}))
	defer server.Close()

	res, err := http.Post(server.URL+"/mcp", "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func postMCP(t *testing.T, url, body string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()
	var decoded map[string]any
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return decoded
}
