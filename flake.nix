{
  description = "Nix flake for development";

  inputs = {
    nixpkgs = {
      url = "github:NixOS/nixpkgs/nixos-unstable";
    };

    devshell = {
      url = "github:numtide/devshell";
    };

    flake-parts = {
      url = "github:hercules-ci/flake-parts";
    };
  };

  outputs =
    inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        inputs.devshell.flakeModule
      ];

      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      perSystem =
        {
          config,
          self',
          inputs',
          pkgs,
          system,
          ...
        }:
        {
          devshells = {
            default = {
              env = [
                {
                  name = "CGO_ENABLED";
                  value = "0";
                }
              ];

              packages = with pkgs; [
                gnumake
                go
                helm-docs
                kubebuilder
                nixfmt-rfc-style
                yamllint
              ];
            };
          };
        };
    };
}
