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
    pkgs.nodePackages.pnpm

    # ssh bridge runtime deps (also needed locally to test)
    pkgs.openssh
    pkgs.sshpass
  ];

  # Add environment variables
  env = { };

  shellHook = ''
    ${pre-commit-check.shellHook}

    # Load custom bash code
  '';
}
