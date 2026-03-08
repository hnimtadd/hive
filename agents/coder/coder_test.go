package coder

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockChatModel implements Eino's model.ChatModel interface for testing
type MockChatModel struct {
	mock.Mock
}

// WithTools implements [model.ToolCallingChatModel].
func (m *MockChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	args := m.Called(tools)
	return args.Get(0).(model.ToolCallingChatModel), args.Error(1)
}

func (m *MockChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	args := m.Called(ctx, messages, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*schema.Message), args.Error(1)
}

func (m *MockChatModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	args := m.Called(ctx, messages, opts)
	return args.Get(0).(*schema.StreamReader[*schema.Message]), args.Error(1)
}

func (m *MockChatModel) BindTools(tools []*schema.ToolInfo) error {
	args := m.Called(tools)
	return args.Error(0)
}

// MockInvokableTool implements Eino's tool.InvokableTool interface for testing
type MockInvokableTool struct {
	mock.Mock
	name string
}

func NewMockInvokableTool(name string) *MockInvokableTool {
	return &MockInvokableTool{name: name}
}

func (m *MockInvokableTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).(*schema.ToolInfo), args.Error(1)
}

func (m *MockInvokableTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	args := m.Called(ctx, argumentsInJSON, opts)
	return args.String(0), args.Error(1)
}

func TestNewEnhancedCoderAgent(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		mockModel := &MockChatModel{}

		// Mock WithTools call that will be made during ReACT agent creation
		mockModel.On("WithTools", mock.AnythingOfType("[]*schema.ToolInfo")).Return(mockModel, nil)

		agent, err := NewAgent(mockModel, nil)

		assert.NoError(t, err)
		assert.NotNil(t, agent)
		assert.Equal(t, "enhanced_coder", agent.GetType())
		assert.NotEmpty(t, agent.GetID())
		assert.NotEmpty(t, agent.GetCapabilities())

		mockModel.AssertExpectations(t)
	})

	t.Run("nil chat model", func(t *testing.T) {
		agent, err := NewAgent(nil, nil)

		assert.Error(t, err)
		assert.Nil(t, agent)
		assert.Contains(t, err.Error(), "chat model is required")
	})
}

func TestEnhancedCoderAgent_CanHandle(t *testing.T) {
	mockModel := &MockChatModel{}
	mockModel.On("WithTools", mock.AnythingOfType("[]*schema.ToolInfo")).Return(mockModel, nil)

	agent, err := NewAgent(mockModel, nil)
	assert.NoError(t, err)

	testCases := []struct {
		name     string
		task     *types.HiveTask
		expected bool
	}{
		{
			name:     "nil task",
			task:     nil,
			expected: false,
		},
		{
			name:     "coding task - implement",
			task:     &types.HiveTask{Goal: "implement a new function"},
			expected: true,
		},
		{
			name:     "coding task - write code",
			task:     &types.HiveTask{Goal: "write code for authentication"},
			expected: true,
		},
		{
			name:     "coding task - debug",
			task:     &types.HiveTask{Goal: "debug the login issue"},
			expected: true,
		},
		{
			name:     "non-coding task",
			task:     &types.HiveTask{Goal: "send an email to customers"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := agent.CanHandle(tc.task)
			assert.Equal(t, tc.expected, result)
		})
	}

	mockModel.AssertExpectations(t)
}

func TestEnhancedCoderAgent_Execute(t *testing.T) {
	t.Run("nil task", func(t *testing.T) {
		mockModel := &MockChatModel{}
		mockModel.On("WithTools", mock.AnythingOfType("[]*schema.ToolInfo")).Return(mockModel, nil)

		agent, err := NewAgent(mockModel, nil)
		assert.NoError(t, err)

		err = agent.Execute(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task cannot be nil")

		mockModel.AssertExpectations(t)
	})
}

func TestEnhancedCoderAgent_Validate(t *testing.T) {
	mockModel := &MockChatModel{}
	mockModel.On("WithTools", mock.AnythingOfType("[]*schema.ToolInfo")).Return(mockModel, nil)

	agent, err := NewAgent(mockModel, nil)
	assert.NoError(t, err)

	t.Run("valid task", func(t *testing.T) {
		task := &types.HiveTask{
			Goal: "implement a function",
		}
		err := agent.Validate(task)
		assert.NoError(t, err)
	})

	t.Run("nil task", func(t *testing.T) {
		err := agent.Validate(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task is required")
	})

	t.Run("empty goal", func(t *testing.T) {
		task := &types.HiveTask{
			Goal: "   ",
		}
		err := agent.Validate(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task goal cannot be empty")
	})

	mockModel.AssertExpectations(t)
}

func TestEnhancedCoderAgent_AddTool(t *testing.T) {
	mockModel := &MockChatModel{}
	mockModel.On("WithTools", mock.AnythingOfType("[]*schema.ToolInfo")).Return(mockModel, nil)

	agent, err := NewAgent(mockModel, nil)
	assert.NoError(t, err)

	mockTool := NewMockInvokableTool("custom_tool")

	err = agent.AddTool(mockTool)
	assert.NoError(t, err)

	tools := agent.ListTools()
	assert.Contains(t, tools, mockTool)

	mockModel.AssertExpectations(t)
}

func TestEnhancedCoderAgent_Misc(t *testing.T) {
	mockModel := &MockChatModel{}
	mockModel.On("WithTools", mock.AnythingOfType("[]*schema.ToolInfo")).Return(mockModel, nil)

	agent, err := NewAgent(mockModel, nil)
	assert.NoError(t, err)

	// Test various methods
	assert.NotEmpty(t, agent.GetID())
	assert.Equal(t, "enhanced_coder", agent.GetType())
	assert.NotEmpty(t, agent.GetCapabilities())
	assert.NotNil(t, agent.ListTools())
	assert.NoError(t, agent.Heartbeat())
	assert.NotNil(t, agent.GetAgent())

	// Test lifecycle methods
	assert.NoError(t, agent.Setup(context.Background(), nil))
	assert.NoError(t, agent.Cleanup(context.Background(), &types.HiveTask{}))
	assert.NoError(t, agent.ReportStatus(context.Background(), &types.HiveTask{}))

	feedback, err := agent.RequestFeedback(context.Background(), &types.HiveTask{}, "test message")
	assert.NoError(t, err)
	assert.NotEmpty(t, feedback)

	mockModel.AssertExpectations(t)
}
