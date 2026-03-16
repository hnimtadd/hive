package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/hnimtadd/hive/internal/agent/react"
	"github.com/hnimtadd/hive/internal/tools"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/errors"
	"github.com/hnimtadd/hive/pkg/types"

	jira "github.com/andygrunwald/go-jira"
)

// Agent analyzes tasks and prepares them with comprehensive context
type Agent struct {
	id           string
	reactAgent   *react.Agent
	tools        []tool.InvokableTool
	errorHandler *errors.ErrorHandler[*schema.Message]
	capabilities []string
}

// NewAnalyzerAgent creates a new task analysis and preparation agent
// It auto-discovers Jira configuration from the application config.
func NewAnalyzerAgent(chatModel model.ToolCallingChatModel, appConfig *config.Config) (*Agent, error) {
	if chatModel == nil {
		return nil, errors.ErrValidation("chat model is required")
	}

	// Create analysis tools for the agent
	// Start with basic tools always available
	agentTools := []tool.InvokableTool{}

	// Auto-discover and create Jira tool from application config
	// Create Jira tool only if Jira is enabled
	if appConfig != nil && appConfig.Jira.Enabled {
		// Get API token from environment
		apiToken := os.Getenv(appConfig.Jira.APITokenEnv)
		if apiToken == "" {
			return nil, fmt.Errorf("jira API token not found in environment variable %s", appConfig.Jira.APITokenEnv)
		}

		tp := jira.BasicAuthTransport{
			Username: appConfig.Jira.UserName,
			Password: apiToken,
		}
		// Create Jira client and wrap it in a tool
		jiraClient, err := jira.NewClient(tp.Client(), appConfig.Jira.BaseURL)
		if err != nil {
			return nil, errors.ErrInternal("failed to create jira client", err)
		}

		agentTools = append(agentTools, tools.NewJiraTool(jiraClient, appConfig.Jira.CustomFields))
		log.Printf("Jira integration enabled for analyzer agent: %s\n", appConfig.Jira.BaseURL)
	}

	// Create ReACT agent for task analysis
	agentID := "analyst-" + uuid.New().String()[:8]
	maxStep := 30
	if appConfig != nil {
		maxStep = appConfig.AI.MaxStep
	}
	reactAgent, err := react.NewWithSystemPrompt(
		agentID,
		chatModel,
		agentTools,
		getAnalyzerSystemPrompt(),
		maxStep,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ReACT agent: %w", err)
	}

	return &Agent{
		id:           agentID,
		reactAgent:   reactAgent,
		tools:        agentTools,
		errorHandler: errors.NewErrorHandler[*schema.Message](),
		capabilities: []string{
			"task_analysis",
			"requirement_parsing",
			"jira_integration",
			"context_enrichment",
			"complexity_assessment",
			"prerequisite_identification",
		},
	}, nil
}

// getAnalyzerSystemPrompt returns the specialized system prompt for task analysis
func getAnalyzerSystemPrompt() string {
	return `You are a Analyze Task Preparation Specialist. Your job is to enrich tasks with comprehensive technical context that the executing agent needs.

Your primary responsibilities:
1. **Extract Technical Requirements**: Pull out WHAT needs to be built
2. **Preserve Original Context**: Include the full problem description and technical details
3. **Identify Dependencies**: What the coder needs to know before starting
4. **Route Intelligently**: Suggest the right agent type

Available tools:
- think: Record your analysis and reasoning
- read_file: Examine existing code or documentation
- analyze_task: Perform deep analysis
- jira_fetch_ticket: Fetch Jira ticket information (if available)

**IMPORTANT - When Using Jira:**
If the task has a Jira ID, use jira_fetch_ticket to get the full ticket data.
The description may be in Atlassian Document Format (nested JSON). Extract the text content carefully.

**CRITICAL - Output Requirements:**

Your analysis MUST include these sections for the executing agent:

## 1. ORIGINAL PROBLEM DESCRIPTION
[Copy the full problem description from Jira or task details.
Include ALL technical context, requirements, and background.
The coder needs to understand WHY this task exists and WHAT problem it solves.]

## 2. TECHNICAL REQUIREMENTS
[What specifically needs to be implemented?
What is the expected behavior?
What are the acceptance criteria?
Be explicit and detailed.]

## 3. AFFECTED COMPONENTS
[Which services, files, or systems are involved?
List them explicitly - the coder needs to know where to work.
Example: "9 services need migration: config-scanner-api, workflow-automation, ..."]

## 4. IMPLEMENTATION APPROACH
[If the ticket/task mentions HOW to implement, include it here.
Any technical guidelines, patterns to follow, or constraints.
If there are code examples or architecture diagrams mentioned, note them.]

## 5. COMPLEXITY & ROUTING
[Brief assessment:
- Complexity: simple/moderate/complex (one line)
- Required Skills: list 3-5 key skills
- Suggested Agent: enhanced_coder/analyst/integrator/etc.
Keep this SHORT - the content above is what matters.]

**WHAT TO AVOID:**
- Meta-analysis about commit history or what's already done
- Detailed breakdowns of "remaining work" or effort estimates
- Process-focused content (code review steps, deployment plans, testing procedures)
- Status tracking details (who's assigned, current status)
- Long lists of generic risks or prerequisites

**WHAT TO FOCUS ON:**
- The actual technical problem description
- What needs to be built and how it should work
- Where to build it (files, services, components)
- Why it's needed (context and motivation)
- Specific requirements and acceptance criteria
- Technical constraints or guidelines

Remember: The coder agent receiving this needs TECHNICAL CONTENT to implement the solution, not project management meta-analysis.`
}

