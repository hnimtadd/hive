package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/andygrunwald/go-jira"
	"github.com/hnimtadd/hive/pkg/hive"
)

type JiraSecrets struct {
	Username string `hive:"key=JIRA_USERNAME;description=Jira account email;required"`
	Token    string `hive:"key=JIRA_ACCESS_TOKEN;description=Jira API token;required"`
	BaseURL  string `hive:"key=JIRA_BASE_URL;description=Jira instance URL (e.g. https://yourcompany.atlassian.net);required"`
}

// GetIssueInput retrieves a specific issue.
type GetIssueInput struct {
	IssueKey string `json:"issue_key" jsonschema:"description=Issue key (e.g. 'PROJ-123')"`
}

type Issue struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

func NewJiraClient(email, token, baseURL string) (*jira.Client, error) {
	tp := jira.BasicAuthTransport{
		Username: email,
		Password: token,
	}
	return jira.NewClient(tp.Client(), baseURL)
}

type JiraTool struct {
	secrets *JiraSecrets
}

func (t *JiraTool) getIssue(_ context.Context, input GetIssueInput) (Issue, error) {
	client, err := NewJiraClient(t.secrets.Username, t.secrets.Token, t.secrets.BaseURL)
	if err != nil {
		return Issue{}, fmt.Errorf("failed to create jira client: %w", err)
	}
	issue, _, err := client.Issue.Get(input.IssueKey, nil)
	if err != nil {
		return Issue{}, fmt.Errorf("failed to to fetch Jira ticket %s: %w", input.IssueKey, err)
	}

	return Issue{
		Key:     input.IssueKey,
		Content: buildResponse(issue),
	}, nil
}

func buildResponse(ticket *jira.Issue) string {
	// Build a focused response for the agent
	// Focus on CONTENT that helps understand what to build, not metadata
	var response strings.Builder

	fmt.Fprintf(&response, "=== JIRA TICKET %s ===\n\n", ticket.Key)
	fmt.Fprintf(&response, "Summary: %s\n\n", ticket.Fields.Summary)

	// Description (already processed by go-jira client as plain text)
	if ticket.Fields.Description != "" {
		response.WriteString("Description:\n")
		response.WriteString(ticket.Fields.Description)
		response.WriteString("\n\n")
	}

	// Basic metadata (keep it minimal)
	response.WriteString("Metadata:\n")
	if ticket.Fields.Type.Name != "" {
		fmt.Fprintf(&response, "- Type: %s\n", ticket.Fields.Type.Name)
	}
	if ticket.Fields.Status.Name != "" {
		fmt.Fprintf(&response, "- Status: %s\n", ticket.Fields.Status.Name)
	}
	if ticket.Fields.Priority.Name != "" {
		fmt.Fprintf(&response, "- Priority: %s\n", ticket.Fields.Priority.Name)
	}
	if ticket.Fields.Assignee != nil && ticket.Fields.Assignee.DisplayName != "" {
		fmt.Fprintf(&response, "- Assignee: %s\n", ticket.Fields.Assignee.DisplayName)
	}
	response.WriteString("\n")

	// Components and Labels (if relevant)
	if len(ticket.Fields.Components) > 0 {
		response.WriteString("Components: ")
		compNames := []string{}
		for _, comp := range ticket.Fields.Components {
			compNames = append(compNames, comp.Name)
		}
		response.WriteString(strings.Join(compNames, ", "))
		response.WriteString("\n\n")
	}

	if len(ticket.Fields.Labels) > 0 {
		fmt.Fprintf(&response, "Labels: %s\n\n", strings.Join(ticket.Fields.Labels, ", "))
	}

	// Sub-tasks (if any)
	if len(ticket.Fields.Subtasks) > 0 {
		fmt.Fprintf(&response, "Sub-tasks (%d):\n", len(ticket.Fields.Subtasks))
		for i, subtask := range ticket.Fields.Subtasks {
			if i >= 10 { // Limit to first 10
				fmt.Fprintf(&response, "... and %d more sub-tasks\n", len(ticket.Fields.Subtasks)-10)
				break
			}
			status := "Unknown"
			if subtask.Fields.Status.Name != "" {
				status = subtask.Fields.Status.Name
			}
			fmt.Fprintf(&response, "- %s: %s [%s]\n", subtask.Key, subtask.Fields.Summary, status)
		}
		response.WriteString("\n")
	}

	// Linked issues (if any)
	if len(ticket.Fields.IssueLinks) > 0 {
		fmt.Fprintf(&response, "Linked Issues (%d):\n", len(ticket.Fields.IssueLinks))
		for i, link := range ticket.Fields.IssueLinks {
			if i >= 10 { // Limit to first 10
				fmt.Fprintf(&response, "... and %d more linked issues\n", len(ticket.Fields.IssueLinks)-10)
				break
			}
			linkType := ""
			linkedKey := ""
			if link.Type.Name != "" {
				linkType = link.Type.Name
			}
			if link.OutwardIssue != nil {
				linkedKey = link.OutwardIssue.Key
			} else if link.InwardIssue != nil {
				linkedKey = link.InwardIssue.Key
			}
			fmt.Fprintf(&response, "- %s: %s\n", linkType, linkedKey)
		}
		response.WriteString("\n")
	}

	// Attachments (if any)
	if len(ticket.Fields.Attachments) > 0 {
		fmt.Fprintf(&response, "Attachments (%d):\n", len(ticket.Fields.Attachments))
		for i, attachment := range ticket.Fields.Attachments {
			if i >= 5 { // Limit to first 5
				fmt.Fprintf(&response, "... and %d more attachments\n", len(ticket.Fields.Attachments)-5)
				break
			}
			fmt.Fprintf(&response, "- %s (%s, %d bytes)\n", attachment.Filename, attachment.MimeType, attachment.Size)
		}
		response.WriteString("\n")
	}

	// Recent comments (most important - might contain decisions and clarifications)
	if ticket.Fields.Comments != nil && len(ticket.Fields.Comments.Comments) > 0 {
		fmt.Fprintf(&response, "Comments (%d total, showing most recent 5):\n", len(ticket.Fields.Comments.Comments))

		// Show last 5 comments (most recent)
		startIdx := max(len(ticket.Fields.Comments.Comments)-5, 0)

		for i := startIdx; i < len(ticket.Fields.Comments.Comments); i++ {
			comment := ticket.Fields.Comments.Comments[i]
			author := "Unknown"
			if comment.Author.DisplayName != "" {
				author = comment.Author.DisplayName
			}
			fmt.Fprintf(&response, "\n--- Comment by %s ---\n", author)
			response.WriteString(comment.Body) // go-jira already processes this as plain text
			response.WriteString("\n")
		}
		response.WriteString("\n")
	}

	response.WriteString("=== END OF JIRA TICKET ===\n")
	return response.String()
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

	fmt.Fprintf(os.Stderr, "Jira tools created successfully\n")
	fmt.Fprintf(os.Stderr, "Tools: %s\n", getIssueTool.Name())
	// Serve the get_issue tool by default
	getIssueTool.Serve()
}
