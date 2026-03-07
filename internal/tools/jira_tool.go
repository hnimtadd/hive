package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	jira "github.com/andygrunwald/go-jira"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// JiraTool provides access to Jira ticket information
type JiraTool struct {
	client       *jira.Client
	customFields map[string]string // Map of field ID to friendly name
}

// NewJiraTool creates a new Jira tool with the provided client
// customFields is an optional map of custom field IDs to friendly names
func NewJiraTool(client *jira.Client, customFields ...map[string]string) tool.InvokableTool {
	fields := make(map[string]string)
	if len(customFields) > 0 && customFields[0] != nil {
		fields = customFields[0]
	}
	// Default custom field if none provided
	if len(fields) == 0 {
		fields["customfield_10206"] = "Implementation Guide & Definition of Done"
	}
	return &JiraTool{
		client:       client,
		customFields: fields,
	}
}

// Info returns tool information
func (t *JiraTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "jira_fetch_ticket",
		Desc: "Fetch comprehensive information from a Jira ticket including summary, description, status, " +
			"priority, sub-tasks, linked issues (dependencies), attachments, time tracking, comments, " +
			"and all other relevant data. Use this when you need detailed context about a Jira ticket.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"ticket_key": {
				Type:     schema.String,
				Desc:     "The Jira ticket key (e.g., 'T6-1301', 'PROJ-123')",
				Required: true,
			},
		}),
	}, nil
}

