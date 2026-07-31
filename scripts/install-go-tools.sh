#!/usr/bin/env bash
set -euo pipefail

# Pin analyzer versions so local results match CI.
go install honnef.co/go/tools/cmd/staticcheck@2025.1.1
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0

printf '%s\n' 'Installed staticcheck 2025.1.1 and golangci-lint v2.4.0.'
printf '%s\n' 'Ensure your Go bin directory is on PATH (go env GOPATH)/bin.'
