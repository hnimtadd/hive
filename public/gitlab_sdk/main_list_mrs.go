package main

import (
	"fmt"
	"os"

	"github.com/hnimtadd/hive/pkg/hive"
)

func mainListMRs() {
	secrets := &GitLabSecrets{}
	gitlabTool := &GitLabTool{secrets: secrets}

	listMRTool, err := hive.NewTool(
		"gitlab_list_mrs",
		"List merge requests for a GitLab project",
		gitlabTool.listMergeRequests,
		hive.WithSecret[ListMergeRequestsInput, ListMergeRequestsOutput](secrets),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create list_mrs tool: %v\n", err)
		os.Exit(1)
	}

	listMRTool.Serve()
}
