package gitlab

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/xanzy/go-gitlab"
)

// GitLabIntegrator handles GitLab operations for The Hive
type GitLabIntegrator struct {
	client       *gitlab.Client
	workspaceDir string
	currentRepo  *RepositoryInfo
}

// RepositoryInfo contains information about the current working repository
type RepositoryInfo struct {
	ProjectID     int    `json:"project_id"`
	ProjectURL    string `json:"project_url"`
	CurrentBranch string `json:"current_branch"`
	SourceBranch  string `json:"source_branch"`
	TargetBranch  string `json:"target_branch"`
}

// CommitInfo represents a git commit
type CommitInfo struct {
	SHA     string   `json:"sha"`
	Message string   `json:"message"`
	Files   []string `json:"files"`
}

// NewGitLabIntegrator creates a new GitLab integrator
func NewGitLabIntegrator(workspaceDir string) (*GitLabIntegrator, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return NewGitLabIntegratorWithConfig(&cfg.GitLab, workspaceDir)
}

// NewGitLabIntegratorWithConfig creates a new GitLab integrator with provided config
func NewGitLabIntegratorWithConfig(cfg *config.GitLabConfig, workspaceDir string) (*GitLabIntegrator, error) {
	token := os.Getenv(cfg.TokenEnv)
	if token == "" {
		return nil, fmt.Errorf("%s environment variable is required", cfg.TokenEnv)
	}

	// Use workspace directory from config if not provided
	if workspaceDir == "" {
		workspaceDir = cfg.WorkspaceDir
	}

	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(cfg.URL))
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	// Ensure workspace directory exists
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	return &GitLabIntegrator{
		client:       client,
		workspaceDir: workspaceDir,
	}, nil
}

// PrepareWorkspace prepares a GitLab workspace for development
func (g *GitLabIntegrator) PrepareWorkspace(ctx context.Context, projectID int, targetBranch string) (*RepositoryInfo, error) {
	// Get project information
	project, _, err := g.client.Projects.GetProject(projectID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %d: %w", projectID, err)
	}

	repoDir := filepath.Join(g.workspaceDir, fmt.Sprintf("project-%d", projectID))

	// Clone or update repository
	if err := g.ensureRepository(project.HTTPURLToRepo, repoDir); err != nil {
		return nil, fmt.Errorf("failed to prepare repository: %w", err)
	}

	// Create feature branch
	sourceBranch := fmt.Sprintf("hive/feature-%s", uuid.New().String()[:8])
	if err := g.createFeatureBranch(repoDir, sourceBranch, targetBranch); err != nil {
		return nil, fmt.Errorf("failed to create feature branch: %w", err)
	}

	repoInfo := &RepositoryInfo{
		ProjectID:     projectID,
		ProjectURL:    project.WebURL,
		CurrentBranch: sourceBranch,
		SourceBranch:  sourceBranch,
		TargetBranch:  targetBranch,
	}

	g.currentRepo = repoInfo
	return repoInfo, nil
}

