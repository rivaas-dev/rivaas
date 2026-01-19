{
  description = "Admin API development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };
      in
      {
        devShells.default = pkgs.mkShell {
          name = "admin-api-dev";

          buildInputs = with pkgs; [
            go
            gopls
            golangci-lint
            open-policy-agent
            httpie
            terraform
          ];
        };
      }
    );
}
