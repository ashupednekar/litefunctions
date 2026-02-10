package vendors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ashupednekar/litefunctions/portal/internal/project/repo"
	"github.com/ashupednekar/litefunctions/portal/internal/project/vendors/workflows"
	"github.com/ashupednekar/litefunctions/portal/pkg"
)

type GiteaClient struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

type giteaStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type giteaJob struct {
	Name   string      `json:"name"`
	Status string      `json:"status"`
	Steps  []giteaStep `json:"steps"`
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

func (c *GiteaClient) AddWorkflow(project string) error {
	if err := repo.WriteFile(
		project,
		".gitea/workflows/ci.yaml",
		workflows.GiteaWorkflow,
		"writing workflow file",
	); err != nil {
		return err
	}
	return nil
}

func (c *GiteaClient) GetActionsProgress(ctx context.Context, owner, repo string, opts ActionsProgressOptions) (*ActionsProgress, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/actions/tasks", c.baseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
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
			DisplayTitle string `json:"display_title"`
			Status       string `json:"status"`
			HeadBranch   string `json:"head_branch"`
			Event        string `json:"event"`
			CreatedAt    string `json:"created_at"`
			UpdatedAt    string `json:"updated_at"`
			URL          string `json:"url"`
			WorkflowID   string `json:"workflow_id"`
		} `json:"workflow_runs"`
	}

	if err := json.Unmarshal(respBody, &giteaActions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	runs := make([]WorkflowRun, 0, len(giteaActions.WorkflowRuns))
	for _, run := range giteaActions.WorkflowRuns {
		var workflowID int64 //gitea uses int, unlike github
		fmt.Sscanf(run.WorkflowID, "%d", &workflowID)
		htmlURL := rewriteToFqdn(run.URL, pkg.Cfg.Fqdn)

		runs = append(runs, WorkflowRun{
			ID:           run.ID,
			Name:         run.Name,
			DisplayTitle: run.DisplayTitle,
			Status:       run.Status,
			Branch:       run.HeadBranch,
			Event:        run.Event,
			CreatedAt:    run.CreatedAt,
			UpdatedAt:    run.UpdatedAt,
			HTMLURL:      htmlURL,
			WorkflowID:   workflowID,
		})
	}

	c.populateCurrentSteps(ctx, owner, repo, runs)

	return &ActionsProgress{
		TotalCount: giteaActions.TotalCount,
		Runs:       runs,
	}, nil
}

func rewriteToFqdn(raw, fqdn string) string {
	if raw == "" {
		return raw
	}

	host := strings.TrimSpace(fqdn)
	if host == "" {
		return raw
	}

	scheme := "https"
	if strings.HasPrefix(host, "http://") {
		scheme = "http"
		host = strings.TrimPrefix(host, "http://")
	} else if strings.HasPrefix(host, "https://") {
		scheme = "https"
		host = strings.TrimPrefix(host, "https://")
	}

	trimmed := strings.TrimLeft(raw, "/")
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		if idx := strings.Index(trimmed, "://"); idx >= 0 {
			if slash := strings.Index(trimmed[idx+3:], "/"); slash >= 0 {
				trimmed = trimmed[idx+3+slash+1:]
			} else {
				trimmed = ""
			}
		}
	}

	if trimmed == "" {
		return fmt.Sprintf("%s://%s", scheme, host)
	}
	return fmt.Sprintf("%s://%s/%s", scheme, host, trimmed)
}

func (c *GiteaClient) populateCurrentSteps(ctx context.Context, owner, repo string, runs []WorkflowRun) {
	const maxLookups = 6
	lookups := 0
	for i := range runs {
		if lookups >= maxLookups {
			return
		}
		status := strings.ToLower(runs[i].Status)
		if status != "in_progress" && status != "queued" && status != "waiting" && status != "running" {
			continue
		}
		job, step, ok := c.getRunCurrentStep(ctx, owner, repo, runs[i].ID)
		if ok {
			runs[i].CurrentJob = job
			runs[i].CurrentStep = step
		}
		lookups++
	}
}

func (c *GiteaClient) getRunCurrentStep(ctx context.Context, owner, repo string, runID int64) (string, string, bool) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/actions/runs/%d/jobs", c.baseURL, owner, repo, runID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", false
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", false
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", false
	}

	var payload struct {
		Jobs []giteaJob `json:"jobs"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return "", "", false
	}

	jobName, stepName := pickGiteaJobStep(payload.Jobs)
	if jobName == "" && stepName == "" {
		return "", "", false
	}
	return jobName, stepName, true
}

func pickGiteaJobStep(jobs []giteaJob) (string, string) {
	if len(jobs) == 0 {
		return "", ""
	}

	pickStep := func(steps []giteaStep) string {
		if len(steps) == 0 {
			return ""
		}
		for _, step := range steps {
			if strings.EqualFold(step.Status, "in_progress") || strings.EqualFold(step.Status, "running") {
				return step.Name
			}
		}
		for _, step := range steps {
			if strings.EqualFold(step.Status, "queued") || strings.EqualFold(step.Status, "waiting") {
				return step.Name
			}
		}
		return steps[len(steps)-1].Name
	}

	for _, job := range jobs {
		if strings.EqualFold(job.Status, "in_progress") || strings.EqualFold(job.Status, "running") {
			return job.Name, pickStep(job.Steps)
		}
	}
	for _, job := range jobs {
		if strings.EqualFold(job.Status, "queued") || strings.EqualFold(job.Status, "waiting") {
			return job.Name, pickStep(job.Steps)
		}
	}

	return jobs[0].Name, pickStep(jobs[0].Steps)
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