// WriteFile writes content to a file in the workspace
func (g *GitLabIntegrator) WriteFile(filePath, content string) error {
	if g.currentRepo == nil {
		return fmt.Errorf("workspace not prepared")
	}

	repoDir := filepath.Join(g.workspaceDir, fmt.Sprintf("project-%d", g.currentRepo.ProjectID))
	fullPath := filepath.Join(repoDir, filePath)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

// ReadFile reads content from a file in the workspace
func (g *GitLabIntegrator) ReadFile(filePath string) (string, error) {
	if g.currentRepo == nil {
		return "", fmt.Errorf("workspace not prepared")
	}

	repoDir := filepath.Join(g.workspaceDir, fmt.Sprintf("project-%d", g.currentRepo.ProjectID))
	fullPath := filepath.Join(repoDir, filePath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return string(content), nil
}

// CommitChanges commits the specified files with a message
func (g *GitLabIntegrator) CommitChanges(files []string, message string) (*CommitInfo, error) {
	if g.currentRepo == nil {
		return nil, fmt.Errorf("workspace not prepared")
	}

	repoDir := filepath.Join(g.workspaceDir, fmt.Sprintf("project-%d", g.currentRepo.ProjectID))

	// Add files to git
	for _, file := range files {
		if err := g.runGitCommand(repoDir, "add", file); err != nil {
			return nil, fmt.Errorf("failed to add file %s: %w", file, err)
		}
	}

	// Check if there are changes to commit
	if hasChanges, err := g.hasUncommittedChanges(repoDir); err != nil {
		return nil, err
	} else if !hasChanges {
		return nil, fmt.Errorf("no changes to commit")
	}

	// Commit changes
	if err := g.runGitCommand(repoDir, "commit", "-m", message); err != nil {
		return nil, fmt.Errorf("failed to commit changes: %w", err)
	}

	// Get commit SHA
	sha, err := g.getLatestCommitSHA(repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit SHA: %w", err)
	}

	return &CommitInfo{
		SHA:     sha,
		Message: message,
		Files:   files,
	}, nil
}

// PushBranch pushes the current branch to GitLab
func (g *GitLabIntegrator) PushBranch() error {
	if g.currentRepo == nil {
		return fmt.Errorf("workspace not prepared")
	}

	repoDir := filepath.Join(g.workspaceDir, fmt.Sprintf("project-%d", g.currentRepo.ProjectID))

	// Push branch
	if err := g.runGitCommand(repoDir, "push", "-u", "origin", g.currentRepo.SourceBranch); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}

	return nil
}

// CreateMergeRequest creates a merge request on GitLab
func (g *GitLabIntegrator) CreateMergeRequest(ctx context.Context, title, description string) (*gitlab.MergeRequest, error) {
	if g.currentRepo == nil {
		return nil, fmt.Errorf("workspace not prepared")
	}

	// Create merge request
	mr, _, err := g.client.MergeRequests.CreateMergeRequest(g.currentRepo.ProjectID, &gitlab.CreateMergeRequestOptions{
		Title:        &title,
		Description:  &description,
		SourceBranch: &g.currentRepo.SourceBranch,
		TargetBranch: &g.currentRepo.TargetBranch,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create merge request: %w", err)
	}

	return mr, nil
}

// GetProjectFiles lists files in the project repository
func (g *GitLabIntegrator) GetProjectFiles(ctx context.Context, path string) ([]string, error) {
	if g.currentRepo == nil {
		return nil, fmt.Errorf("workspace not prepared")
	}

	repoDir := filepath.Join(g.workspaceDir, fmt.Sprintf("project-%d", g.currentRepo.ProjectID))
	searchPath := filepath.Join(repoDir, path)

	var files []string
	err := filepath.Walk(searchPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Make path relative to repo root
			relPath, err := filepath.Rel(repoDir, filePath)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}
		return nil
	})

	return files, err
}

// ensureRepository clones or updates the repository
func (g *GitLabIntegrator) ensureRepository(repoURL, repoDir string) error {
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		// Clone repository
		if err := g.runGitCommand(g.workspaceDir, "clone", repoURL, filepath.Base(repoDir)); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
	} else {
		// Repository exists, fetch latest changes
		if err := g.runGitCommand(repoDir, "fetch", "origin"); err != nil {
			return fmt.Errorf("failed to fetch updates: %w", err)
		}
	}

	return nil
}

// createFeatureBranch creates and checks out a new feature branch
func (g *GitLabIntegrator) createFeatureBranch(repoDir, branchName, baseBranch string) error {
	// Ensure we're on the base branch and it's up to date
	if err := g.runGitCommand(repoDir, "checkout", baseBranch); err != nil {
		return fmt.Errorf("failed to checkout base branch: %w", err)
	}

	if err := g.runGitCommand(repoDir, "pull", "origin", baseBranch); err != nil {
		return fmt.Errorf("failed to pull latest changes: %w", err)
	}

	// Create and checkout feature branch
	if err := g.runGitCommand(repoDir, "checkout", "-b", branchName); err != nil {
		return fmt.Errorf("failed to create feature branch: %w", err)
	}

	return nil
}

// runGitCommand executes a git command in the specified directory
func (g *GitLabIntegrator) runGitCommand(workDir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git command failed: %s (output: %s)", err, output)
	}

	return nil
}

// getLatestCommitSHA gets the SHA of the latest commit
func (g *GitLabIntegrator) getLatestCommitSHA(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit SHA: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// hasUncommittedChanges checks if there are staged changes ready to commit
func (g *GitLabIntegrator) hasUncommittedChanges(repoDir string) (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = repoDir

	err := cmd.Run()
	if err != nil {
		// If the command fails, there are staged changes
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("failed to check for changes: %w", err)
	}

	// If the command succeeds, there are no staged changes
	return false, nil
}

// GetWorkspaceDir returns the current workspace directory
func (g *GitLabIntegrator) GetWorkspaceDir() string {
	return g.workspaceDir
}

// GetCurrentRepo returns information about the current repository
func (g *GitLabIntegrator) GetCurrentRepo() *RepositoryInfo {
	return g.currentRepo
}

// Cleanup removes the workspace directory
func (g *GitLabIntegrator) Cleanup() error {
	if g.currentRepo != nil {
		repoDir := filepath.Join(g.workspaceDir, fmt.Sprintf("project-%d", g.currentRepo.ProjectID))
		return os.RemoveAll(repoDir)
	}
	return nil
}

