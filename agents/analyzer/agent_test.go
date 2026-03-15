package analyzer

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockChatModel is a mock implementation of ToolCallingChatModel
type MockChatModel struct {
	mock.Mock
}

// Generate implements [model.ToolCallingChatModel].
func (m *MockChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	panic("unimplemented")
}

// Stream implements [model.ToolCallingChatModel].
func (m *MockChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	panic("unimplemented")
}

// WithTools implements [model.ToolCallingChatModel].
func (m *MockChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil // Return self for testing
}

var _ model.ToolCallingChatModel = &MockChatModel{
	Mock: mock.Mock{},
}

// MockRegistry is a mock implementation of agent Registry
type MockRegistry struct {
	mock.Mock
}

var _ agent.Registry = &MockRegistry{}

func (m *MockRegistry) RegisterAgent(agent agent.HiveAgent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockRegistry) UnregisterAgent(agentID string) error {
	args := m.Called(agentID)
	return args.Error(0)
}

func (m *MockRegistry) FindAgent(task *types.HiveTask) (agent.HiveAgent, error) {
	args := m.Called(task)
	return args.Get(0).(agent.HiveAgent), args.Error(1)
}

func (m *MockRegistry) ListAgents() []agent.HiveAgent {
	args := m.Called()
	return args.Get(0).([]agent.HiveAgent)
}

func (m *MockRegistry) GetAgent(agentID string) (agent.HiveAgent, error) {
	args := m.Called(agentID)
	return args.Get(0).(agent.HiveAgent), args.Error(1)
}

func (m *MockRegistry) GetAgentsByType(agentType string) []agent.HiveAgent {
	args := m.Called(agentType)
	return args.Get(0).([]agent.HiveAgent)
}

// MockHiveAgent is a mock implementation of HiveAgent
type MockHiveAgent struct {
	mock.Mock
}

func (m *MockHiveAgent) GetID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockHiveAgent) GetType() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockHiveAgent) CanHandle(task *types.HiveTask) bool {
	args := m.Called(task)
	return args.Bool(0)
}

func (m *MockHiveAgent) Execute(ctx context.Context, task *types.HiveTask) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockHiveAgent) ReportStatus(ctx context.Context, task *types.HiveTask) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockHiveAgent) Setup(ctx context.Context, feedbackCh agent.FeedbackChannel) error {
	args := m.Called(ctx, feedbackCh)
	return args.Error(0)
}

func (m *MockHiveAgent) RequestFeedback(ctx context.Context, task *types.HiveTask, message string) (string, error) {
	args := m.Called(ctx, task, message)
	return args.String(0), args.Error(1)
}

func (m *MockHiveAgent) Validate(task *types.HiveTask) error {
	args := m.Called(task)
	return args.Error(0)
}

func (m *MockHiveAgent) Cleanup(ctx context.Context, task *types.HiveTask) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockHiveAgent) GetCapabilities() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockHiveAgent) Heartbeat() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewAnalyzerAgent(t *testing.T) {
	tests := []struct {
		name        string
		chatModel   model.ToolCallingChatModel
		expectError bool
	}{
		{
			name:        "Valid inputs",
			chatModel:   &MockChatModel{},
			expectError: false,
		},
		{
			name:        "Nil chat model",
			chatModel:   nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := NewAnalyzerAgent(tt.chatModel, nil)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, agent)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, agent)
				assert.Equal(t, "analyst", agent.GetType())
			}
		})
	}
}

func TestAnalyzerAgent_CanHandle(t *testing.T) {
	mockModel := &MockChatModel{}

	agent, err := NewAnalyzerAgent(mockModel, nil)
	assert.NoError(t, err)

	tests := []struct {
		name     string
		task     *types.HiveTask
		expected bool
	}{
		{
			name:     "Nil task",
			task:     nil,
			expected: false,
		},
		{
			name: "Valid task",
			task: &types.HiveTask{
				Goal: "Implement a new feature",
			},
			expected: true,
		},
		{
			name: "Another valid task",
			task: &types.HiveTask{
				Goal: "Debug an issue",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.CanHandle(tt.task)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnalyzerAgent_Validate(t *testing.T) {
	mockModel := &MockChatModel{}

	agent, err := NewAnalyzerAgent(mockModel, nil)
	assert.NoError(t, err)

	tests := []struct {
		name        string
		task        *types.HiveTask
		expectError bool
	}{
		{
			name:        "Nil task",
			task:        nil,
			expectError: true,
		},
		{
			name: "Empty goal",
			task: &types.HiveTask{
				Goal: "   ",
			},
			expectError: true,
		},
		{
			name: "Valid task",
			task: &types.HiveTask{
				Goal: "Implement feature X",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := agent.Validate(tt.task)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
