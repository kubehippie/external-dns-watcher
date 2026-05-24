{
  description = "Nix flake for development";

  inputs = {
    nixpkgs = {
      url = "github:nixos/nixpkgs/nixos-25.11";
    };

    devenv = {
      url = "github:cachix/devenv";
    };

    flake-parts = {
      url = "github:hercules-ci/flake-parts";
    };

    git-hooks = {
      url = "github:cachix/git-hooks.nix";
    };
  };

  outputs =
    inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        inputs.devenv.flakeModule
        inputs.git-hooks.flakeModule
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
          imports = [
            {
              _module.args.pkgs = import inputs.nixpkgs {
                inherit system;
                config.allowUnfree = true;
              };
            }
          ];

          devenv = {
            shells = {
              default = {
                git-hooks = {
                  hooks = {
                    nixfmt = {
                      enable = true;
                    };

                    gofmt = {
                      enable = true;
                    };
                  };
                };

                languages = {
                  go = {
                    enable = true;
                    package = pkgs.go_1_26;
                  };
                };

                packages = with pkgs; [
                  bash
                  gnumake
                  helm-docs
                  kubebuilder
                  kubernetes-helm
                  nixfmt
                  tilt
                  yamllint
                ];

                env = {
                  CGO_ENABLED = "0";
                };
              };
            };
          };
        };
    };
}
