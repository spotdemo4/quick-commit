#!/usr/bin/env bash

cd "$(git rev-parse --show-toplevel)"
updated=false

echo "updating nix flake"
nix flake update
if ! git diff --exit-code flake.nix; then
    git add flake.nix
    git commit -m "build(nix): updated nix dependencies"
fi

echo "updating go"
go get -u
go mod tidy
if ! git diff --exit-code go.mod go.sum; then
    git add go.mod
    git add go.sum
    git commit -m "build(go): updated go dependencies"
    updated=true
fi

if [ "${updated}" = true ]; then
    echo "updating nix hashes"
    nix-update --flake --version=skip default
    git add flake.nix
    git commit -m "build(nix): updated nix hashes"
fi