package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/pkg/config"
)

// AnalysisResult represents the AI's analysis of a feature request.
type AnalysisResult struct {
	// Feature understanding
	FeatureSummary    string `json:"feature_summary"`
	TechnicalApproach string `json:"technical_approach"`

	// Implementation plan
	FilesToModify []string `json:"files_to_modify"`
	FilesToCreate []string `json:"files_to_create"`

	// Code changes
	Changes []CodeChange `json:"changes"`

	// Clarification questions
	Questions []string `json:"questions"`

	// Merge request details
	MRTitle       string `json:"mr_title"`
	MRDescription string `json:"mr_description"`

	// Estimated complexity
	Complexity    string `json:"complexity"` // "low", "medium", "high"
	EstimatedTime string `json:"estimated_time"`
}

// CodeChange represents a single code modification.
type CodeChange struct {
	FilePath      string   `json:"file_path"`
	ChangeType    string   `json:"change_type"` // "create", "modify", "delete"
	Description   string   `json:"description"`
	CodeContent   string   `json:"code_content"`
	CommitMessage string   `json:"commit_message"`
	Dependencies  []string `json:"dependencies"` // Other files this change depends on
}

// ClaudeCodeAnalyzer handles AI-powered code analysis and generation.
type ClaudeCodeAnalyzer struct {
	model *claude.ChatModel
}

// NewClaudeCodeAnalyzer creates a new Claude-powered code analyzer.
func NewClaudeCodeAnalyzer() (*ClaudeCodeAnalyzer, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return NewClaudeCodeAnalyzerWithConfig(&cfg.AI)
}

// NewClaudeCodeAnalyzerWithConfig creates a new Claude-powered code analyzer with provided config.
func NewClaudeCodeAnalyzerWithConfig(cfg *config.AIConfig) (*ClaudeCodeAnalyzer, error) {
	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is required", cfg.APIKeyEnv)
	}

	claudeConfig := &claude.Config{
		APIKey: apiKey,
		Model:  cfg.Model,
	}

	if cfg.BaseURL != "" {
		baseURL := cfg.BaseURL
		claudeConfig.BaseURL = &baseURL
	}

	model, err := claude.NewChatModel(context.Background(), claudeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Claude model: %w", err)
	}

	return &ClaudeCodeAnalyzer{model: model}, nil
}

// AnalyzeFeature analyzes a feature request and provides implementation guidance.
func (c *ClaudeCodeAnalyzer) AnalyzeFeature(ctx context.Context, featureSpec, codebaseContext string) (*AnalysisResult, error) {
	prompt := c.buildAnalysisPrompt(featureSpec, codebaseContext)

	response, err := c.model.Generate(ctx, []*schema.Message{{
		Role:    schema.User,
		Content: prompt,
	}})
	if err != nil {
		return nil, fmt.Errorf("claude analysis failed: %w", err)
	}

	return c.parseAnalysisResult(response.Content)
}

// RefineAnalysis refines the analysis based on human feedback.
func (c *ClaudeCodeAnalyzer) RefineAnalysis(ctx context.Context, analysis *AnalysisResult, feedback string) (*AnalysisResult, error) {
	prompt := c.buildRefinementPrompt(analysis, feedback)

	response, err := c.model.Generate(ctx, []*schema.Message{{
		Role:    schema.User,
		Content: prompt,
	}})
	if err != nil {
		return nil, fmt.Errorf("claude refinement failed: %w", err)
	}

	return c.parseAnalysisResult(response.Content)
}

// GenerateCode generates specific code for a given change.
func (c *ClaudeCodeAnalyzer) GenerateCode(ctx context.Context, change *CodeChange, context string) (*CodeChange, error) {
	prompt := c.buildCodeGenerationPrompt(change, context)

	response, err := c.model.Generate(ctx, []*schema.Message{{
		Role:    schema.User,
		Content: prompt,
	}})
	if err != nil {
		return nil, fmt.Errorf("code generation failed: %w", err)
	}

	// Extract code from response
	code := c.extractCodeFromResponse(response.Content)
	change.CodeContent = code

	return change, nil
}

