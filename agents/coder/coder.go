package coder

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/gitlab"
	"github.com/hnimtadd/hive/internal/llm"
	"github.com/hnimtadd/hive/internal/redis"
	"github.com/hnimtadd/hive/pkg/types"
)

// AICodeEditorAgent is an AI-powered code editor agent that uses Claude for intelligent development.
type AICodeEditorAgent struct {
	id           string
	agentType    string
	redisClient  *redis.Client
	claudeClient *llm.ClaudeCodeAnalyzer
	gitlabClient *gitlab.GitLabIntegrator
	feedbackCh   agent.FeedbackChannel
	capabilities []string
}

// NewAICodeEditorAgent creates a new AI-powered code editor agent.
func NewAICodeEditorAgent(redisClient *redis.Client) (*AICodeEditorAgent, error) {
	// Initialize Claude client using config
	claudeClient, err := llm.NewClaudeCodeAnalyzer()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Claude client: %w", err)
	}

	// Initialize GitLab client using config
	gitlabClient, err := gitlab.NewGitLabIntegrator("")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GitLab client: %w", err)
	}

	agent := &AICodeEditorAgent{
		id:           "ai-code-editor-" + uuid.New().String()[:8],
		agentType:    "ai_code_editor",
		redisClient:  redisClient,
		claudeClient: claudeClient,
		gitlabClient: gitlabClient,
		capabilities: []string{
			"ai_code_generation",
			"feature_development",
			"gitlab_integration",
			"automated_commits",
			"merge_request_creation",
			"intelligent_analysis",
		},
	}

	return agent, nil
}

// GetID returns the agent's unique identifier.
func (a *AICodeEditorAgent) GetID() string {
	return a.id
}

// GetType returns the agent type.
func (a *AICodeEditorAgent) GetType() string {
	return a.agentType
}

// CanHandle determines if this agent can process the given task.
func (a *AICodeEditorAgent) CanHandle(task *types.HiveTask) bool {
	// Check for AI development keywords
	goal := strings.ToLower(task.Goal)

	aiKeywords := []string{
		"implement", "add", "create", "build", "develop", "feature",
		"function", "method", "class", "component", "service", "api",
		"endpoint", "handler", "middleware", "authentication", "database",
	}

	for _, keyword := range aiKeywords {
		if strings.Contains(goal, keyword) {
			return true
		}
	}

	// Also check if GitLab project ID is provided
	return task.GitLabProjectID > 0
}

// Setup initializes the agent with necessary tools.
func (a *AICodeEditorAgent) Setup(_ context.Context, feedbackCh agent.FeedbackChannel) error {
	a.feedbackCh = feedbackCh
	log.Printf("AI Code Editor Agent %s setup completed", a.id)
	return nil
}

// Execute performs AI-powered feature development.
func (a *AICodeEditorAgent) Execute(ctx context.Context, task *types.HiveTask) error {
	log.Printf("AI Agent %s executing task: %s", a.id, task.ID)

	// Step 1: Mark task as started
	if err := task.MarkStarted(ctx, a.id); err != nil {
		return fmt.Errorf("failed to mark task as started: %w", err)
	}

	// Step 2: AI Analysis Phase (10-30%)
	if err := a.analyzeFeature(ctx, task); err != nil {
		return task.MarkFailed(ctx, fmt.Sprintf("AI analysis failed: %v", err))
	}

	// Step 3: Human feedback if needed (30-40%)
	if len(task.AIQuestions) > 0 {
		if err := a.handleAIQuestions(ctx, task); err != nil {
			return task.MarkFailed(ctx, fmt.Sprintf("Feedback handling failed: %v", err))
		}
	}

	// Step 4: GitLab workspace preparation (40-50%)
	if err := a.prepareWorkspace(ctx, task); err != nil {
		return task.MarkFailed(ctx, fmt.Sprintf("Workspace preparation failed: %v", err))
	}

	// Step 5: AI code generation and implementation (50-90%)
	if err := a.generateAndImplementCode(ctx, task); err != nil {
		return task.MarkFailed(ctx, fmt.Sprintf("Code generation failed: %v", err))
	}

	// Step 6: Create merge request (90-100%)
	if err := a.createMergeRequest(ctx, task); err != nil {
		return task.MarkFailed(ctx, fmt.Sprintf("MR creation failed: %v", err))
	}

	// Complete the task
	summary := a.buildCompletionSummary(task)
	return task.MarkCompleted(ctx, summary)
}

