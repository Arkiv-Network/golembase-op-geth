{
  description = "Golem Base L3 Store Prototype";
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";

    systems.url = "github:nix-systems/default";

    rpcplorer = {
      url = "github:Golem-Base/rpcplorer/v0.0.3";
      inputs.systems.follows = "systems";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      systems,
      rpcplorer,
      ...
    }@inputs:
    let
      eachSystem = f: nixpkgs.lib.genAttrs (import systems) (system: f system nixpkgs.legacyPackages.${system});
    in
    {
      packages = eachSystem (
        _system:
        pkgs:
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
            vendorHash = "sha256-+xBV3MAKpTJoscliPQssC6na9DTF0VO43AUlNpadVO4=";

            ldflags = [
              "-s"
              "-w"
            ];

            meta = with lib; {
              description = "";
              homepage = "https://github.com/Golem-Base/op-geth";
              license = licenses.gpl3Only;
              mainProgram = "geth";
            };
          };
        }
      );

      devShells = eachSystem (system: pkgs: {
        default = pkgs.mkShell {
          shellHook = ''
            # Set here the env vars you want to be available in the shell
          '';
          hardeningDisable = [ "all" ];

          packages = with pkgs; [
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
          ] ++ lib.optional pkgs.stdenv.hostPlatform.isLinux [
            # For podman networking
            slirp4netns
          ] ++ [ rpcplorer.packages.${system}.default ] ;
        };
      });
    };
}
