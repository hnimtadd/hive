#!/bin/bash
# Build all required tools for the hive setup

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PUBLIC_DIR="$REPO_ROOT/examples/tools"
TOOLS_DIR="$REPO_ROOT/examples/hive/tools"

echo "Building Hive SDK tools..."
echo "Repository root: $REPO_ROOT"
echo "Tools directory: $TOOLS_DIR"
echo ""

# Build GitLab tools
echo "Building GitLab tools..."
cd "$PUBLIC_DIR/gitlab_sdk"
go mod tidy >/dev/null 2>&1
go build -o gitlab_get_mr main.go
cp gitlab_get_mr "$TOOLS_DIR/gitlab_get_mr"
echo "gitlab_get_mr built and copied"

# Build Jira tools
echo "Building Jira tools..."
cd "$PUBLIC_DIR/jira_sdk"
go mod tidy >/dev/null 2>&1
go build -o jira_get_issue main.go
cp jira_get_issue "$TOOLS_DIR/jira_get_issue"
echo "jira_get_issue built and copied"

# Note: jira_search_issues needs to be built separately
# For now, the main binary serves both functions

echo ""
echo "All tools built and copied to $TOOLS_DIR"
echo ""
echo "Tool binaries:"
ls -lh "$TOOLS_DIR"/{shell_execute,read_file,write_file,gitlab_get_mr,jira_get_issue} 2>/dev/null || true
echo ""
echo "Next steps:"
echo "  1. Set environment variables:"
echo "       export ANTHROPIC_AUTH_TOKEN=\"your-token\""
echo "       export GITLAB_ACCESS_TOKEN=\"your-gitlab-token\""
echo "       export JIRA_ACCESS_TOKEN=\"your-jira-token\""
echo ""
echo "  2. Run hive server:"
echo "       ./bin/hive serve --config examples/hive/.hive.yaml"
echo ""
echo "  3. Execute a task:"
echo "       ./bin/hive execute --config examples/hive/.hive.yaml --task \"Your task\""
