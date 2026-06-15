{ inputs, pkgs, ... }:
let
  pre-commit-check = import ./checks/pre-commit-check.nix { inherit inputs pkgs; };
in
pkgs.mkShell {
  # Add build dependencies
  packages = [
    # backend
    pkgs.go
    pkgs.gopls

    # frontend
    pkgs.nodejs
    pkgs.pnpm

    # ssh bridge — openssh still useful for key generation / manual testing
    pkgs.openssh
  ];

  # Add environment variables
  env = { };

  shellHook = ''
    ${pre-commit-check.shellHook}

    # Load custom bash code
  '';
}
