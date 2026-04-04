package main

import (
	"fmt"
	"os"

	"github.com/hnimtadd/hive/pkg/hive"
)

func mainGetIssue() {
	secrets := &GitLabSecrets{}
	gitlabTool := &GitLabTool{secrets: secrets}

	getIssueTool, err := hive.NewTool(
		"gitlab_get_issue",
		"Retrieve a specific GitLab issue by project and issue IID",
		gitlabTool.getIssue,
		hive.WithSecret[GetIssueInput, Issue](secrets),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create get_issue tool: %v\n", err)
		os.Exit(1)
	}

	getIssueTool.Serve()
}
