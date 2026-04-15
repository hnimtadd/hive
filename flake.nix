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
        go = pkgs.go_1_25;

        # Build the hive CLI
        hive-cli = pkgs.buildGoModule {
          pname = "hive";
          version = "0.1.0";

          src = ./.;

          vendorHash = null; # We'll let Nix figure this out

          subPackages = [ "cmd/hive" ];

          buildInputs = with pkgs; [
            sqlite
            git  # Required for GitLab operations
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
            git  # Required for GitLab operations
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

            # Git operations
            git

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
            echo "Welcome to The Hive development environment!"

            # Set project-specific environment variables
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

        };
      });
}