// analyzeFeature uses Claude to analyze the feature request.
func (a *AICodeEditorAgent) analyzeFeature(ctx context.Context, task *types.HiveTask) error {
	task.Progress = 10.0
	task.ExecutionSummary = "AI analyzing feature requirements..."
	_ = a.updateTaskProgress(ctx, task)

	// Get codebase context
	codebaseContext, err := llm.GetCodebaseContext(a.gitlabClient.GetWorkspaceDir())
	if err != nil {
		log.Printf("Warning: Failed to get codebase context: %v", err)
		codebaseContext = "No codebase context available"
	}

	// Analyze feature with Claude
	analysis, err := a.claudeClient.AnalyzeFeature(ctx, task.Goal, codebaseContext)
	if err != nil {
		return err
	}

	// Store analysis results in task
	task.FeatureSpec = analysis.FeatureSummary
	task.TechnicalContext = analysis.TechnicalApproach
	task.FilesToModify = analysis.FilesToModify
	task.FilesToCreate = analysis.FilesToCreate
	task.AIComplexity = analysis.Complexity
	task.AIEstimatedTime = analysis.EstimatedTime
	task.AIQuestions = analysis.Questions

	task.Progress = 30.0
	task.ExecutionSummary = fmt.Sprintf("AI analysis complete - %s complexity, %d files to modify",
		analysis.Complexity, len(analysis.FilesToModify)+len(analysis.FilesToCreate))

	return a.updateTaskProgress(ctx, task)
}

// handleAIQuestions processes questions from AI and gets human feedback.
func (a *AICodeEditorAgent) handleAIQuestions(ctx context.Context, task *types.HiveTask) error {
	questionsText := strings.Join(task.AIQuestions, "\n• ")
	message := fmt.Sprintf("AI needs clarification on the following:\n• %s", questionsText)

	feedback, err := a.RequestFeedback(ctx, task, message)
	if err != nil {
		return err
	}

	// Refine analysis with feedback
	analysis := &llm.AnalysisResult{
		FeatureSummary:    task.FeatureSpec,
		TechnicalApproach: task.TechnicalContext,
		FilesToModify:     task.FilesToModify,
		FilesToCreate:     task.FilesToCreate,
		Questions:         task.AIQuestions,
		Complexity:        task.AIComplexity,
		EstimatedTime:     task.AIEstimatedTime,
	}

	refinedAnalysis, err := a.claudeClient.RefineAnalysis(ctx, analysis, feedback)
	if err != nil {
		return err
	}

	// Update task with refined analysis
	task.FeatureSpec = refinedAnalysis.FeatureSummary
	task.TechnicalContext = refinedAnalysis.TechnicalApproach
	task.FilesToModify = refinedAnalysis.FilesToModify
	task.FilesToCreate = refinedAnalysis.FilesToCreate
	task.AIComplexity = refinedAnalysis.Complexity
	task.AIEstimatedTime = refinedAnalysis.EstimatedTime
	task.AIQuestions = refinedAnalysis.Questions // Should be empty now

	task.Progress = 40.0
	task.ExecutionSummary = "Human feedback incorporated, analysis refined"

	return a.updateTaskProgress(ctx, task)
}

// prepareWorkspace sets up GitLab workspace for development.
func (a *AICodeEditorAgent) prepareWorkspace(ctx context.Context, task *types.HiveTask) error {
	task.Progress = 45.0
	task.ExecutionSummary = "Preparing GitLab workspace..."
	_ = a.updateTaskProgress(ctx, task)

	repoInfo, err := a.gitlabClient.PrepareWorkspace(ctx, task.GitLabProjectID, task.TargetBranch)
	if err != nil {
		return err
	}

	// Update task with repository information
	task.SourceBranch = repoInfo.SourceBranch
	task.WorkingDir = a.gitlabClient.GetWorkspaceDir()

	task.Progress = 50.0
	task.ExecutionSummary = fmt.Sprintf("Workspace ready - branch: %s", repoInfo.SourceBranch)

	return a.updateTaskProgress(ctx, task)
}

