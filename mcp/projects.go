package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type projectInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// portalRequest calls the portal's cluster-internal project API. The portal
// owns project rows, repositories, webhooks, and workflows, so the MCP server
// never creates them directly.
func (s *Server) portalRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	base := strings.TrimRight(strings.TrimSpace(s.settings.PortalURL), "/")
	if base == "" {
		return nil, fmt.Errorf("portal URL is not configured")
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	requestCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, method, base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := strings.TrimSpace(s.settings.PortalToken); token != "" {
		req.Header.Set("X-MCP-Token", token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call portal: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("portal returned %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	return payload, nil
}

func (s *Server) listProjects(ctx context.Context, raw json.RawMessage) (string, error) {
	payload, err := s.portalRequest(ctx, http.MethodGet, "/internal/projects/", nil)
	if err != nil {
		return "", err
	}
	var response struct {
		Projects []projectInfo `json:"projects"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return "", fmt.Errorf("decode projects: %w", err)
	}
	if response.Projects == nil {
		response.Projects = []projectInfo{}
	}
	return marshalResult(map[string]any{"projects": response.Projects})
}

func (s *Server) getProject(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	name, err := projectName(args.Name)
	if err != nil {
		return "", err
	}
	payload, err := s.portalRequest(ctx, http.MethodGet, "/internal/projects/"+name+"/", nil)
	if err != nil {
		return "", err
	}
	var project projectInfo
	if err := json.Unmarshal(payload, &project); err != nil {
		return "", fmt.Errorf("decode project: %w", err)
	}
	return marshalResult(map[string]any{
		"project":  project,
		"selected": project.Name,
		"hint":     "Pass this name as project to the other tools.",
	})
}

func (s *Server) createProject(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	name, err := projectName(args.Name)
	if err != nil {
		return "", err
	}
	payload, err := s.portalRequest(ctx, http.MethodPost, "/internal/projects/", map[string]string{"name": name})
	if err != nil {
		return "", err
	}
	var project projectInfo
	if err := json.Unmarshal(payload, &project); err != nil {
		return "", fmt.Errorf("decode project: %w", err)
	}
	return marshalResult(map[string]any{"project": project, "status": "created"})
}

func projectName(value string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(value))
	if !functionNamePattern.MatchString(name) {
		return "", fmt.Errorf("invalid project name %q", value)
	}
	return name, nil
}
