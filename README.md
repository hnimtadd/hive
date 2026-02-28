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

## Example Workflows

### Basic Task Execution

```bash
# Submit task
$ ./bin/hive "Fix the authentication bug in the login handler" --jira "AUTH-456"
Task submitted successfully with ID: abc123

# Monitor progress
⏳ Task in progress: Analyzing code structure (20.0%)
⏳ Task in progress: Identifying target files (40.0%)
⏸️  Task paused - feedback required: Should I proceed with modifying the main auth module?
Your response: yes
⏳ Task in progress: Making code modifications (70.0%)
⏳ Task in progress: Running validation tests (90.0%)
✅ Task completed successfully!
Summary: Fixed authentication bug in login handler. Modified 2 files, changed 8 lines.
```

### Human-in-the-Loop Feedback

When an agent needs clarification:

```bash
🤔 Human input required:
Should I proceed with modifying the main configuration file? This will affect traffic routing.
Your response: yes, but create a backup first
```

## Configuration

Create `~/.hive.yaml` for custom settings:

```yaml
redis:
  addr: "localhost:6379"
  password: ""
  db: 0

agents:
  max_concurrent: 5
  timeout: 300

logging:
  level: "info"
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