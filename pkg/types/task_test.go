package types_test

import (
	"encoding/json"
	"testing"

	"github.com/hnimtadd/hive/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestHiveTask_Detail(t *testing.T) {
	// Create a sample task that the Grab Agent would analyze
	task := types.NewHiveTask("Implement user authentication", "T6-1301")
	task.Description = "Add JWT-based authentication with rate limiting"
	task.Context = "User management system needs secure authentication"
	task.TechnicalContext = "Working on Go microservice. Need to integrate with existing user management system."
	task.FeatureSpec = "Must support refresh tokens, rate limiting, and audit logging"
	task.Priority = types.TaskPriorityHigh
	task.WorkingDir = "/path/to/project"
	task.FilesToModify = []string{"auth.go", "middleware.go"}
	task.FilesToCreate = []string{"jwt.go", "rate_limiter.go"}
	task.Environment = map[string]string{
		"GO_ENV": "development",
	}

	// Get task detail (what Grab Agent sees)
	detail := task.Detail()

	// Verify only relevant fields are present
	assert.NotNil(t, detail)
	assert.Equal(t, "Implement user authentication", detail["goal"])
	assert.Equal(t, "T6-1301", detail["jira_id"])
	assert.Equal(t, "Add JWT-based authentication with rate limiting", detail["description"])
	assert.Equal(t, "high", detail["priority"])
	assert.Contains(t, detail["technical_context"], "Go microservice")
	assert.Equal(t, "/path/to/project", detail["working_dir"])

	// Verify files arrays
	filesToModify := detail["files_to_modify"].([]interface{})
	assert.Len(t, filesToModify, 2)
	filesToCreate := detail["files_to_create"].([]interface{})
	assert.Len(t, filesToCreate, 2)

	// Verify environment
	env := detail["environment"].(map[string]interface{})
	assert.Equal(t, "development", env["GO_ENV"])

	// Verify tracking fields are NOT present (Grab Agent doesn't need these)
	assert.Nil(t, detail["id"])
	assert.Nil(t, detail["status"])
	assert.Nil(t, detail["progress"])
	assert.Nil(t, detail["assigned_agent"])
	assert.Nil(t, detail["created_at"])
	assert.Nil(t, detail["updated_at"])
	assert.Nil(t, detail["retry_count"])
	assert.Nil(t, detail["tests_passed"])
	assert.Nil(t, detail["lines_changed"])
	assert.Nil(t, detail["gitlab_project_id"])
}

func TestHiveTask_DetailJSON(t *testing.T) {
	// Create a sample task
	task := types.NewHiveTask("Fix bug in payment processing", "T6-2001")
	task.Description = "Payment gateway timeout issue under high load"
	task.Context = "Production issue affecting customer transactions"
	task.Priority = types.TaskPriorityCritical
	task.WorkingDir = "/services/payment"
	task.FilesToModify = []string{"payment_handler.go", "timeout_config.go"}

	// Get JSON representation
	jsonStr, err := task.DetailJSON()
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonStr)

	// Verify it's valid JSON
	var result map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &result)
	assert.NoError(t, err)

	// Verify key fields that Grab Agent needs
	assert.Equal(t, "Fix bug in payment processing", result["goal"])
	assert.Equal(t, "T6-2001", result["jira_id"])
	assert.Equal(t, "critical", result["priority"])
	assert.Equal(t, "/services/payment", result["working_dir"])

	// Verify internal tracking fields are NOT in JSON
	assert.Nil(t, result["status"])
	assert.Nil(t, result["progress"])
	assert.Nil(t, result["assigned_agent"])
}

func TestHiveTask_DetailMinimal(t *testing.T) {
	// Test with minimal task (only required fields)
	task := types.NewHiveTask("Simple refactoring task", "")
	task.Description = "Clean up unused imports"

	detail := task.Detail()

	// Should still have basic fields
	assert.Equal(t, "Simple refactoring task", detail["goal"])
	assert.Equal(t, "Clean up unused imports", detail["description"])
	assert.Equal(t, "medium", detail["priority"]) // Default priority

	// JiraID should not be present if empty
	_, hasJiraID := detail["jira_id"]
	assert.False(t, hasJiraID)
}

func TestHiveTask_DetailWithFullContext(t *testing.T) {
	// Create a task with rich context
	task := types.NewHiveTask("Implement new API endpoint", "T6-5000")
	task.Description = "Create REST endpoint for user profile management"
	task.Context = "Part of user management epic"
	task.TechnicalContext = "RESTful API using Gin framework, PostgreSQL database"
	task.FeatureSpec = `
API Specification:
- GET /api/v1/users/:id/profile
- PUT /api/v1/users/:id/profile
- Must include authentication
- Rate limit: 100 req/min
`
	task.Priority = types.TaskPriorityHigh
	task.WorkingDir = "/api/services"
	task.FilesToCreate = []string{"profile_handler.go", "profile_service.go", "profile_repository.go"}
	task.Environment = map[string]string{
		"DATABASE_URL": "postgres://localhost:5432/users",
		"API_VERSION":  "v1",
	}

	// Get detail
	detail := task.Detail()

	// All context fields should be present
	assert.Equal(t, "Implement new API endpoint", detail["goal"])
	assert.Equal(t, "T6-5000", detail["jira_id"])
	assert.Contains(t, detail["context"], "user management epic")
	assert.Contains(t, detail["technical_context"], "Gin framework")
	assert.Contains(t, detail["feature_spec"], "authentication")
	assert.Equal(t, "high", detail["priority"])
	assert.Equal(t, "/api/services", detail["working_dir"])

	filesToCreate := detail["files_to_create"].([]interface{})
	assert.Len(t, filesToCreate, 3)

	env := detail["environment"].(map[string]interface{})
	assert.Equal(t, "v1", env["API_VERSION"])
}

func TestHiveTask_DetailJSONFormatting(t *testing.T) {
	// Test that JSON is properly formatted (indented)
	task := types.NewHiveTask("Test task", "T6-9999")
	task.Description = "Testing JSON formatting"

	jsonStr, err := task.DetailJSON()
	assert.NoError(t, err)

	// Should be indented (contains newlines and spaces)
	assert.Contains(t, jsonStr, "\n")
	assert.Contains(t, jsonStr, "  ")

	// Should be parseable
	var result map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &result)
	assert.NoError(t, err)
}
