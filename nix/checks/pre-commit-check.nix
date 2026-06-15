{ inputs, pkgs, ... }:
let
  treefmtEval = inputs.treefmt.lib.evalModule pkgs ../treefmt.nix;
in
inputs.git-hooks.lib.${pkgs.stdenv.hostPlatform.system}.run {
  src = inputs.self;
  hooks = {
    nil.enable = true;
    statix.enable = true;
    treefmt = {
      enable = true;
      package = treefmtEval.config.build.wrapper;
    };
    golangci-lint = {
      enable = true;
      entry = "bash -c 'cd backend && golangci-lint run ./...'";
      pass_filenames = false;
    };
  };
}
