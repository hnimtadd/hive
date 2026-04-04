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

type JiraSecrets struct {
	Email   string `hive:"key=JIRA_EMAIL;description=Jira account email;required"`
	Token   string `hive:"key=JIRA_API_TOKEN;description=Jira API token;required"`
	BaseURL string `hive:"key=JIRA_BASE_URL;description=Jira instance URL (e.g. https://yourcompany.atlassian.net);required"`
}

// GetIssueInput retrieves a specific issue
type GetIssueInput struct {
	IssueKey string `json:"issue_key" jsonschema:"description=Issue key (e.g. 'PROJ-123')"`
}

type Issue struct {
	Key    string     `json:"key"`
	Fields IssueFields `json:"fields"`
}

type IssueFields struct {
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Status      Status      `json:"status"`
	IssueType   IssueType   `json:"issuetype"`
	Priority    Priority    `json:"priority"`
	Assignee    *User       `json:"assignee"`
	Reporter    User        `json:"reporter"`
	Created     time.Time   `json:"created"`
	Updated     time.Time   `json:"updated"`
	Labels      []string    `json:"labels"`
}

type Status struct {
	Name string `json:"name"`
}

type IssueType struct {
	Name string `json:"name"`
}

type Priority struct {
	Name string `json:"name"`
}

type User struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

// SearchIssuesInput searches for issues using JQL
type SearchIssuesInput struct {
	JQL        string `json:"jql" jsonschema:"description=JQL query string (e.g. 'project = PROJ AND status = Open')"`
	MaxResults int    `json:"max_results" jsonschema:"description=Maximum number of issues to return (default: 50, max: 100)"`
}

type SearchIssuesOutput struct {
	Issues     []Issue `json:"issues"`
	Total      int     `json:"total"`
	MaxResults int     `json:"max_results"`
}

type JiraClient struct {
	baseURL string
	email   string
	token   string
	client  *http.Client
}

func NewJiraClient(email, token, baseURL string) *JiraClient {
	return &JiraClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		email:   email,
		token:   token,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *JiraClient) doRequest(method, path string) ([]byte, error) {
	url := c.baseURL + path

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Jira uses Basic Auth with email and API token
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

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
		return nil, fmt.Errorf("Jira API error (status %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

type JiraTool struct {
	secrets *JiraSecrets
}

func (t *JiraTool) getIssue(ctx context.Context, input GetIssueInput) (Issue, error) {
	client := NewJiraClient(t.secrets.Email, t.secrets.Token, t.secrets.BaseURL)

	path := fmt.Sprintf("/rest/api/3/issue/%s", input.IssueKey)

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

func (t *JiraTool) searchIssues(ctx context.Context, input SearchIssuesInput) (SearchIssuesOutput, error) {
	client := NewJiraClient(t.secrets.Email, t.secrets.Token, t.secrets.BaseURL)

	// Set defaults
	if input.MaxResults == 0 {
		input.MaxResults = 50
	}
	if input.MaxResults > 100 {
		input.MaxResults = 100
	}

	// URL encode JQL
	jql := strings.ReplaceAll(input.JQL, " ", "%20")
	path := fmt.Sprintf("/rest/api/3/search?jql=%s&maxResults=%d", jql, input.MaxResults)

	body, err := client.doRequest("GET", path)
	if err != nil {
		return SearchIssuesOutput{}, err
	}

	var result struct {
		Issues     []Issue `json:"issues"`
		Total      int     `json:"total"`
		MaxResults int     `json:"maxResults"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return SearchIssuesOutput{}, fmt.Errorf("failed to parse response: %w", err)
	}

	return SearchIssuesOutput{
		Issues:     result.Issues,
		Total:      result.Total,
		MaxResults: result.MaxResults,
	}, nil
}

func main() {
	secrets := &JiraSecrets{}
	jiraTool := &JiraTool{secrets: secrets}

	// Create get_issue tool
	getIssueTool, err := hive.NewTool(
		"jira_get_issue",
		"Retrieve a specific Jira issue by issue key",
		jiraTool.getIssue,
		hive.WithSecret[GetIssueInput, Issue](secrets),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create get_issue tool: %v\n", err)
		os.Exit(1)
	}

	// Create search_issues tool
	searchTool, err := hive.NewTool(
		"jira_search_issues",
		"Search for Jira issues using JQL query",
		jiraTool.searchIssues,
		hive.WithSecret[SearchIssuesInput, SearchIssuesOutput](secrets),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create search_issues tool: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Jira tools created successfully\n")
	fmt.Fprintf(os.Stderr, "Tools: %s, %s\n", getIssueTool.Name(), searchTool.Name())

	// Serve the get_issue tool by default
	getIssueTool.Serve()
}