// buildAnalysisPrompt creates a comprehensive prompt for feature analysis.
func (c *ClaudeCodeAnalyzer) buildAnalysisPrompt(featureSpec, codebaseContext string) string {
	return fmt.Sprintf(`You are a senior software engineer analyzing a feature request for implementation.

FEATURE REQUEST:
%s

CODEBASE CONTEXT:
%s

Please provide a comprehensive analysis in JSON format with the following structure:
{
  "feature_summary": "Brief summary of what needs to be built",
  "technical_approach": "High-level technical approach",
  "files_to_modify": ["list", "of", "existing", "files", "to", "modify"],
  "files_to_create": ["list", "of", "new", "files", "to", "create"],
  "changes": [
    {
      "file_path": "path/to/file.go",
      "change_type": "modify|create|delete",
      "description": "What changes to make in this file",
      "commit_message": "Conventional commit message for this change",
      "dependencies": ["other files this change depends on"]
    }
  ],
  "questions": ["any clarification questions for the human"],
  "mr_title": "Concise MR title following conventional commits",
  "mr_description": "Detailed MR description with implementation notes",
  "complexity": "low|medium|high",
  "estimated_time": "rough time estimate"
}

Focus on:
- Go best practices and idiomatic code
- Proper error handling
- Clear separation of concerns
- Testability
- Security considerations
- Performance implications

If you need clarification on requirements, business logic, or technical decisions, add specific questions to the "questions" array.

Respond with ONLY the JSON, no additional text.`, featureSpec, codebaseContext)
}

// buildRefinementPrompt creates a prompt for refining analysis based on feedback.
func (c *ClaudeCodeAnalyzer) buildRefinementPrompt(analysis *AnalysisResult, feedback string) string {
	analysisJSON, _ := json.MarshalIndent(analysis, "", "  ")

	return fmt.Sprintf(`Based on the previous analysis and human feedback, please refine the implementation plan.

PREVIOUS ANALYSIS:
%s

HUMAN FEEDBACK:
%s

Please provide an updated analysis in the same JSON format, incorporating the feedback and addressing any concerns raised. Make sure to:
- Update implementation details based on feedback
- Resolve any ambiguities mentioned
- Adjust complexity and time estimates if needed
- Clear the questions array if feedback resolved them

Respond with ONLY the updated JSON, no additional text.`, analysisJSON, feedback)
}

// buildCodeGenerationPrompt creates a prompt for generating specific code.
func (c *ClaudeCodeAnalyzer) buildCodeGenerationPrompt(change *CodeChange, context string) string {
	return fmt.Sprintf(`Generate the specific code for the following change:

CHANGE DETAILS:
- File: %s
- Type: %s
- Description: %s

CONTEXT:
%s

Requirements:
- Write idiomatic Go code
- Include proper error handling
- Add appropriate comments for complex logic
- Follow Go naming conventions
- Include necessary imports
- Make code testable and maintainable

For new files, include the complete file content with package declaration and imports.
For modifications, provide the specific code segments to add/modify.

Respond with ONLY the code, no explanations or markdown formatting.`,
		change.FilePath, change.ChangeType, change.Description, context)
}

// parseAnalysisResult parses Claude's JSON response into AnalysisResult.
func (c *ClaudeCodeAnalyzer) parseAnalysisResult(content string) (*AnalysisResult, error) {
	// Clean the response - sometimes Claude includes markdown formatting
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result AnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse Claude response: %w\nContent: %s", err, content)
	}

	return &result, nil
}

// extractCodeFromResponse extracts code content from Claude's response.
func (c *ClaudeCodeAnalyzer) extractCodeFromResponse(content string) string {
	// Remove markdown formatting if present
	content = strings.TrimSpace(content)

	// Handle code blocks
	if strings.Contains(content, "```") {
		lines := strings.Split(content, "\n")
		var codeLines []string
		inCodeBlock := false

		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			if inCodeBlock {
				codeLines = append(codeLines, line)
			}
		}

		if len(codeLines) > 0 {
			return strings.Join(codeLines, "\n")
		}
	}

	return content
}

// GetCodebaseContext analyzes the current codebase to provide context for AI.
func GetCodebaseContext(workspaceDir string) (string, error) {
	// TODO: Implement codebase analysis
	// This could include:
	// - Go module information
	// - Package structure
	// - Key dependencies
	// - Existing patterns and conventions
	// - Recent changes

	context := fmt.Sprintf(`
Codebase Analysis for: %s

Go Module: The Hive - Distributed AI Agent Platform
Architecture: Microservices with Redis message bus
Key Components:
- CLI client (cmd/hive) using Cobra
- Agent workers (cmd/agent)
- Task management (pkg/types)
- Redis integration (internal/redis)
- Agent interfaces (internal/agent)

Tech Stack:
- Go 1.25
- Redis for messaging and queuing
- Eino framework for LLM integration
- GitLab integration for code management
- SQLite for persistence (planned)

Code Style:
- Standard Go formatting with gofmt
- Conventional commit messages
- Interface-driven design
- Error wrapping with fmt.Errorf
- Context-based cancellation
`, workspaceDir)

	return context, nil
}

