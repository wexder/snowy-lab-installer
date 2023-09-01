{ pkgs ? import <nixpkgs> { } }:
pkgs.mkShell {
  # nativeBuildInputs is usually what you want -- tools you need to run
  nativeBuildInputs = with pkgs.buildPackages; [
    ngrok
    python310Packages.servefile
    go
  ];
  shellHook =
    ''
      echo Starting Dev server
      # servefile --tar --compression gzip --port 22355 . & ngrok http 22355
    '';

}
