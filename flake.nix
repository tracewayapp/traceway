{
  description = "traceway — error tracking and monitoring platform (backend, frontend, docs)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Toolchain versions are derived from the manifests rather than
        # restated here, so bumping backend/go.mod or frontend/package.json
        # moves the dev shell with it and the two cannot drift apart.
        goVersion = builtins.match ".*\ngo ([0-9]+)\\.([0-9]+)\\.[0-9]+.*" (
          builtins.readFile ./backend/go.mod
        );
        go = pkgs."go_${builtins.elemAt goVersion 0}_${builtins.elemAt goVersion 1}";

        frontendPackage = builtins.fromJSON (builtins.readFile ./frontend/package.json);
        nodeMajor = builtins.head (builtins.match "([0-9]+).*" frontendPackage.engines.node);
        nodejs = pkgs."nodejs_${nodeMajor}";

        goTools = [
          go
        ]
        ++ (with pkgs; [
          gopls
          gotools
          delve
          golangci-lint
          gofumpt
          govulncheck
          gotestsum
        ]);

        jsTools = [ nodejs ];

        sharedTools = with pkgs; [
          gh
          git
          jq
          just
        ];

        # The banner is gated on an interactive shell: `nix develop --command`
        # evaluates the hook too, and an unconditional echo lands on stdout
        # ahead of the command's own output, so `x=$(nix develop --command ...)`
        # would capture it. Versions are interpolated at eval time rather than
        # probed, so entering a shell does not fork go/node.
        #
        # Nothing here exports backend configuration: the backend supplies its
        # own defaults, so a dev shell that set them would only shadow
        # backend/.env (godotenv.Load never overwrites an already-set variable)
        # and would drift from the code it duplicates.
        mkShell =
          {
            name,
            packages,
            banner ? "",
          }:
          pkgs.mkShell {
            inherit name packages;
            shellHook = ''
              if [[ $- == *i* ]]; then
                echo "traceway dev shell: ${name}"
                ${banner}
              fi
            '';
          };

        goBanner = ''echo "  go     ${go.version}"'';
        nodeBanner = ''echo "  node   ${nodejs.version}"'';

        goShell = mkShell {
          name = "backend";
          packages = goTools ++ sharedTools;
          banner = ''
            ${goBanner}
            echo "  run    cd backend && go run ./cmd/traceway"
            echo "  duckdb go build -tags telemetry_duckdb ./cmd/traceway"
          '';
        };
      in
      {
        devShells = {
          default = mkShell {
            name = "default";
            packages = goTools ++ jsTools ++ sharedTools;
            banner = ''
              ${goBanner}
              ${nodeBanner}
              echo "  commands: see the Quick Start table in CLAUDE.md"
            '';
          };

          backend = goShell;

          frontend = mkShell {
            name = "frontend";
            packages = jsTools ++ sharedTools;
            banner = nodeBanner;
          };

          # cargo builds liboxc_shim.a; the Go toolchain then consumes it via
          # `go build -tags oxc`. No node — the shim never touches it.
          oxc = mkShell {
            name = "oxc";
            packages =
              goTools
              ++ sharedTools
              ++ (with pkgs; [
                cargo
                rustc
              ]);
            banner = ''
              ${goBanner}
              echo "  cargo  ${pkgs.cargo.version}"
              echo "  build  ./scripts/build-oxc-shim.sh"
            '';
          };
        };

        formatter = pkgs.nixfmt;
      }
    );
}
