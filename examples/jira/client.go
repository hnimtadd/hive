package main

import (
	"encoding/json"
	"fmt"
	"os"

	jira "github.com/andygrunwald/go-jira"
)

func main() {
	tp := jira.BasicAuthTransport{
		Username: os.Getenv("JIRA_USERNAME"),
		Password: os.Getenv("JIRA_ACCESS_TOKEN"),
	}
	jiraClient, _ := jira.NewClient(tp.Client(), os.Getenv("JIRA_BASE_URL"))
	issue, _, _ := jiraClient.Issue.Get("T6-1274", nil)
	content, _ := json.MarshalIndent(issue, "", "  ")
	fmt.Fprint(os.Stdout, string(content))
}
