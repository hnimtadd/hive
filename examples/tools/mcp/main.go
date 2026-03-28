package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	s := mcp.NewServer(&mcp.Implementation{Name: "example", Version: "1.0.0"}, nil)
	type args struct {
		Name string `json:"name" jsonschema:"the person to greet"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "greet",
		Description: "say hi",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args args) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "Hi " + args.Name,
				},
			},
		}, nil, nil
	})

	if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("Server failed: %v", err)
	}
}
