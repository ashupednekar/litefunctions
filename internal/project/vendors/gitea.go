package vendors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type GiteaClient struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

func NewGiteaClient(baseURL, token string) *GiteaClient {
	return &GiteaClient{
		token:   token,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *GiteaClient) CreateRepo(ctx context.Context, opts CreateRepoOptions) (*Repository, error) {
	url := fmt.Sprintf("%s/api/v1/user/repos", c.baseURL)

	payload := map[string]any{
		"name":        opts.Name,
		"description": opts.Description,
		"private":     opts.Private,
		"auto_init":   opts.AutoInit,
	}

	if opts.DefaultBranch != "" {
		payload["default_branch"] = opts.DefaultBranch
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var giteaRepo struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		Description   string `json:"description"`
		Private       bool   `json:"private"`
		HTMLURL       string `json:"html_url"`
		CloneURL      string `json:"clone_url"`
		SSHURL        string `json:"ssh_url"`
		DefaultBranch string `json:"default_branch"`
		CreatedAt     string `json:"created_at"`
		UpdatedAt     string `json:"updated_at"`
	}

	if err := json.Unmarshal(respBody, &giteaRepo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &Repository{
		ID:            giteaRepo.ID,
		Name:          giteaRepo.Name,
		FullName:      giteaRepo.FullName,
		Description:   giteaRepo.Description,
		Private:       giteaRepo.Private,
		HTMLURL:       giteaRepo.HTMLURL,
		CloneURL:      giteaRepo.CloneURL,
		SSHURL:        giteaRepo.SSHURL,
		DefaultBranch: giteaRepo.DefaultBranch,
		CreatedAt:     giteaRepo.CreatedAt,
		UpdatedAt:     giteaRepo.UpdatedAt,
	}, nil
}

func (c *GiteaClient) AddWebhook(ctx context.Context, owner, repo string, opts WebhookOptions) (*Webhook, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/hooks", c.baseURL, owner, repo)

	config := map[string]string{
		"url":          opts.URL,
		"content_type": opts.ContentType,
	}

	if opts.Secret != "" {
		config["secret"] = opts.Secret
	}

	events := opts.Events
	if len(events) == 0 {
		events = []string{"push"}
	}

	payload := map[string]any{
		"type":   "gitea",
		"active": opts.Active,
		"events": events,
		"config": config,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var giteaWebhook struct {
		ID        int64    `json:"id"`
		Events    []string `json:"events"`
		Active    bool     `json:"active"`
		CreatedAt string   `json:"created_at"`
		UpdatedAt string   `json:"updated_at"`
		Config    struct {
			URL string `json:"url"`
		} `json:"config"`
	}

	if err := json.Unmarshal(respBody, &giteaWebhook); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &Webhook{
		ID:        giteaWebhook.ID,
		URL:       giteaWebhook.Config.URL,
		Events:    giteaWebhook.Events,
		Active:    giteaWebhook.Active,
		CreatedAt: giteaWebhook.CreatedAt,
		UpdatedAt: giteaWebhook.UpdatedAt,
	}, nil
}

func (c *GiteaClient) GetActionsProgress(ctx context.Context, owner, repo string, opts ActionsProgressOptions) (*ActionsProgress, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/actions/runs", c.baseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	if opts.Branch != "" {
		q.Add("branch", opts.Branch)
	}
	if opts.Status != "" {
		q.Add("status", opts.Status)
	}
	if opts.Event != "" {
		q.Add("event", opts.Event)
	}
	if opts.WorkflowID != "" {
		q.Add("workflow_id", opts.WorkflowID)
	}
	if opts.Limit > 0 {
		q.Add("limit", fmt.Sprintf("%d", opts.Limit))
	} else {
		q.Add("limit", "30")
	}
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var giteaActions struct {
		TotalCount   int `json:"total_count"`
		WorkflowRuns []struct {
			ID           int64  `json:"id"`
			Name         string `json:"name"`
			Status       string `json:"status"`
			Conclusion   string `json:"conclusion"`
			HeadBranch   string `json:"head_branch"`
			Event        string `json:"event"`
			CreatedAt    string `json:"created_at"`
			UpdatedAt    string `json:"updated_at"`
			HTMLURL      string `json:"html_url"`
			WorkflowID   string `json:"workflow_id"`
			WorkflowName string `json:"workflow_name"`
		} `json:"workflow_runs"`
	}

	if err := json.Unmarshal(respBody, &giteaActions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	runs := make([]WorkflowRun, 0, len(giteaActions.WorkflowRuns))
	for _, run := range giteaActions.WorkflowRuns {
		var workflowID int64 //gitea uses int, unlike github
		fmt.Sscanf(run.WorkflowID, "%d", &workflowID)

		runs = append(runs, WorkflowRun{
			ID:           run.ID,
			Name:         run.Name,
			Status:       run.Status,
			Conclusion:   run.Conclusion,
			Branch:       run.HeadBranch,
			Event:        run.Event,
			CreatedAt:    run.CreatedAt,
			UpdatedAt:    run.UpdatedAt,
			HTMLURL:      run.HTMLURL,
			WorkflowID:   workflowID,
			WorkflowName: run.WorkflowName,
		})
	}

	return &ActionsProgress{
		TotalCount: giteaActions.TotalCount,
		Runs:       runs,
	}, nil
}

func (c *GiteaClient) DeleteRepo(ctx context.Context, owner, repo string) error {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s", c.baseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("gitea: failed to create delete request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitea: failed to execute delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gitea: unexpected delete status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