// generateAndImplementCode uses Claude to generate code and implements changes.
func (a *AICodeEditorAgent) generateAndImplementCode(ctx context.Context, task *types.HiveTask) error {
	// Re-analyze to get detailed changes
	codebaseContext, _ := llm.GetCodebaseContext(task.WorkingDir)
	detailedAnalysis, err := a.claudeClient.AnalyzeFeature(ctx, task.FeatureSpec, codebaseContext)
	if err != nil {
		return err
	}

	totalChanges := len(detailedAnalysis.Changes)
	progressStep := 40.0 / float64(totalChanges) // 50% to 90%

	for i, change := range detailedAnalysis.Changes {
		task.Progress = 50.0 + float64(i)*progressStep
		task.ExecutionSummary = fmt.Sprintf("Generating code for %s (%d/%d)",
			change.FilePath, i+1, totalChanges)
		_ = a.updateTaskProgress(ctx, task)

		// Generate specific code for this change
		var generatedChange *llm.CodeChange
		generatedChange, err = a.claudeClient.GenerateCode(ctx, &change, codebaseContext)
		if err != nil {
			return fmt.Errorf("failed to generate code for %s: %w", change.FilePath, err)
		}

		// Write code to file
		if err = a.gitlabClient.WriteFile(generatedChange.FilePath, generatedChange.CodeContent); err != nil {
			return fmt.Errorf("failed to write file %s: %w", generatedChange.FilePath, err)
		}

		// Commit this change
		var commitInfo *gitlab.CommitInfo
		commitInfo, err = a.gitlabClient.CommitChanges([]string{generatedChange.FilePath}, generatedChange.CommitMessage)
		if err != nil {
			return fmt.Errorf("failed to commit %s: %w", generatedChange.FilePath, err)
		}

		// Track commits
		task.CommitMessages = append(task.CommitMessages, commitInfo.Message)
		task.CommitSHAs = append(task.CommitSHAs, commitInfo.SHA)
		task.FilesModified = append(task.FilesModified, generatedChange.FilePath)
	}

	task.Progress = 90.0
	task.ExecutionSummary = fmt.Sprintf("Code generation complete - %d commits created", len(task.CommitSHAs))
	task.LinesChanged = a.estimateLinesChanged(detailedAnalysis.Changes)

	return a.updateTaskProgress(ctx, task)
}

// createMergeRequest pushes code and creates MR on GitLab.
func (a *AICodeEditorAgent) createMergeRequest(ctx context.Context, task *types.HiveTask) error {
	task.Progress = 95.0
	task.ExecutionSummary = "Creating merge request..."
	_ = a.updateTaskProgress(ctx, task)

	// Push branch to GitLab
	if err := a.gitlabClient.PushBranch(); err != nil {
		return err
	}

	// Create merge request
	mrTitle := fmt.Sprintf("feat: %s", task.FeatureSpec)
	mrDescription := a.buildMRDescription(task)

	mr, err := a.gitlabClient.CreateMergeRequest(ctx, mrTitle, mrDescription)
	if err != nil {
		return err
	}

	// Update task with MR information
	task.MergeRequestURL = mr.WebURL
	task.MergeRequestID = mr.IID
	task.Progress = 100.0

	return nil
}

// buildMRDescription creates a detailed merge request description.
func (a *AICodeEditorAgent) buildMRDescription(task *types.HiveTask) string {
	return fmt.Sprintf(`## Feature Implementation

**Jira Issue:** %s
**AI Complexity:** %s
**Estimated Time:** %s

## Summary
%s

## Technical Approach
%s

## Changes Made
- **Files Modified:** %d
- **Files Created:** %d
- **Total Commits:** %d
- **Lines Changed:** ~%d

### Modified Files:
%s

### Created Files:
%s

## AI-Generated Implementation
This feature was implemented using AI-powered code generation with human oversight for requirements clarification.

## Ready for Review
- [x] Code generated and committed
- [x] All files properly structured
- [x] Conventional commit messages
- [ ] Code review required
- [ ] Testing recommended

---
*Generated by The Hive AI Agent (%s)*`,
		task.JiraID,
		task.AIComplexity,
		task.AIEstimatedTime,
		task.FeatureSpec,
		task.TechnicalContext,
		len(task.FilesToModify),
		len(task.FilesToCreate),
		len(task.CommitSHAs),
		task.LinesChanged,
		strings.Join(task.FilesToModify, "\n- "),
		strings.Join(task.FilesToCreate, "\n- "),
		a.id)
}

