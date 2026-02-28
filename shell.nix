# shell.nix - For users who don't use flakes
{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    # Go development
    go_1_21
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
    echo "🐝 Welcome to The Hive development environment (legacy shell.nix)!"
    echo ""
    echo "Note: Consider using 'nix develop' with flakes for better reproducibility"
    echo ""
    echo "Available commands:"
    echo "  go run cmd/hive/main.go     - Run the CLI"
    echo "  go run cmd/agent/main.go    - Run an agent worker"
    echo "  redis-server                - Start Redis server"
    echo "  air                         - Live reload development"
    echo ""
    
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
}