// InvokableRun executes the tool
func (t *JiraTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	if t.client == nil {
		return "", fmt.Errorf("jira client not initialized")
	}

	// Parse arguments
	var args struct {
		TicketKey string `json:"ticket_key"`
	}

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if args.TicketKey == "" {
		return "", fmt.Errorf("ticket_key is required")
	}

	// Fetch ticket information
	ticket, _, err := t.client.Issue.Get(args.TicketKey, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Jira ticket %s: %w", args.TicketKey, err)
	}

	// Build a focused response for the agent
	// Focus on CONTENT that helps understand what to build, not metadata
	var response strings.Builder

	response.WriteString(fmt.Sprintf("=== JIRA TICKET %s ===\n\n", ticket.Key))
	response.WriteString(fmt.Sprintf("Summary: %s\n\n", ticket.Fields.Summary))

	// Description (already processed by go-jira client as plain text)
	if ticket.Fields.Description != "" {
		response.WriteString("Description:\n")
		response.WriteString(ticket.Fields.Description)
		response.WriteString("\n\n")
	}

	// Custom fields - extract important ones
	// Try to extract "What to do" / "Definition of Done" field (customfield_10206 in your Jira)
	customFields := t.extractImportantCustomFields(ticket)
	if len(customFields) > 0 {
		response.WriteString("Additional Context:\n")
		for fieldName, fieldValue := range customFields {
			response.WriteString(fmt.Sprintf("\n--- %s ---\n", fieldName))
			response.WriteString(fieldValue)
			response.WriteString("\n")
		}
		response.WriteString("\n")
	}

	// Basic metadata (keep it minimal)
	response.WriteString("Metadata:\n")
	if ticket.Fields.Type.Name != "" {
		response.WriteString(fmt.Sprintf("- Type: %s\n", ticket.Fields.Type.Name))
	}
	if ticket.Fields.Status.Name != "" {
		response.WriteString(fmt.Sprintf("- Status: %s\n", ticket.Fields.Status.Name))
	}
	if ticket.Fields.Priority.Name != "" {
		response.WriteString(fmt.Sprintf("- Priority: %s\n", ticket.Fields.Priority.Name))
	}
	if ticket.Fields.Assignee != nil && ticket.Fields.Assignee.DisplayName != "" {
		response.WriteString(fmt.Sprintf("- Assignee: %s\n", ticket.Fields.Assignee.DisplayName))
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
		response.WriteString(fmt.Sprintf("Labels: %s\n\n", strings.Join(ticket.Fields.Labels, ", ")))
	}

	// Sub-tasks (if any)
	if len(ticket.Fields.Subtasks) > 0 {
		response.WriteString(fmt.Sprintf("Sub-tasks (%d):\n", len(ticket.Fields.Subtasks)))
		for i, subtask := range ticket.Fields.Subtasks {
			if i >= 10 { // Limit to first 10
				response.WriteString(fmt.Sprintf("... and %d more sub-tasks\n", len(ticket.Fields.Subtasks)-10))
				break
			}
			status := "Unknown"
			if subtask.Fields.Status.Name != "" {
				status = subtask.Fields.Status.Name
			}
			response.WriteString(fmt.Sprintf("- %s: %s [%s]\n", subtask.Key, subtask.Fields.Summary, status))
		}
		response.WriteString("\n")
	}

	// Linked issues (if any)
	if len(ticket.Fields.IssueLinks) > 0 {
		response.WriteString(fmt.Sprintf("Linked Issues (%d):\n", len(ticket.Fields.IssueLinks)))
		for i, link := range ticket.Fields.IssueLinks {
			if i >= 10 { // Limit to first 10
				response.WriteString(fmt.Sprintf("... and %d more linked issues\n", len(ticket.Fields.IssueLinks)-10))
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
			response.WriteString(fmt.Sprintf("- %s: %s\n", linkType, linkedKey))
		}
		response.WriteString("\n")
	}

	// Attachments (if any)
	if len(ticket.Fields.Attachments) > 0 {
		response.WriteString(fmt.Sprintf("Attachments (%d):\n", len(ticket.Fields.Attachments)))
		for i, attachment := range ticket.Fields.Attachments {
			if i >= 5 { // Limit to first 5
				response.WriteString(fmt.Sprintf("... and %d more attachments\n", len(ticket.Fields.Attachments)-5))
				break
			}
			response.WriteString(fmt.Sprintf("- %s (%s, %d bytes)\n", attachment.Filename, attachment.MimeType, attachment.Size))
		}
		response.WriteString("\n")
	}

	// Recent comments (most important - might contain decisions and clarifications)
	if ticket.Fields.Comments != nil && len(ticket.Fields.Comments.Comments) > 0 {
		response.WriteString(fmt.Sprintf("Comments (%d total, showing most recent 5):\n", len(ticket.Fields.Comments.Comments)))

		// Show last 5 comments (most recent)
		startIdx := len(ticket.Fields.Comments.Comments) - 5
		if startIdx < 0 {
			startIdx = 0
		}

		for i := startIdx; i < len(ticket.Fields.Comments.Comments); i++ {
			comment := ticket.Fields.Comments.Comments[i]
			author := "Unknown"
			if comment.Author.DisplayName != "" {
				author = comment.Author.DisplayName
			}
			response.WriteString(fmt.Sprintf("\n--- Comment by %s ---\n", author))
			response.WriteString(comment.Body) // go-jira already processes this as plain text
			response.WriteString("\n")
		}
		response.WriteString("\n")
	}

	response.WriteString("=== END OF JIRA TICKET ===\n")

	return response.String(), nil
}

// extractImportantCustomFields extracts configured custom fields
// This includes fields like "What to do", "Definition of Done", "Acceptance Criteria", etc.
func (t *JiraTool) extractImportantCustomFields(ticket *jira.Issue) map[string]string {
	result := make(map[string]string)

	// Access the raw unknowns map from jira.Issue
	if ticket.Fields.Unknowns != nil {
		for fieldID, fieldName := range t.customFields {
			if value, exists := ticket.Fields.Unknowns[fieldID]; exists && value != nil {
				// Convert to string
				strValue := ""
				switch v := value.(type) {
				case string:
					strValue = v
				case map[string]interface{}, []interface{}:
					// Skip complex objects for now
					continue
				default:
					strValue = fmt.Sprintf("%v", v)
				}

				// Only include if not empty
				if strings.TrimSpace(strValue) != "" {
					result[fieldName] = strValue
				}
			}
		}
	}

	return result
}
