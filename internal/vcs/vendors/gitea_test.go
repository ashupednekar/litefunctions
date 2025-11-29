package vendors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGiteaClient_CreateRepo(t *testing.T) {
	tests := []struct {
		name           string
		opts           CreateRepoOptions
		mockResponse   any
		mockStatusCode int
		wantErr        bool
		wantRepoName   string
	}{
		{
			name: "successful repo creation",
			opts: CreateRepoOptions{
				Name:        "test-repo",
				Description: "Test repository",
				Private:     false,
				AutoInit:    true,
			},
			mockResponse: map[string]any{
				"id":             12345,
				"name":           "test-repo",
				"full_name":      "testuser/test-repo",
				"description":    "Test repository",
				"private":        false,
				"html_url":       "https://gitea.example.com/testuser/test-repo",
				"clone_url":      "https://gitea.example.com/testuser/test-repo.git",
				"ssh_url":        "git@gitea.example.com:testuser/test-repo.git",
				"default_branch": "main",
				"created_at":     "2023-01-01T00:00:00Z",
				"updated_at":     "2023-01-01T00:00:00Z",
			},
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
			wantRepoName:   "test-repo",
		},
		{
			name: "repo creation with custom branch",
			opts: CreateRepoOptions{
				Name:          "test-repo-2",
				DefaultBranch: "develop",
			},
			mockResponse: map[string]any{
				"id":             12346,
				"name":           "test-repo-2",
				"full_name":      "testuser/test-repo-2",
				"default_branch": "develop",
				"html_url":       "https://gitea.example.com/testuser/test-repo-2",
				"clone_url":      "https://gitea.example.com/testuser/test-repo-2.git",
				"ssh_url":        "git@gitea.example.com:testuser/test-repo-2.git",
				"created_at":     "2023-01-01T00:00:00Z",
				"updated_at":     "2023-01-01T00:00:00Z",
			},
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
			wantRepoName:   "test-repo-2",
		},
		{
			name: "repo creation failure - conflict",
			opts: CreateRepoOptions{
				Name: "existing-repo",
			},
			mockResponse: map[string]any{
				"message": "Repository creation failed.",
			},
			mockStatusCode: http.StatusConflict,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.Method != "POST" {
					t.Errorf("Expected POST request, got %s", r.Method)
				}
				if r.URL.Path != "/api/v1/user/repos" {
					t.Errorf("Expected path /api/v1/user/repos, got %s", r.URL.Path)
				}

				// Check headers
				if auth := r.Header.Get("Authorization"); auth != "token test-token" {
					t.Errorf("Expected Authorization header 'token test-token', got '%s'", auth)
				}

				// Send mock response
				w.WriteHeader(tt.mockStatusCode)
				json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			// Create client with mock server
			client := NewGiteaClient(server.URL, "test-token")

			// Execute test
			repo, err := client.CreateRepo(context.Background(), tt.opts)

			// Verify results
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && repo != nil {
				if repo.Name != tt.wantRepoName {
					t.Errorf("CreateRepo() repo.Name = %v, want %v", repo.Name, tt.wantRepoName)
				}
			}
		})
	}
}

func TestGiteaClient_AddWebhook(t *testing.T) {
	tests := []struct {
		name           string
		owner          string
		repo           string
		opts           WebhookOptions
		mockResponse   any
		mockStatusCode int
		wantErr        bool
		wantWebhookID  int64
	}{
		{
			name:  "successful webhook creation",
			owner: "testuser",
			repo:  "test-repo",
			opts: WebhookOptions{
				URL:         "https://example.com/webhook",
				ContentType: "json",
				Secret:      "secret123",
				Events:      []string{"push", "pull_request"},
				Active:      true,
			},
			mockResponse: map[string]any{
				"id":     123,
				"events": []string{"push", "pull_request"},
				"active": true,
				"config": map[string]any{
					"url":          "https://example.com/webhook",
					"content_type": "json",
				},
				"created_at": "2023-01-01T00:00:00Z",
				"updated_at": "2023-01-01T00:00:00Z",
			},
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
			wantWebhookID:  123,
		},
		{
			name:  "webhook creation with default events",
			owner: "testuser",
			repo:  "test-repo",
			opts: WebhookOptions{
				URL:    "https://example.com/webhook",
				Active: true,
			},
			mockResponse: map[string]any{
				"id":     124,
				"events": []string{"push"},
				"active": true,
				"config": map[string]any{
					"url": "https://example.com/webhook",
				},
				"created_at": "2023-01-01T00:00:00Z",
				"updated_at": "2023-01-01T00:00:00Z",
			},
			mockStatusCode: http.StatusCreated,
			wantErr:        false,
			wantWebhookID:  124,
		},
		{
			name:  "webhook creation failure - not found",
			owner: "testuser",
			repo:  "nonexistent-repo",
			opts: WebhookOptions{
				URL: "https://example.com/webhook",
			},
			mockResponse: map[string]any{
				"message": "Not Found",
			},
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.Method != "POST" {
					t.Errorf("Expected POST request, got %s", r.Method)
				}
				expectedPath := "/api/v1/repos/" + tt.owner + "/" + tt.repo + "/hooks"
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				// Send mock response
				w.WriteHeader(tt.mockStatusCode)
				json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			// Create client with mock server
			client := NewGiteaClient(server.URL, "test-token")

			// Execute test
			webhook, err := client.AddWebhook(context.Background(), tt.owner, tt.repo, tt.opts)

			// Verify results
			if (err != nil) != tt.wantErr {
				t.Errorf("AddWebhook() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && webhook != nil {
				if webhook.ID != tt.wantWebhookID {
					t.Errorf("AddWebhook() webhook.ID = %v, want %v", webhook.ID, tt.wantWebhookID)
				}
			}
		})
	}
}

