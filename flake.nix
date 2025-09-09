{
  description = "Golem Base L3 Store Prototype";
  inputs = {
    nixpkgs.url = "https://channels.nixos.org/nixos-unstable/nixexprs.tar.xz";

    systems.url = "github:nix-systems/default";

    rpcplorer = {
      url = "github:Golem-Base/rpcplorer/v0.0.3";
      inputs.systems.follows = "systems";
    };
  };

  outputs =
    inputs:
    let
      eachSystem =
        f:
        inputs.nixpkgs.lib.genAttrs (import inputs.systems) (
          system: f system inputs.nixpkgs.legacyPackages.${system}
        );
    in
    {
      packages = eachSystem (
        _system: pkgs:
          let
            inherit (pkgs) lib;
          in
          {
            default = pkgs.buildGoModule {
              name = "gb-op-geth";

              src = ./.;

              subPackages = [
                "cmd/abidump"
                "cmd/abigen"
                "cmd/clef"
                "cmd/devp2p"
                "cmd/ethkey"
                "cmd/evm"
                "cmd/geth"
                "cmd/rlpdump"
                "cmd/utils"
              ];

              proxyVendor = true;
              vendorHash = "sha256-Uj0CIeCDjs1ARpqXmjDUUz+0lvlmAFpQulFZfmKH/+Y=";

              ldflags = [
                "-s"
                "-w"
              ];

              meta = with lib; {
                description = "";
                homepage = "https://github.com/Golem-Base/golembase-op-geth";
                license = licenses.gpl3Only;
                mainProgram = "geth";
              };
            };

            golembase-cli = pkgs.buildGoModule {
              name = "golembase";
              src = ./.;
              subPackages = [ "cmd/golembase" ];
              vendorHash = "sha256-n5JidCrpnqisDRnnT+eAAG7Nof1P3vcDaEs3/WbeqH0=";
              meta = with lib; {
                description = "golembase CLI - Golem Base";
                homepage = "https://github.com/Golem-Base/golembase-op-geth";
                license = licenses.gpl3Only;
                mainProgram = "golembase";
              };
            };
          }
      );

      devShells = eachSystem (
        system: pkgs: {
          default = pkgs.mkShell {
            shellHook = ''
              # Set here the env vars you want to be available in the shell
            '';
            hardeningDisable = [ "all" ];

            packages =
              with pkgs;
              [
                go
                go-tools # staticccheck
                gopls # lsp
                gotools # goimports, ...
                shellcheck
                sqlc
                sqlite
                overmind
                mongosh
                openssl
                goreleaser
              ]
              ++ lib.optional pkgs.stdenv.hostPlatform.isLinux [
                # For podman networking
                slirp4netns
              ]
              ++ [ inputs.rpcplorer.packages.${system}.default ];
          };
        }
      );
    };
}
