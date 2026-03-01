package todoer

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type TodoAgent struct {
}

type Config struct {
	Model  string
	APIKey string
}

func NewTodoAgent(conf Config) compose.Runnable[[]*schema.Message, []*schema.Message] {
	todoTools := []tool.BaseTool{
		newTodoAddTool(),
	}
	chatModel, err := claude.NewChatModel(context.Background(), &claude.Config{
		Model:  conf.Model,
		APIKey: conf.APIKey,
	})
	if err != nil {
		log.Fatal(err)
	}
	// Bind tool infos to ChatModel
	toolInfos := make([]*schema.ToolInfo, 0, len(todoTools))
	for _, tool := range todoTools {
		var info *schema.ToolInfo
		info, err = tool.Info(context.TODO())
		if err != nil {
			log.Fatal(err)
		}
		toolInfos = append(toolInfos, info)
	}
	err = chatModel.BindTools(toolInfos)
	if err != nil {
		log.Fatal(err)
	}

	// Create tools node
	todoToolsNode, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: todoTools,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Build chain
	chain := compose.NewChain[[]*schema.Message, []*schema.Message]()
	chain.
		AppendChatModel(chatModel, compose.WithNodeName("chat_model")).
		AppendToolsNode(todoToolsNode, compose.WithNodeName("tools"))

	// Compile and run
	agent, err := chain.Compile(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	return agent
}

type TodoAddParams struct {
	ID        string  `json:"id"                   jsonschema:"description=id of the todo"`
	Content   *string `json:"content,omitempty"    jsonschema:"description=content of the todo"`
	StartedAt *int64  `json:"started_at,omitempty" jsonschema:"description=start time in unix timestamp"`
	Deadline  *int64  `json:"deadline,omitempty"   jsonschema:"description=deadline of the todo in unix timestamp"`
	Done      *bool   `json:"done,omitempty"       jsonschema:"description=done status"`
}

func UpdateTodoFunc(_ context.Context, _ *TodoAddParams) (string, error) {
	return `{"msg": "update todo success"}`, nil
}

func newTodoAddTool() tool.InvokableTool {
	// Build tool with InferTool
	tool, _ := utils.InferTool(
		"update_todo", // tool name
		"Update a todo item, eg: content,deadline...", // description
		UpdateTodoFunc)
	return tool
}
