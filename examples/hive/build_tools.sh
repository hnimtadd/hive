#!/bin/bash
# Build all required tools for the hive setup

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PUBLIC_DIR="$REPO_ROOT/public"
TOOLS_DIR="$REPO_ROOT/examples/hive/tools"

echo "Building Hive SDK tools..."
echo "Repository root: $REPO_ROOT"
echo "Tools directory: $TOOLS_DIR"
echo ""

# Build shell_execute
echo "Building shell_execute..."
cd "$PUBLIC_DIR/shell_executor_sdk"
go mod tidy >/dev/null 2>&1
go build -o shell_execute main.go
cp shell_execute "$TOOLS_DIR/shell_execute"
echo "shell_execute built and copied"

# Build read_file
echo "Building read_file..."
cd "$PUBLIC_DIR/read_file_sdk"
go mod tidy >/dev/null 2>&1
go build -o read_file main.go
cp read_file "$TOOLS_DIR/read_file"
echo "read_file built and copied"

# Build write_file
echo "Building write_file..."
cd "$PUBLIC_DIR/write_file_sdk"
go mod tidy >/dev/null 2>&1
go build -o write_file main.go
cp write_file "$TOOLS_DIR/write_file"
echo "write_file built and copied"

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
