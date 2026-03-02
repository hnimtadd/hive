package react

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	einoreact "github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hnimtadd/hive/internal/tools/einotools"
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

func TestNewEinoReACTAgent(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		mockModel := &MockChatModel{}
		tools := []tool.InvokableTool{
			einotools.NewThinkTool(),
		}

		// Mock the BindTools call that Eino makes internally
		mockModel.On("BindTools", mock.AnythingOfType("[]*schema.ToolInfo")).Return(nil)

		agent, err := NewReACTAgent("test-agent", mockModel, tools)
		require.NoError(t, err)
		assert.Equal(t, "test-agent", agent.id)
		assert.NotNil(t, agent.agent)
		assert.Equal(t, len(tools), len(agent.tools))
		assert.NotNil(t, agent.config)

		mockModel.AssertExpectations(t)
	})

	t.Run("with custom options", func(t *testing.T) {
		mockModel := &MockChatModel{}
		tools := []tool.InvokableTool{
			einotools.NewThinkTool(),
		}

		// Mock the BindTools call
		mockModel.On("BindTools", mock.AnythingOfType("[]*schema.ToolInfo")).Return(nil)

		agent, err := NewReACTAgent(
			"custom-agent",
			mockModel,
			tools,
			WithMaxIterations(5),
			WithSystemPrompt("Custom prompt"),
			WithGraphName("test-graph"),
		)
		require.NoError(t, err)
		assert.Equal(t, "custom-agent", agent.id)
		assert.Equal(t, len(tools), len(agent.tools))
		assert.NotNil(t, agent.config.MessageModifier)
		assert.Equal(t, "test-graph", agent.config.GraphName)

		mockModel.AssertExpectations(t)
	})

	t.Run("empty tools list", func(t *testing.T) {
		mockModel := &MockChatModel{}
		tools := []tool.InvokableTool{}

		// With no tools, BindTools should not be called
		agent, err := NewReACTAgent("test-agent", mockModel, tools)
		require.NoError(t, err)
		assert.Equal(t, "test-agent", agent.id)
		assert.Equal(t, 0, len(agent.tools))
		assert.Equal(t, 0, len(agent.config.ToolsConfig.Tools))

		mockModel.AssertExpectations(t)
	})
}

func TestEinoReACTAgent_ID(t *testing.T) {
	mockModel := &MockChatModel{}
	agent, err := NewReACTAgent("test-id", mockModel, []tool.InvokableTool{})
	require.NoError(t, err)

	assert.Equal(t, "test-id", agent.ID())
}

func TestEinoReACTAgent_GetTools(t *testing.T) {
	mockModel := &MockChatModel{}
	tools := []tool.InvokableTool{
		einotools.NewThinkTool(),
		einotools.NewFileReadTool(),
	}

	// Mock the BindTools call
	mockModel.On("BindTools", mock.AnythingOfType("[]*schema.ToolInfo")).Return(nil)

	agent, err := NewReACTAgent("test-agent", mockModel, tools)
	require.NoError(t, err)

	retrievedTools := agent.GetTools()
	assert.Equal(t, len(tools), len(retrievedTools))

	mockModel.AssertExpectations(t)
}

func TestEinoReACTAgent_GetAgent(t *testing.T) {
	mockModel := &MockChatModel{}
	agent, err := NewReACTAgent("test-agent", mockModel, []tool.InvokableTool{})
	require.NoError(t, err)

	einoAgent := agent.GetAgent()
	assert.NotNil(t, einoAgent)
}

func TestEinoReACTAgent_Methods(t *testing.T) {
	t.Run("interface check", func(t *testing.T) {
		mockModel := &MockChatModel{}
		tools := []tool.InvokableTool{
			einotools.NewThinkTool(),
		}

		// Mock the BindTools call
		mockModel.On("BindTools", mock.AnythingOfType("[]*schema.ToolInfo")).Return(nil)

		reactAgent, err := NewReACTAgent("test-agent", mockModel, tools)
		require.NoError(t, err)

		// Test that the agent has the correct methods
		assert.NotNil(t, reactAgent.agent)

		// Test method signatures without calling (since we'd need real model)
		ctx := context.Background()
		messages := []*schema.Message{schema.UserMessage("test")}

		// These would normally work with a real model
		_, _ = reactAgent.RunWithMessages(ctx, messages)
		_, _ = reactAgent.StreamWithMessages(ctx, messages)
		_, _ = reactAgent.Run(ctx, "test")
		_, _ = reactAgent.Stream(ctx, "test")

		mockModel.AssertExpectations(t)
	})
}

func TestOptions(t *testing.T) {
	t.Run("WithMaxIterations", func(t *testing.T) {
		agent := &ReACTAgent{config: &einoreact.AgentConfig{}}
		opt := WithMaxIterations(20)
		opt(agent)
		// WithMaxIterations is currently a no-op placeholder
	})

	t.Run("WithSystemPrompt", func(t *testing.T) {
		agent := &ReACTAgent{config: &einoreact.AgentConfig{}}
		opt := WithSystemPrompt("custom prompt")
		opt(agent)
		assert.NotNil(t, agent.config.MessageModifier)

		// Test the message modifier
		ctx := context.Background()
		input := []*schema.Message{schema.UserMessage("test")}
		result := agent.config.MessageModifier(ctx, input)

		assert.Equal(t, 2, len(result))
		assert.Equal(t, schema.System, result[0].Role)
		assert.Contains(t, result[0].Content, "custom prompt")
		assert.Equal(t, schema.User, result[1].Role)
	})

	t.Run("WithGraphName", func(t *testing.T) {
		agent := &ReACTAgent{config: &einoreact.AgentConfig{}}
		opt := WithGraphName("test-graph")
		opt(agent)
		assert.Equal(t, "test-graph", agent.config.GraphName)
	})

	t.Run("WithMessageModifier", func(t *testing.T) {
		agent := &ReACTAgent{config: &einoreact.AgentConfig{}}

		customModifier := func(ctx context.Context, input []*schema.Message) []*schema.Message {
			return append([]*schema.Message{schema.SystemMessage("custom")}, input...)
		}

		opt := WithMessageModifier(customModifier)
		opt(agent)
		assert.NotNil(t, agent.config.MessageModifier)
	})

	t.Run("multiple options", func(t *testing.T) {
		agent := &ReACTAgent{config: &einoreact.AgentConfig{}}

		WithMaxIterations(10)(agent)
		WithSystemPrompt("test")(agent)
		WithGraphName("multi-test")(agent)

		assert.NotNil(t, agent.config.MessageModifier)
		assert.Equal(t, "multi-test", agent.config.GraphName)
	})
}

// Integration test with real Eino components (requires a real model)
func TestEinoReACTAgent_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require actual Eino model setup
	// For now, it's a placeholder to show how integration tests could be structured
	t.Skip("Integration test requires real model configuration")

	// Example of what the integration test might look like:
	/*
		model, err := claude.NewChatModel(ctx, &claude.Config{
			APIKey: os.Getenv("CLAUDE_API_KEY"),
			Model:  "claude-3-sonnet-20240229",
		})
		require.NoError(t, err)

		tools := []tool.InvokableTool{
			einotools.NewThinkTool(),
		}

		agent, err := NewEinoReACTAgent("integration-test", model, tools,
			WithSystemPrompt("You are a helpful assistant for testing."),
			WithGraphName("integration-test-graph"),
		)
		require.NoError(t, err)

		result, err := agent.Run(context.Background(), "Think about the number 42 and tell me what's special about it.")
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.Content)
	*/
}