// buildCompletionSummary creates a summary for task completion.
func (a *AICodeEditorAgent) buildCompletionSummary(task *types.HiveTask) string {
	return fmt.Sprintf(`AI-powered feature development completed!

**Merge Request:** %s
**Statistics:**
  - %d commits created
  - %d files modified/created
  - ~%d lines of code generated
  - Complexity: %s

**AI Agent:** %s
**Total Time:** %v

The feature is ready for code review and testing!`,
		task.MergeRequestURL,
		len(task.CommitSHAs),
		len(task.FilesToModify)+len(task.FilesToCreate),
		task.LinesChanged,
		task.AIComplexity,
		a.id,
		task.ExecutionTime)
}

// estimateLinesChanged provides a rough estimate of lines changed.
func (a *AICodeEditorAgent) estimateLinesChanged(changes []llm.CodeChange) int {
	total := 0
	for _, change := range changes {
		// Rough estimation based on content length
		lines := len(strings.Split(change.CodeContent, "\n"))
		if change.ChangeType == "create" {
			total += lines
		} else {
			total += lines / 2 // Assume modifications are ~50% of file
		}
	}
	return total
}

// updateTaskProgress updates task progress in Redis.
func (a *AICodeEditorAgent) updateTaskProgress(ctx context.Context, task *types.HiveTask) error {
	return a.redisClient.UpdateTask(ctx, task)
}

// RequestFeedback implements the HiveAgent interface.
func (a *AICodeEditorAgent) RequestFeedback(ctx context.Context, task *types.HiveTask, message string) (string, error) {
	log.Printf("AI Agent %s requesting feedback for task %s: %s", a.id, task.ID, message)

	// Mark task as paused and requiring feedback
	if err := task.RequestFeedback(ctx, message); err != nil {
		return "", fmt.Errorf("failed to request feedback: %w", err)
	}

	if err := a.redisClient.UpdateTask(ctx, task); err != nil {
		return "", fmt.Errorf("failed to update task: %w", err)
	}

	// Wait for feedback
	feedbackCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	response, err := a.redisClient.WaitForFeedback(feedbackCtx, task.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get feedback: %w", err)
	}

	// Process feedback and resume
	if err = task.ProvideFeedback(ctx, response); err != nil {
		return "", fmt.Errorf("failed to process feedback: %w", err)
	}

	if err = a.redisClient.UpdateTask(ctx, task); err != nil {
		return "", fmt.Errorf("failed to update task after feedback: %w", err)
	}

	log.Printf("AI Agent %s received feedback: %s", a.id, response)
	return response, nil
}

// ReportStatus provides real-time status updates during execution.
func (a *AICodeEditorAgent) ReportStatus(_ context.Context, _ *types.HiveTask) error {
	// Status updates happen in Execute() method via updateTaskProgress
	return nil
}

// Validate performs pre-execution validation of the task.
func (a *AICodeEditorAgent) Validate(task *types.HiveTask) error {
	if task.Goal == "" {
		return errors.New("task goal cannot be empty")
	}

	if task.GitLabProjectID <= 0 {
		return errors.New("GitLab project ID must be specified for AI development tasks")
	}

	if task.TargetBranch == "" {
		task.TargetBranch = "main" // Default to main branch
	}

	return nil
}

// Cleanup performs cleanup after task completion or failure.
func (a *AICodeEditorAgent) Cleanup(_ context.Context, task *types.HiveTask) error {
	log.Printf("AI Agent %s cleaning up after task %s", a.id, task.ID)

	// Cleanup GitLab workspace if needed
	if err := a.gitlabClient.Cleanup(); err != nil {
		log.Printf("Warning: Failed to cleanup GitLab workspace: %v", err)
	}

	return nil
}

// GetCapabilities returns the agent's capabilities.
func (a *AICodeEditorAgent) GetCapabilities() []string {
	return a.capabilities
}

// Heartbeat indicates the agent is alive and ready.
func (a *AICodeEditorAgent) Heartbeat() error {
	ctx := context.Background()
	return a.redisClient.Heartbeat(ctx, a.id)
}