func TestGiteaClient_GetActionsProgress(t *testing.T) {
	tests := []struct {
		name           string
		owner          string
		repo           string
		opts           ActionsProgressOptions
		mockResponse   any
		mockStatusCode int
		wantErr        bool
		wantTotalCount int
		wantRunsCount  int
	}{
		{
			name:  "successful actions query",
			owner: "testuser",
			repo:  "test-repo",
			opts: ActionsProgressOptions{
				Branch: "main",
				Status: "completed",
				Limit:  10,
			},
			mockResponse: map[string]any{
				"total_count": 2,
				"workflow_runs": []map[string]any{
					{
						"id":            1001,
						"name":          "CI",
						"status":        "completed",
						"conclusion":    "success",
						"head_branch":   "main",
						"event":         "push",
						"created_at":    "2023-01-01T00:00:00Z",
						"updated_at":    "2023-01-01T00:10:00Z",
						"html_url":      "https://gitea.example.com/testuser/test-repo/actions/runs/1001",
						"workflow_id":   "5",
						"workflow_name": "CI Workflow",
					},
					{
						"id":            1002,
						"name":          "Tests",
						"status":        "completed",
						"conclusion":    "failure",
						"head_branch":   "main",
						"event":         "push",
						"created_at":    "2023-01-01T00:00:00Z",
						"updated_at":    "2023-01-01T00:15:00Z",
						"html_url":      "https://gitea.example.com/testuser/test-repo/actions/runs/1002",
						"workflow_id":   "6",
						"workflow_name": "Test Workflow",
					},
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantTotalCount: 2,
			wantRunsCount:  2,
		},
		{
			name:  "empty actions result",
			owner: "testuser",
			repo:  "test-repo",
			opts:  ActionsProgressOptions{},
			mockResponse: map[string]any{
				"total_count":   0,
				"workflow_runs": []map[string]any{},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantTotalCount: 0,
			wantRunsCount:  0,
		},
		{
			name:  "actions query failure - not found",
			owner: "testuser",
			repo:  "nonexistent-repo",
			opts:  ActionsProgressOptions{},
			mockResponse: map[string]any{
				"message": "Not Found",
			},
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				expectedPath := "/api/v1/repos/" + tt.owner + "/" + tt.repo + "/actions/runs"
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				if tt.opts.Branch != "" {
					if branch := r.URL.Query().Get("branch"); branch != tt.opts.Branch {
						t.Errorf("Expected branch query param %s, got %s", tt.opts.Branch, branch)
					}
				}

				w.WriteHeader(tt.mockStatusCode)
				json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			client := NewGiteaClient(server.URL, "test-token")

			progress, err := client.GetActionsProgress(context.Background(), tt.owner, tt.repo, tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetActionsProgress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && progress != nil {
				if progress.TotalCount != tt.wantTotalCount {
					t.Errorf("GetActionsProgress() progress.TotalCount = %v, want %v", progress.TotalCount, tt.wantTotalCount)
				}
				if len(progress.Runs) != tt.wantRunsCount {
					t.Errorf("GetActionsProgress() len(progress.Runs) = %v, want %v", len(progress.Runs), tt.wantRunsCount)
				}
			}
		})
	}
}
