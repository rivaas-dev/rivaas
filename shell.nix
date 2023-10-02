# use a pinned version of nixpkgs (nixos-23.05) for reproducability
{ pkgs ? import
  (fetchTarball "https://github.com/NixOS/nixpkgs/archive/nixos-23.05.tar.gz")
  { } }:
pkgs.mkShell {
  name = "dev-environment";
  buildInputs = with pkgs; [ go mage open-policy-agent httpie ];
  shellHook = ''
    echo "[!] Build the project:"
    echo "$ mage build"
    echo;
    echo "[!] Run the project:"
    echo "$ mage run"
    echo;
    echo "[!] Test the project:"
    echo "$ mage test"
    echo;
    echo "[!] Lint the project:"
    echo "$ mage lint"
    echo;
    echo "[!] Run scripts inside the '/scripts' directory."
    echo "[!] For instance, get list of policies:"
    echo "$ ./scripts/list-policies.sh"
    echo;
  '';
}
