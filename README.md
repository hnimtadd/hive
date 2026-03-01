# The Hive - Distributed AI Agent Platform

A distributed AI agent platform designed to reduce developer cognitive load and automate project management through natural language CLI commands.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   CLI Client    │───▶│   Redis Queue   │───▶│  Worker Agents  │
│                 │    │                 │    │                 │
│ hive "command"  │    │ • Task Storage  │    │ • Code Editor   │
│ --jira PROJ-123 │    │ • Pub/Sub       │    │ • Test Runner   │
│                 │    │ • Feedback      │    │ • Deployer      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Quick Start

### Option 1: Nix (Recommended)

```bash
# Clone the repository
git clone <repository-url>
cd hive

# Enter development environment (with flakes)
nix develop

# Or for legacy Nix users
nix-shell

# Build and run demo
demo
```

### Option 2: Traditional Setup

#### Prerequisites
- Go 1.21+
- Redis server running on localhost:6379
- SQLite3 (for future persistence layer)

#### Installation

```bash
# Clone the repository
git clone <repository-url>
cd hive

# Install dependencies
go mod tidy

# Build the CLI
go build -o bin/hive cmd/hive/main.go

# Build the agent worker
go build -o bin/agent cmd/agent/main.go
```

### Usage

1. **Start Redis server:**
   ```bash
   redis-server
   ```

2. **Start an agent worker:**
   ```bash
   ./bin/agent
   ```

3. **Submit a task via CLI:**
   ```bash
   ./bin/hive "Update the traffic shift script to deal with 0:100 page" --jira "PROJ-123"
   ```

4. **Check task status:**
   ```bash
   ./bin/hive status <task-id>
   ```

5. **List active tasks:**
   ```bash
   ./bin/hive list
   ```

## Core Components

### Task Structure (`pkg/types/task.go`)

The `HiveTask` struct represents a complete task with:
- Unique ID and Jira integration
- Status tracking (pending → in_progress → completed/failed)
- Progress monitoring and metrics
- Human-in-the-loop feedback support
- Execution context and environment

### Agent Interface (`internal/agent/interface.go`)

The `HiveAgent` interface defines the contract for all agents:
- `Execute()` - Performs the main work
- `ReportStatus()` - Provides real-time updates
- `RequestFeedback()` - Enables human interaction
- `CanHandle()` - Determines task compatibility

### Intent Parser (`internal/parser/intent.go`)

Parses natural language commands to extract:
- Action type (update, fix, add, create, etc.)
- Target (script, function, API, etc.)
- Urgency level
- Technology context

### Redis Integration (`internal/redis/client.go`)

Provides distributed communication via:
- Task queuing (FIFO with `BRPOP`)
- Pub/Sub for real-time updates
- Feedback channels for human-in-the-loop
- Agent heartbeat and discovery

## AI-Powered Feature Development

### **NEW: AI Code Editor Agent**

The Hive now includes an AI-powered agent that uses **Claude via Eino** to handle complete feature development workflows:

```bash
# AI develops complete features with GitLab integration
hive "Add user authentication with JWT tokens and refresh mechanism" \
  --jira "AUTH-123" \
  --gitlab-project 42 \
  --target-branch "main"

# Output:
AI analyzing feature requirements... (10%)
AI analysis complete - medium complexity, 4 files to modify (30%)
AI needs clarification: "Should JWT tokens expire after 1 hour or 24 hours?"
Your response: 1 hour for access tokens, 7 days for refresh tokens
Human feedback incorporated, analysis refined (40%)
Preparing GitLab workspace... (45%)
Workspace ready - branch: hive/feature-a1b2c3d4 (50%)
Generating code for auth/handler.go (1/4) (60%)
Generating code for auth/middleware.go (2/4) (70%)
Generating code for auth/jwt.go (3/4) (80%)
Generating code for auth/models.go (4/4) (85%)
Code generation complete - 4 commits created (90%)
Creating merge request... (95%)
Task completed successfully!

AI-powered feature development completed!
Merge Request: https://gitlab.com/project/merge_requests/123
Statistics:
  - 4 commits created
  - 4 files modified/created
  - ~156 lines of code generated
  - Complexity: medium
```

### **Key Capabilities**
- **Intelligent Analysis**: Claude analyzes requirements and technical approach
- **Human-in-the-Loop**: AI asks clarifying questions when needed
- **GitLab Integration**: Automated branch creation, commits, and MR creation
- **Code Generation**: Production-ready Go code following project conventions
- **Progress Monitoring**: Real-time updates with progress indicators

## Configuration

### **AI & GitLab Setup**

1. **Copy example configuration:**
   ```bash
   cp .hive.example.yaml ~/.hive.yaml
   ```

2. **Set required environment variables:**
   ```bash
   export ANTHROPIC_API_KEY="your-claude-api-key"
   export GITLAB_TOKEN="your-gitlab-personal-access-token"
   ```

3. **Configure your settings in `~/.hive.yaml`:**

```yaml
ai:
  provider: "claude"
  model: "claude-3-5-sonnet-20241022"
  api_key_env: "ANTHROPIC_API_KEY"

gitlab:
  url: "https://gitlab.com"  # or your self-hosted GitLab
  token_env: "GITLAB_TOKEN"
  workspace_dir: "$HOME/.hive/workspace"

agents:
  ai_code_editor:
    enabled: true
    max_tasks: 2
```

### **Usage Examples**

```bash
# Traditional task processing
hive "Fix authentication bug in login handler" --jira "AUTH-456"

# AI-powered feature development
hive "Add rate limiting with Redis backend" \
  --gitlab-project 42 \
  --jira "API-789" \
  --target-branch "develop"

# Check task status
hive status abc123

# List active tasks
hive list
```

## Development

### Nix Development Environment

The project includes a comprehensive Nix flake for reproducible development:

```bash
# Enter development shell
nix develop

# Available development commands in Nix shell:
start-redis    # Start Redis with project-specific config
stop-redis     # Stop Redis server
build-all      # Build both CLI and agent binaries
test-all       # Run all tests
lint          # Run golangci-lint
demo          # Start complete demo environment

# Run specific components
nix run .#hive -- "your command" --jira "TICKET-123"
nix run .#agent

# Build packages
nix build .#hive    # Build CLI
nix build .#agent   # Build agent worker
```

#### Direnv Integration

If you use direnv, the `.envrc` file will automatically load the Nix environment:

```bash
# Allow direnv (first time only)
direnv allow

# Environment loads automatically when entering directory
cd hive  # Environment activates automatically
```

### Adding New Agent Types

1. Implement the `HiveAgent` interface
2. Register agent capabilities
3. Add task routing logic
4. Include in agent worker

### Extending Intent Parser

Add new action patterns and context extraction in `internal/parser/intent.go`.

## Future Roadmap

- [ ] SQLite persistence layer for task history
- [ ] gRPC plugin system using HashiCorp go-plugin
- [ ] Web dashboard for task monitoring
- [ ] Slack/Discord integration for feedback
- [ ] LLM integration for enhanced intent parsing
- [ ] Kanban board synchronization

## License

MIT License
