package main //nolint:cyclop// this is acceptable

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/andygrunwald/go-jira"
)

// JiraTool provides access to Jira ticket information.
type JiraTool struct {
	client *jira.Client
}

// NewJiraTool creates a new Jira tool with the provided client
// customFields is an optional map of custom field IDs to friendly names.
func NewJiraTool(client *jira.Client) *JiraTool {
	// Default custom field if none provided
	return &JiraTool{
		client: client,
	}
}

// Info returns tool information
// func (t *JiraTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
// 	return &schema.ToolInfo{
// 		Name: "jira_fetch_ticket",
// 		Desc: "Fetch comprehensive information from a Jira ticket including summary, description, status, " +
// 			"priority, sub-tasks, linked issues (dependencies), attachments, time tracking, comments, " +
// 			"and all other relevant data. Use this when you need detailed context about a Jira ticket.",
// 		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
// 			"ticket_key": {
// 				Type:     schema.String,
// 				Desc:     "The Jira ticket key (e.g., 'T6-1301', 'PROJ-123')",
// 				Required: true,
// 			},
// 		}),
// 	}, nil
// }

// InvokableRun executes the tool.
func (t *JiraTool) InvokableRun(argumentsInJSON string) (string, error) { //nolint:gocognit,cyclop,gocyclo,funlen // this is acceptable
	if t.client == nil {
		return "", errors.New("jira client not initialized")
	}

	// Parse arguments
	var args struct {
		TicketKey string `json:"ticket_key"`
	}

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		log.Printf("failed to parse arguments: %s, args: %s\n", err, argumentsInJSON)
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if args.TicketKey == "" {
		log.Printf("ticket key is empty")
		return "", errors.New("ticket_key is required")
	}

	// Fetch ticket information
	ticket, _, err := t.client.Issue.Get(args.TicketKey, nil)
	if err != nil {
		log.Printf("failed to fetch Jira ticket: %s", err)
		return "", fmt.Errorf("failed to fetch Jira ticket %s: %w", args.TicketKey, err)
	}

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

	return response.String(), nil
}

func main() {
	baseURL := os.Getenv("JIRA_BASE_URL")
	userName := os.Getenv("JIRA_USERNAME")
	accessToken := os.Getenv("JIRA_ACCESS_TOKEN")
	tp := jira.BasicAuthTransport{
		Username: userName,
		Password: accessToken,
	}
	jiraClient, err := jira.NewClient(tp.Client(), baseURL)
	if err != nil {
		fmt.Fprintf(os.Stdout, "failed to init jira client: %s", err)
		return
	}
	tool := NewJiraTool(jiraClient)

	stdinBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stdout, "failed to read input: %s", err)
		return
	}
	output, err := tool.InvokableRun(string(stdinBytes))
	if err != nil {
		fmt.Fprintf(os.Stdout, "failed to invoke tool: %s", err)
		return
	}
	fmt.Fprint(os.Stdout, output)
	fmt.Fprintln(os.Stderr, "success")
}
