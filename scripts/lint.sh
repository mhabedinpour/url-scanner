#!/bin/bash

set -e

set -o pipefail
echo 'running golangci-lint ...'
golangci-lint run --config .golangci.yml {./internal/...,./cmd/...,./pkg/...} || true
jq -r '.[] | "\(.location.path):\(.location.lines.begin) \(.description)"' gl-code-quality-report.json

