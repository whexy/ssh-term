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

    # wasm toolchain — required to rebuild @wterm/ghostty's ghostty-vt.wasm
    # (ghostty v1.3.1 requires exactly Zig 0.15.x)
    pkgs.zig_0_15

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
