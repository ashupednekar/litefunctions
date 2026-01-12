package vendors

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/ashupednekar/litewebservices-portal/internal/project/vendors/workflows"
)

const (
	GitHubAPIBaseURL = "https://api.github.com"
)

type GitHubClient struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

func NewGitHubClient(token string) *GitHubClient {
	return &GitHubClient{
		token:   token,
		baseURL: GitHubAPIBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *GitHubClient) CreateRepo(ctx context.Context, opts CreateRepoOptions) (*Repository, error) {
	url := fmt.Sprintf("%s/user/repos", c.baseURL)

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

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

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

	var ghRepo struct {
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

	if err := json.Unmarshal(respBody, &ghRepo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &Repository{
		ID:            ghRepo.ID,
		Name:          ghRepo.Name,
		FullName:      ghRepo.FullName,
		Description:   ghRepo.Description,
		Private:       ghRepo.Private,
		HTMLURL:       ghRepo.HTMLURL,
		CloneURL:      ghRepo.CloneURL,
		SSHURL:        ghRepo.SSHURL,
		DefaultBranch: ghRepo.DefaultBranch,
		CreatedAt:     ghRepo.CreatedAt,
		UpdatedAt:     ghRepo.UpdatedAt,
	}, nil
}

func (c *GitHubClient) AddWorkflow(
	ctx context.Context,
	owner, repo string,
) error {
	repository, err := c.GetRepo(ctx, owner, repo)
	if err != nil{
		return fmt.Errorf("failed to get repo :%s", err)
	}
	content := base64.StdEncoding.EncodeToString(workflows.GitHubWorkflow)
	url := fmt.Sprintf(
		"%s/repos/%s/%s/contents/.github/workflows/ci.yaml",
		c.baseURL, owner, repo,
	)
	slog.Warn("branch: ", "debug", repository.DefaultBranch)
	payload := map[string]any{
		"message": "Add default GitHub Actions workflow",
		"content": content,
		"branch":  repository.DefaultBranch,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		url,
		bytes.NewBuffer(body),
	)
	if err != nil{
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"add workflow failed (%d): %s",
			resp.StatusCode, b,
		)
	}
	return nil
}

func (c *GitHubClient) AddWebhook(ctx context.Context, owner, repo string, opts WebhookOptions) (*Webhook, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/hooks", c.baseURL, owner, repo)

	config := map[string]any{
		"url":          opts.URL,
		"content_type": opts.ContentType,
	}

	if opts.Secret != "" {
		config["secret"] = opts.Secret
	}

	if opts.InsecureSSL {
		config["insecure_ssl"] = "1"
	} else {
		config["insecure_ssl"] = "0"
	}

	events := opts.Events
	if len(events) == 0 {
		events = []string{"push"}
	}

	payload := map[string]any{
		"name":   "web",
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

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

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

	var ghWebhook struct {
		ID        int64    `json:"id"`
		URL       string   `json:"url"`
		Events    []string `json:"events"`
		Active    bool     `json:"active"`
		CreatedAt string   `json:"created_at"`
		UpdatedAt string   `json:"updated_at"`
		Config    struct {
			URL string `json:"url"`
		} `json:"config"`
	}

	if err := json.Unmarshal(respBody, &ghWebhook); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &Webhook{
		ID:        ghWebhook.ID,
		URL:       ghWebhook.Config.URL,
		Events:    ghWebhook.Events,
		Active:    ghWebhook.Active,
		CreatedAt: ghWebhook.CreatedAt,
		UpdatedAt: ghWebhook.UpdatedAt,
	}, nil
}

// GetActionsProgress gets the status of GitHub Actions workflows
func (c *GitHubClient) GetActionsProgress(ctx context.Context, owner, repo string, opts ActionsProgressOptions) (*ActionsProgress, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs", c.baseURL, owner, repo)

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
	if opts.Limit > 0 {
		q.Add("per_page", fmt.Sprintf("%d", opts.Limit))
	} else {
		q.Add("per_page", "30")
	}
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

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

	var ghActions struct {
		TotalCount   int `json:"total_count"`
		WorkflowRuns []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HeadBranch string `json:"head_branch"`
			Event      string `json:"event"`
			CreatedAt  string `json:"created_at"`
			UpdatedAt  string `json:"updated_at"`
			HTMLURL    string `json:"html_url"`
			WorkflowID int64  `json:"workflow_id"`
		} `json:"workflow_runs"`
	}

	if err := json.Unmarshal(respBody, &ghActions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	runs := make([]WorkflowRun, 0, len(ghActions.WorkflowRuns))
	for _, run := range ghActions.WorkflowRuns {
		runs = append(runs, WorkflowRun{
			ID:         run.ID,
			Name:       run.Name,
			Status:     run.Status,
			Conclusion: run.Conclusion,
			Branch:     run.HeadBranch,
			Event:      run.Event,
			CreatedAt:  run.CreatedAt,
			UpdatedAt:  run.UpdatedAt,
			HTMLURL:    run.HTMLURL,
			WorkflowID: run.WorkflowID,
		})
	}

	return &ActionsProgress{
		TotalCount: ghActions.TotalCount,
		Runs:       runs,
	}, nil
}


func (c *GitHubClient) DeleteRepo(ctx context.Context, owner, repo string) error {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("github: failed to create delete request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: failed to execute delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github: unexpected delete status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}


func (c *GitHubClient) GetRepo(
	ctx context.Context,
	owner, repo string,
) (*Repository, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get repo failed: %s", b)
	}
	var r Repository
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	r.DefaultBranch = "main"
	return &r, nil
}