// AnalyzeTask performs comprehensive task analysis
// The agent will use available tools (including Jira tool if available) to enrich the analysis.
// Returns the raw analysis text which the server can use for routing decisions.
func (a *Agent) AnalyzeTask(ctx context.Context, task *types.HiveTask) (string, error) {
	// Build analysis prompt with all available context
	analysisPrompt := a.buildAnalysisPrompt(task)
	log.Println(analysisPrompt)

	// Use the ReACT agent to analyze the task
	// The agent will automatically use the jira_fetch_ticket tool if task has JiraID
	result, err := a.reactAgent.Execute(ctx, analysisPrompt)
	if err != nil {
		return "", fmt.Errorf("task analysis failed: %w", err)
	}
	log.Println(result)

	// Return raw analysis text
	// The Hive server will read this to decide which agent to route to
	return fmt.Sprintf("%v", result), nil
}

// buildAnalysisPrompt creates an analysis prompt with task context
func (a *Agent) buildAnalysisPrompt(task *types.HiveTask) string {
	// Get the focused task detail that agents care about
	taskDetailJSON, _ := a.detail(task)

	prompt := fmt.Sprintf(`
Analyze this task comprehensively.

TASK DETAILS:
%s`, taskDetailJSON)

	// Add additional guidance if Jira ticket is present
	if task.JiraID != "" {
		prompt += fmt.Sprintf(`

IMPORTANT: This task is associated with Jira ticket: %s
Please use the jira_fetch_ticket tool to get comprehensive information about this ticket.
The Jira data will provide crucial context for your analysis.`, task.JiraID)
	}

	prompt += `

Provide a comprehensive analysis that will help route this task to the right agent and give them the context they need to succeed.`

	return prompt
}

// Interface implementation
func (a *Agent) GetID() string {
	return a.id
}

func (a *Agent) GetType() string {
	return "analyst"
}

func (a *Agent) CanHandle(task *types.HiveTask) bool {
	if task == nil {
		return false
	}

	// Analyzer agent can handle any task for analysis
	// It will analyze and route to appropriate agents
	return true
}

func (a *Agent) Execute(ctx context.Context, task *types.HiveTask) error {
	// Analyze task and get comprehensive analysis text
	analysisText, err := a.AnalyzeTask(ctx, task)
	if err != nil {
		return fmt.Errorf("task analysis failed: %w", err)
	}

	// Store the analysis in the task's TechnicalContext
	// The Hive server will read this to decide which agent to route to
	if task.TechnicalContext == "" {
		task.TechnicalContext = analysisText
	} else {
		task.TechnicalContext = fmt.Sprintf("%s\n\n=== ANALYZER AGENT ANALYSIS ===\n%s", task.TechnicalContext, analysisText)
	}

	return nil // Task preparation complete
}

func (a *Agent) Validate(task *types.HiveTask) error {
	if task == nil {
		return errors.ErrValidation("task is required")
	}
	if strings.TrimSpace(task.Goal) == "" {
		return errors.ErrValidation("task goal cannot be empty")
	}
	return nil
}

// Description return a self-description about agent capabilities.
func (a *Agent) Description() string {
	return ""
}

// TaskDetail represents the essential information that the analyzer Agent needs
// to analyze and prepare a task for execution
type TaskDetail struct {
	// Core task information
	Goal        string `json:"goal"`
	Description string `json:"description"`
	JiraID      string `json:"jira_id,omitempty"`

	// Existing context
	Context          string `json:"context,omitempty"`
	TechnicalContext string `json:"technical_context,omitempty"`
	FeatureSpec      string `json:"feature_spec,omitempty"`

	// Work scope
	FilesToModify []string `json:"files_to_modify,omitempty"`
	FilesToCreate []string `json:"files_to_create,omitempty"`
	WorkingDir    string   `json:"working_dir,omitempty"`

	// Priority context
	Priority string `json:"priority"`

	// Environment (for context about runtime)
	Environment map[string]string `json:"environment,omitempty"`
}

// Detail returns a focused JSON object containing only the information
// that agents (particularly the analyzer Agent) need for task analysis.
// This excludes internal tracking fields, progress metrics, and timestamps.
func (a *Agent) detail(t *types.HiveTask) (string, error) {
	detail := &TaskDetail{
		Goal:             t.Goal,
		Description:      t.Description,
		JiraID:           t.JiraID,
		Context:          t.Context,
		TechnicalContext: t.TechnicalContext,
		FeatureSpec:      t.FeatureSpec,
		FilesToModify:    t.FilesToModify,
		FilesToCreate:    t.FilesToCreate,
		WorkingDir:       t.WorkingDir,
		Priority:         string(t.Priority),
		Environment:      t.Environment,
	}

	// Convert to map[string]interface{} for flexible JSON representation

	data, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
