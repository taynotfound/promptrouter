{
  description = "promptrouter: judge a coding task and route it to the right model";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "promptrouter";
          version = "0.1.0";
          src = ./.;
          vendorHash = null;
          env.CGO_ENABLED = 0;
          subPackages = [ "cmd/route" ];
          ldflags = [ "-s" "-w" "-X" "main.version=0.1.0" ];
          postInstall = ''
            install -Dm644 models.yaml $out/etc/promptrouter/models.yaml
          '';
          meta = with pkgs.lib; {
            description = "Judge a coding task and route it to the right model";
            homepage = "https://github.com/taynotfound/promptrouter";
            license = licenses.mit;
            mainProgram = "route";
          };
        };

        devShells.default = pkgs.mkShell {
          packages = [ pkgs.go pkgs.gopls pkgs.goreleaser pkgs.nfpm ];
        };
      });
}
