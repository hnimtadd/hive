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
	BaseURL string `hive:"key=GITLAB_BASE_URL;description=GitLab API base URL (default: https://gitlab.com/api/v4);omitempty"`
}

// GetMergeRequestInput retrieves a specific merge request
type GetMergeRequestInput struct {
	Project string `json:"project" jsonschema:"description=Project ID or path (e.g. 'group/project' or '12345')"`
	MR      int    `json:"mr"      jsonschema:"description=Merge request IID (internal ID)"`
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

// ListMergeRequestsInput lists merge requests for a project
type ListMergeRequestsInput struct {
	Project string `json:"project" jsonschema:"description=Project ID or path (e.g. 'group/project')"`
	State   string `json:"state"   jsonschema:"description=Filter by state: opened, closed, merged, all (default: opened)"`
	Limit   int    `json:"limit"   jsonschema:"description=Maximum number of MRs to return (default: 20, max: 100)"`
}

type ListMergeRequestsOutput struct {
	MergeRequests []MergeRequest `json:"merge_requests"`
	Count         int            `json:"count"`
}

// GetIssueInput retrieves a specific issue
type GetIssueInput struct {
	Project string `json:"project" jsonschema:"description=Project ID or path"`
	Issue   int    `json:"issue"   jsonschema:"description=Issue IID (internal ID)"`
}

type Issue struct {
	IID         int       `json:"iid"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	State       string    `json:"state"`
	Author      User      `json:"author"`
	Assignees   []User    `json:"assignees"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	WebURL      string    `json:"web_url"`
	Labels      []string  `json:"labels"`
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

func (t *GitLabTool) getMergeRequest(ctx context.Context, input GetMergeRequestInput) (MergeRequest, error) {
	client := NewGitLabClient(t.secrets.Token, t.secrets.BaseURL)

	// Encode project path
	projectPath := strings.ReplaceAll(input.Project, "/", "%2F")
	path := fmt.Sprintf("/projects/%s/merge_requests/%d", projectPath, input.MR)

	body, err := client.doRequest("GET", path)
	if err != nil {
		return MergeRequest{}, err
	}

	var mr MergeRequest
	if err := json.Unmarshal(body, &mr); err != nil {
		return MergeRequest{}, fmt.Errorf("failed to parse response: %w", err)
	}

	return mr, nil
}

func (t *GitLabTool) listMergeRequests(ctx context.Context, input ListMergeRequestsInput) (ListMergeRequestsOutput, error) {
	client := NewGitLabClient(t.secrets.Token, t.secrets.BaseURL)

	// Set defaults
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

func (t *GitLabTool) getIssue(ctx context.Context, input GetIssueInput) (Issue, error) {
	client := NewGitLabClient(t.secrets.Token, t.secrets.BaseURL)

	projectPath := strings.ReplaceAll(input.Project, "/", "%2F")
	path := fmt.Sprintf("/projects/%s/issues/%d", projectPath, input.Issue)

	body, err := client.doRequest("GET", path)
	if err != nil {
		return Issue{}, err
	}

	var issue Issue
	if err := json.Unmarshal(body, &issue); err != nil {
		return Issue{}, fmt.Errorf("failed to parse response: %w", err)
	}

	return issue, nil
}

func main() {
	secrets := &GitLabSecrets{}
	gitlabTool := &GitLabTool{secrets: secrets}

	// Create get_merge_request tool
	getMRTool, err := hive.NewTool(
		"gitlab_get_mr",
		"Retrieve a specific GitLab merge request by project and MR IID",
		gitlabTool.getMergeRequest,
		hive.WithSecret[GetMergeRequestInput, MergeRequest](secrets),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create get_mr tool: %v\n", err)
		os.Exit(1)
	}

	// Create list_merge_requests tool
	listMRTool, err := hive.NewTool(
		"gitlab_list_mrs",
		"List merge requests for a GitLab project",
		gitlabTool.listMergeRequests,
		hive.WithSecret[ListMergeRequestsInput, ListMergeRequestsOutput](secrets),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create list_mrs tool: %v\n", err)
		os.Exit(1)
	}

	// Create get_issue tool
	getIssueTool, err := hive.NewTool(
		"gitlab_get_issue",
		"Retrieve a specific GitLab issue by project and issue IID",
		gitlabTool.getIssue,
		hive.WithSecret[GetIssueInput, Issue](secrets),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create get_issue tool: %v\n", err)
		os.Exit(1)
	}

	// For now, serve the first tool (you'd need a multi-tool server in practice)
	// Or create separate binaries for each
	fmt.Fprintf(os.Stderr, "GitLab tools created successfully\n")
	fmt.Fprintf(os.Stderr, "Tools: %s, %s, %s\n", getMRTool.Name(), listMRTool.Name(), getIssueTool.Name())

	// Serve the get_mr tool by default
	getMRTool.Serve()
}
