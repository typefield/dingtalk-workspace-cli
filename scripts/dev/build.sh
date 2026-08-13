#!/bin/sh
set -eu

# No -buildmode=pie: PIE forces dyld relocation of the 28MB image on every
# launch and is the dominant hot-startup cost (measured median 430ms -> 290ms
# on arm64 macOS). Go's runtime provides its own memory-safety guarantees, so
# the ASLR benefit of PIE for a Go CLI is marginal.
go build -trimpath -ldflags="-s -w" -o dws ./cmd
