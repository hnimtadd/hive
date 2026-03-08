package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/xanzy/go-gitlab"
)

// GitlabAPITool provides direct access to GitLab API
type GitlabAPITool struct {
	client *gitlab.Client
}

// Info implements [tool.InvokableTool].
func (t *GitlabAPITool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "gitlab_api",
		Desc: "Call GitLab API directly. Supports operations like: get_project (get project info by path), create_merge_request (create MR), list_branches, get_file (read file from GitLab), create_branch, etc.",
		ParamsOneOf: schema.NewParamsOneOfByParams(
			map[string]*schema.ParameterInfo{
				"operation": {
					Type:     schema.String,
					Desc:     "API operation: get_project, create_merge_request, list_branches, get_file, create_branch",
					Required: true,
					Enum:     []string{"get_project", "create_merge_request", "list_branches", "get_file", "create_branch"},
				},
				"project_path": {
					Type:     schema.String,
					Desc:     "GitLab project path (e.g., 'group/repo'). Required for most operations",
					Required: false,
				},
				"source_branch": {
					Type:     schema.String,
					Desc:     "Source branch name (for create_merge_request, create_branch)",
					Required: false,
				},
				"target_branch": {
					Type:     schema.String,
					Desc:     "Target branch name (for create_merge_request)",
					Required: false,
				},
				"title": {
					Type:     schema.String,
					Desc:     "Title (for create_merge_request)",
					Required: false,
				},
				"description": {
					Type:     schema.String,
					Desc:     "Description (for create_merge_request)",
					Required: false,
				},
				"file_path": {
					Type:     schema.String,
					Desc:     "File path in repository (for get_file)",
					Required: false,
				},
				"ref": {
					Type:     schema.String,
					Desc:     "Git reference/branch (for get_file, create_branch)",
					Required: false,
				},
			},
		),
	}, nil
}

// InvokableRun implements [tool.InvokableTool].
func (t *GitlabAPITool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Operation    string  `json:"operation"`
		ProjectPath  *string `json:"project_path,omitempty"`
		SourceBranch *string `json:"source_branch,omitempty"`
		TargetBranch *string `json:"target_branch,omitempty"`
		Title        *string `json:"title,omitempty"`
		Description  *string `json:"description,omitempty"`
		FilePath     *string `json:"file_path,omitempty"`
		Ref          *string `json:"ref,omitempty"`
	}

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if args.Operation == "" {
		return "", fmt.Errorf("operation is required")
	}

	switch args.Operation {
	case "get_project":
		return t.getProject(args.ProjectPath)

	case "create_merge_request":
		return t.createMergeRequest(args.ProjectPath, args.SourceBranch, args.TargetBranch, args.Title, args.Description)

	case "list_branches":
		return t.listBranches(args.ProjectPath)

	case "get_file":
		return t.getFile(args.ProjectPath, args.FilePath, args.Ref)

	case "create_branch":
		return t.createBranch(args.ProjectPath, args.SourceBranch, args.Ref)

	default:
		return "", fmt.Errorf("unsupported operation: %s", args.Operation)
	}
}

func (t *GitlabAPITool) getProject(projectPath *string) (string, error) {
	if projectPath == nil || *projectPath == "" {
		return "", fmt.Errorf("project_path is required for get_project")
	}

	project, _, err := t.client.Projects.GetProject(*projectPath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get project: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"status":              "success",
		"id":                  project.ID,
		"name":                project.Name,
		"path":                project.Path,
		"path_with_namespace": project.PathWithNamespace,
		"web_url":             project.WebURL,
		"http_url_to_repo":    project.HTTPURLToRepo,
		"ssh_url_to_repo":     project.SSHURLToRepo,
		"default_branch":      project.DefaultBranch,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(result), nil
}

func (t *GitlabAPITool) createMergeRequest(projectPath, sourceBranch, targetBranch, title, description *string) (string, error) {
	if projectPath == nil || *projectPath == "" {
		return "", fmt.Errorf("project_path is required for create_merge_request")
	}
	if sourceBranch == nil || *sourceBranch == "" {
		return "", fmt.Errorf("source_branch is required for create_merge_request")
	}
	if targetBranch == nil || *targetBranch == "" {
		return "", fmt.Errorf("target_branch is required for create_merge_request")
	}

	mrTitle := "Automated Merge Request"
	if title != nil && *title != "" {
		mrTitle = *title
	}

	mrDesc := "Created by Hive"
	if description != nil && *description != "" {
		mrDesc = *description
	}

	mr, _, err := t.client.MergeRequests.CreateMergeRequest(*projectPath, &gitlab.CreateMergeRequestOptions{
		Title:        &mrTitle,
		Description:  &mrDesc,
		SourceBranch: sourceBranch,
		TargetBranch: targetBranch,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create merge request: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"status":        "success",
		"id":            mr.ID,
		"iid":           mr.IID,
		"title":         mr.Title,
		"web_url":       mr.WebURL,
		"source_branch": mr.SourceBranch,
		"target_branch": mr.TargetBranch,
		"state":         mr.State,
		"merge_status":  mr.DetailedMergeStatus,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(result), nil
}

func (t *GitlabAPITool) listBranches(projectPath *string) (string, error) {
	if projectPath == nil || *projectPath == "" {
		return "", fmt.Errorf("project_path is required for list_branches")
	}

	branches, _, err := t.client.Branches.ListBranches(*projectPath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list branches: %w", err)
	}

	branchNames := make([]string, len(branches))
	for i, branch := range branches {
		branchNames[i] = branch.Name
	}

	result, err := json.Marshal(map[string]any{
		"status":   "success",
		"count":    len(branches),
		"branches": branchNames,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(result), nil
}

func (t *GitlabAPITool) getFile(projectPath, filePath, ref *string) (string, error) {
	if projectPath == nil || *projectPath == "" {
		return "", fmt.Errorf("project_path is required for get_file")
	}
	if filePath == nil || *filePath == "" {
		return "", fmt.Errorf("file_path is required for get_file")
	}

	opts := &gitlab.GetFileOptions{}
	if ref != nil && *ref != "" {
		opts.Ref = ref
	}

	file, _, err := t.client.RepositoryFiles.GetFile(*projectPath, *filePath, opts)
	if err != nil {
		return "", fmt.Errorf("failed to get file: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"status":    "success",
		"file_path": file.FilePath,
		"file_name": file.FileName,
		"content":   file.Content,
		"size":      file.Size,
		"encoding":  file.Encoding,
		"ref":       file.Ref,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(result), nil
}

func (t *GitlabAPITool) createBranch(projectPath, branchName, ref *string) (string, error) {
	if projectPath == nil || *projectPath == "" {
		return "", fmt.Errorf("project_path is required for create_branch")
	}
	if branchName == nil || *branchName == "" {
		return "", fmt.Errorf("source_branch is required for create_branch (branch name to create)")
	}
	if ref == nil || *ref == "" {
		return "", fmt.Errorf("ref is required for create_branch (branch to create from)")
	}

	branch, _, err := t.client.Branches.CreateBranch(*projectPath, &gitlab.CreateBranchOptions{
		Branch: branchName,
		Ref:    ref,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create branch: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"status": "success",
		"name":   branch.Name,
		"commit": branch.Commit.ID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(result), nil
}

// NewGitlabAPITool creates a new GitLab API tool
func NewGitlabAPITool(baseURL, token string) (tool.InvokableTool, error) {
	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	return &GitlabAPITool{
		client: client,
	}, nil
}
