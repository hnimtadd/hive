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

type GreetSecret struct {
	Key string `hive:"key=KEY;description=Say hello to someone;omitempty"`
}

func main() {
	s := &GreetSecret{}
	tool, err := hive.NewTool(
		"greet",
		"A simple greeting tool",
		func(ctx context.Context, input GreetInput) (GreetOutput, error) {
			hive.Debugln("Secret", s)
			return GreetOutput{
				Message: fmt.Sprintf("Hello, %s!", input.Name),
			}, nil
		},
		hive.WithSecret[GreetInput, GreetOutput](s),
	)
	if err != nil {
		log.Println(err)
	}

	// Start serving requests via stdin/stdout
	tool.Serve()
}
