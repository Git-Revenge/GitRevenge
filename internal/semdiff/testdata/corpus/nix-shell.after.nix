{ pkgs ? import <nixpkgs> {} }:
pkgs.mkShell {
  buildInputs = [
    pkgs.go
    pkgs.nodejs_22
  ];
  LEVEL = "debug";
}
