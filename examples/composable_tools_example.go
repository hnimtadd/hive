package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/hnimtadd/hive/internal/tools"
	"github.com/hnimtadd/hive/pkg/config"
)

// This example demonstrates the composable local tools approach
// The agent can orchestrate these low-level tools to accomplish complex workflows

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Create composable tools
	var localTools []tool.InvokableTool

	// 1. Shell execution tool - for git and other commands
	shellTool := tools.NewExecuteShellTool("")
	localTools = append(localTools, shellTool)

	// 2. File system tools
	localTools = append(localTools,
		tools.NewListFilesTool(""),
		tools.NewLocalFileReadTool(""),
		tools.NewLocalFileWriteTool(""),
	)

	// 3. GitLab API tool - for GitLab-specific operations
	gitlabAPITool, err := tools.NewGitlabAPITool(cfg.Gitlab.URL, cfg.Gitlab.TokenEnv)
	if err != nil {
		log.Fatal("Failed to create GitLab API tool:", err)
	}
	localTools = append(localTools, gitlabAPITool)

	// Step 1: Get project info from GitLab
	fmt.Println("1. Getting project info from GitLab API...")
	result, err := gitlabAPITool.InvokableRun(ctx, `{
		"operation": "get_project",
		"project_path": "mygroup/myrepo"
	}`)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n\n", result)
	}

	// Step 2: Clone repository using shell
	fmt.Println("2. Cloning repository using shell...")
	result, err = shellTool.InvokableRun(ctx, `{
		"command": "git clone git@gitlab.com:mygroup/myrepo.git /tmp/myrepo",
		"working_dir": "/tmp"
	}`)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n\n", result)
	}

	// Step 3: Create a new branch using shell
	fmt.Println("3. Creating feature branch...")
	result, err = shellTool.InvokableRun(ctx, `{
		"command": "git checkout -b feature/my-feature",
		"working_dir": "/tmp/myrepo"
	}`)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n\n", result)
	}

	// Step 4: List files in the repo
	fmt.Println("4. Listing files in repository...")
	listTool := tools.NewListFilesTool("/tmp/myrepo")
	result, err = listTool.InvokableRun(ctx, `{
		"path": ".",
		"recursive": false
	}`)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n\n", result)
	}

	// Step 5: Read a file
	fmt.Println("5. Reading README.md...")
	readTool := tools.NewLocalFileReadTool("/tmp/myrepo")
	result, err = readTool.InvokableRun(ctx, `{
		"path": "README.md"
	}`)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n\n", result)
	}

	// Step 6: Write a new file
	fmt.Println("6. Writing new file...")
	writeTool := tools.NewLocalFileWriteTool("/tmp/myrepo")
	result, err = writeTool.InvokableRun(ctx, `{
		"path": "src/newfile.go",
		"content": "package main\n\nfunc main() {\n\t// TODO: implement\n}\n"
	}`)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n\n", result)
	}

	// Step 7: Commit changes using shell
	fmt.Println("7. Committing changes...")
	result, err = shellTool.InvokableRun(ctx, `{
		"command": "git add . && git commit -m 'Add new feature'",
		"working_dir": "/tmp/myrepo"
	}`)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n\n", result)
	}

	// Step 8: Push branch using shell
	fmt.Println("8. Pushing branch to GitLab...")
	result, err = shellTool.InvokableRun(ctx, `{
		"command": "git push -u origin feature/my-feature",
		"working_dir": "/tmp/myrepo"
	}`)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n\n", result)
	}

	// Step 9: Create merge request using GitLab API
	fmt.Println("9. Creating merge request via GitLab API...")
	result, err = gitlabAPITool.InvokableRun(ctx, `{
		"operation": "create_merge_request",
		"project_path": "mygroup/myrepo",
		"source_branch": "feature/my-feature",
		"target_branch": "main",
		"title": "Add new feature",
		"description": "This MR adds a new feature to the project"
	}`)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n\n", result)
	}

	log.Println("=== Workflow Complete ===")
	log.Println("\nKey Benefits of This Approach:")
	log.Println("1. Flexible: Agent can adapt workflow to any git provider")
	log.Println("2. Transparent: Each step is visible through tool calls")
	log.Println("3. Composable: Tools can be mixed and matched")
	log.Println("4. Simple: Each tool does one thing well")
}
