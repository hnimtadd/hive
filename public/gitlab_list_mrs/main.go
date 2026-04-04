package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hnimtadd/hive/pkg/hive"
)

type GitLabSecrets struct {
	Token   string `hive:"key=GITLAB_TOKEN;description=GitLab personal access token or OAuth token;required"`
	BaseURL string `hive:"key=GITLAB_BASE_URL;description=GitLab API base URL (default https://gitlab.com/api/v4);omitempty"`
}

type ListMergeRequestsInput struct {
	Project string `json:"project" jsonschema:"description=Project ID or path (e.g. 'group/project')"`
	State   string `json:"state" jsonschema:"description=Filter by state opened, closed, merged, all (default opened)"`
	Limit   int    `json:"limit" jsonschema:"description=Maximum number of MRs to return (default 20, max 100)"`
}

type ListMergeRequestsOutput struct {
	MergeRequests []MergeRequest `json:"merge_requests"`
	Count         int            `json:"count"`
}

type MergeRequest struct {
	IID          int       `json:"iid"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	State        string    `json:"state"`
	Author       User      `json:"author"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	WebURL       string    `json:"web_url"`
	SHA          string    `json:"sha"`
	MergeStatus  string    `json:"merge_status"`
	Labels       []string  `json:"labels"`
}

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type GitLabClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewGitLabClient(token, baseURL string) *GitLabClient {
	if baseURL == "" {
		baseURL = "https://gitlab.com/api/v4"
	}
	return &GitLabClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *GitLabClient) doRequest(method, path string) ([]byte, error) {
	url := c.baseURL + path

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GitLab API error (status %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

type GitLabTool struct {
	secrets *GitLabSecrets
}

func (t *GitLabTool) listMergeRequests(ctx context.Context, input ListMergeRequestsInput) (ListMergeRequestsOutput, error) {
	client := NewGitLabClient(t.secrets.Token, t.secrets.BaseURL)

	if input.State == "" {
		input.State = "opened"
	}
	if input.Limit == 0 {
		input.Limit = 20
	}
	if input.Limit > 100 {
		input.Limit = 100
	}

	projectPath := strings.ReplaceAll(input.Project, "/", "%2F")
	path := fmt.Sprintf("/projects/%s/merge_requests?state=%s&per_page=%d", projectPath, input.State, input.Limit)

	body, err := client.doRequest("GET", path)
	if err != nil {
		return ListMergeRequestsOutput{}, err
	}

	var mrs []MergeRequest
	if err := json.Unmarshal(body, &mrs); err != nil {
		return ListMergeRequestsOutput{}, fmt.Errorf("failed to parse response: %w", err)
	}

	return ListMergeRequestsOutput{
		MergeRequests: mrs,
		Count:         len(mrs),
	}, nil
}

func main() {
	secrets := &GitLabSecrets{}
	gitlabTool := &GitLabTool{secrets: secrets}

	tool, err := hive.NewTool(
		"gitlab_list_mrs",
		"List merge requests for a GitLab project",
		gitlabTool.listMergeRequests,
		hive.WithSecret[ListMergeRequestsInput, ListMergeRequestsOutput](secrets),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create tool: %v\n", err)
		os.Exit(1)
	}

	tool.Serve()
}
