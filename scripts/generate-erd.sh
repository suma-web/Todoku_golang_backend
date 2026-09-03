#!/usr/bin/env sh

set -eu

erd_dir="docs/architecture/erd"
image="ghcr.io/burntsushi/erd:latest"

docker run --rm --platform linux/amd64 -i "$image" --fmt=svg \
  < "$erd_dir/todoku.er" \
  > "$erd_dir/todoku-erd.svg"

docker run --rm --platform linux/amd64 -i "$image" --fmt=pdf \
  < "$erd_dir/todoku.er" \
  > "$erd_dir/todoku-erd.pdf"
