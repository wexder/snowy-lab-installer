{ pkgs ? import <nixpkgs> { } }:
let
  snowyLabInstaller = pkgs.buildGoModule {
    name = "snowy-lab-installer";
    src = ./.;
    vendorSha256 = "sha256-WiEK8nq13mdTFnyxDiMRWM1tb60mlZY0fVmtlH5ZWgw=";
  };

in
pkgs.mkShell
{
  name = "snowy-lab-installer";
  buildInputs = with pkgs; [ go snowyLabInstaller ];
  shellHook = "exec snowy-lab-installer";
}
