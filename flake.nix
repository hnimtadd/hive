{
  description = "The Hive - Distributed AI Agent Platform";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Define the Go version we want
        go = pkgs.go_1_26;

        # Build the hive CLI
        hive-cli = pkgs.buildGoModule {
          pname = "hive";
          version = "0.1.0";

          src = ./.;

          vendorHash = null; # We'll let Nix figure this out

          subPackages = [ "cmd/hive" ];

          buildInputs = with pkgs; [
            sqlite
          ];

          meta = with pkgs.lib; {
            description = "The Hive - Distributed AI Agent Platform CLI";
            homepage = "https://github.com/hnimtadd/hive";
            license = licenses.mit;
            maintainers = [ ];
          };
        };

        # Build the agent worker
        hive-agent = pkgs.buildGoModule {
          pname = "hive-agent";
          version = "0.1.0";

          src = ./.;

          vendorHash = null;

          subPackages = [ "cmd/agent" ];

          buildInputs = with pkgs; [
            sqlite
          ];

          meta = with pkgs.lib; {
            description = "The Hive Agent Worker";
            homepage = "https://github.com/hnimtadd/hive";
            license = licenses.mit;
            maintainers = [ ];
          };
        };

      in
      {
        # Development shell
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go development
            go
            gopls
            gotools
            go-tools

            # Database
            sqlite
            redis

            # Development tools
            air          # Live reloading for Go
            delve        # Go debugger
            golangci-lint

            # System tools
            curl
            jq

            # Testing
            gotestsum

            # Optional: Docker for containerized Redis
            docker
            docker-compose
          ];

          shellHook = ''
            echo "🐝 Welcome to The Hive development environment!"
            echo ""
            echo "Available commands:"
            echo "  go run cmd/hive/main.go     - Run the CLI"
            echo "  go run cmd/agent/main.go    - Run an agent worker"
            echo "  redis-server                - Start Redis server"
            echo "  air                         - Live reload development"
            echo ""
            echo "Redis should be available at localhost:6379"
            echo "Run 'nix develop --help' for more Nix commands"

            # Set up Go environment
            export GOPATH="$(pwd)/.go"
            export GOCACHE="$(pwd)/.go/cache"
            mkdir -p $GOPATH $GOCACHE

            # Ensure Redis data directory exists
            mkdir -p .redis-data

            # Set project-specific environment variables
            export HIVE_REDIS_ADDR="localhost:6379"
            export HIVE_LOG_LEVEL="info"
          '';
        };

        # Package outputs
        packages = {
          default = hive-cli;
          hive = hive-cli;
          agent = hive-agent;
        };

        # Application outputs for `nix run`
        apps = {
          default = flake-utils.lib.mkApp {
            drv = hive-cli;
            exePath = "/bin/hive";
          };

          hive = flake-utils.lib.mkApp {
            drv = hive-cli;
            exePath = "/bin/hive";
          };

          agent = flake-utils.lib.mkApp {
            drv = hive-agent;
            exePath = "/bin/agent";
          };
        };

        # Development scripts
        devShells.scripts = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            redis
            sqlite
          ];

          shellHook = ''
            # Helper scripts for development

            start-redis() {
              echo "Starting Redis server..."
              redis-server --dir .redis-data --dbfilename hive.rdb --daemonize yes
              echo "Redis started on localhost:6379"
            }

            stop-redis() {
              echo "Stopping Redis server..."
              redis-cli shutdown
              echo "Redis stopped"
            }

            build-all() {
              echo "Building all components..."
              go build -o bin/hive cmd/hive/main.go
              go build -o bin/agent cmd/agent/main.go
              echo "Built: bin/hive, bin/agent"
            }

            test-all() {
              echo "Running all tests..."
              go test -v ./...
            }

            lint() {
              echo "Running linter..."
              golangci-lint run
            }

            demo() {
              echo "Starting demo environment..."
              start-redis
              sleep 2
              echo "Starting agent in background..."
              go run cmd/agent/main.go &
              AGENT_PID=$!
              sleep 2
              echo ""
              echo "Demo ready! Try:"
              echo '  go run cmd/hive/main.go "Update the config file" --jira "DEMO-123"'
              echo ""
              echo "Press Ctrl+C to stop demo"
              trap "kill $AGENT_PID; stop-redis; exit" INT
              wait $AGENT_PID
            }

            echo "🔧 Development scripts loaded!"
            echo "Available functions: start-redis, stop-redis, build-all, test-all, lint, demo"
          '';
        };
      });
}
