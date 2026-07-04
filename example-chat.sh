#!/usr/bin/env bash
# Builds and runs the chat mock TUI example. Must be run in a real
# interactive terminal (raw mode requires a TTY on stdin).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
go run ./_examples/chat
