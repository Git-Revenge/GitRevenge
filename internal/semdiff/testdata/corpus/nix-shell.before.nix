{ pkgs ? import <nixpkgs> {} }:
pkgs.mkShell {
  buildInputs = [ pkgs.go pkgs.nodejs ];
  LEVEL = "debug";
}
