package react

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockChatModel implements a mock chat model for testing
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
	return args.Get(0).(*schema.Message), args.Error(1)
}

func (m *MockChatModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	args := m.Called(ctx, messages, opts)
	return args.Get(0).(*schema.StreamReader[*schema.Message]), args.Error(1)
}

func (m *MockChatModel) BindTools(toolInfo []*schema.ToolInfo) error {
	args := m.Called(toolInfo)
	return args.Error(0)
}

func TestNew(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		mockModel := &MockChatModel{}
		tools := []tool.InvokableTool{}

		mockModel.On("WithTools", mock.AnythingOfType("[]*schema.ToolInfo")).Return(mockModel, nil)

		agent, err := New("test-agent", mockModel, tools, 10)
		require.NoError(t, err)
		assert.Equal(t, "test-agent", agent.ID())
		assert.NotNil(t, agent.agent)

		mockModel.AssertExpectations(t)
	})

	t.Run("empty tools list", func(t *testing.T) {
		mockModel := &MockChatModel{}
		tools := []tool.InvokableTool{}

		agent, err := New("test-agent", mockModel, tools, 1)
		require.NoError(t, err)
		assert.Equal(t, "test-agent", agent.ID())

		mockModel.AssertExpectations(t)
	})
}

func TestNewWithSystemPrompt(t *testing.T) {
	mockModel := &MockChatModel{}
	tools := []tool.InvokableTool{}

	mockModel.On("WithTools", mock.AnythingOfType("[]*schema.ToolInfo")).Return(mockModel, nil)

	agent, err := NewWithSystemPrompt("test-agent", mockModel, tools, "Custom system prompt", 10)
	require.NoError(t, err)
	assert.Equal(t, "test-agent", agent.ID())
	assert.NotNil(t, agent.agent)

	mockModel.AssertExpectations(t)
}
