{
  description = "Passion — climbing training";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (s: f nixpkgs.legacyPackages.${s});
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.nodejs_22
            pkgs.pnpm
            pkgs.postgresql_18
            pkgs.air
          ];

          shellHook = ''
            # Go must use the toolchain nix provides, not one it downloads itself.
            export GOTOOLCHAIN=local

            # initdb inherits the ambient locale, and a mismatched glibc locale
            # archive makes it refuse to run.
            export LC_ALL=C

            export PGDATA="$PWD/.pgdata"
            export PGHOST="$PGDATA"
            export PGDATABASE=passion

            export DATABASE_URL="postgres:///passion?host=$PGDATA"
            export TEST_DATABASE_URL="postgres:///passion_test?host=$PGDATA"
          '';
        };
      });
    };
}
