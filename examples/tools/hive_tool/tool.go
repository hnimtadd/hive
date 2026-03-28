package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hnimtadd/hive/pkg/hive"
)

type GreetInput struct {
	Name string `json:"name" jsonschema:"description=The name to greet"`
}

type GreetOutput struct {
	Message string `json:"message"`
}

func main() {
	tool, err := hive.NewTool(
		"greet",
		"A simple greeting tool",
		func(ctx context.Context, input GreetInput) (GreetOutput, error) {
			return GreetOutput{
				Message: fmt.Sprintf("Hello, %s!", input.Name),
			}, nil
		},
	)
	if err != nil {
		log.Println(err)
	}

	// Start serving requests via stdin/stdout
	tool.Serve()
}
