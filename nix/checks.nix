# CI checks - run with: nix flake check
{ pkgs, lib, self }:

pkgs.runCommand "rivaas-checks" {
  buildInputs = [ lib.go pkgs.golangci-lint ];
  src = self;
} ''
  export HOME=$(mktemp -d)
  export GOPATH=$HOME/go
  export GOCACHE=$HOME/.cache/go-build
  export GOLANGCI_LINT_CACHE=$HOME/.cache/golangci-lint
  mkdir -p $GOPATH $GOCACHE $GOLANGCI_LINT_CACHE

  cd $src

  echo "==> Running go vet..."
  ${lib.go}/bin/go vet ./...

  echo "==> Running tests..."
  ${lib.go}/bin/go test ./... -count=1

  echo "==> All checks passed!"
  touch $out
''
