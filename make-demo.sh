#!/bin/bash
# Records demo.gif from demo.tape via vhs (https://github.com/charmbracelet/vhs),
# driving the actual _examples/chat binary in a real recorded terminal.
# Adapted from the eee project's make-demo.sh, simplified: tuigo's chat demo
# is a single instance (no tmux split / clipboard handoff needed like eee's
# two-peer chat demo required).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

VHS="${HOME}/go/bin/vhs"
if [ ! -x "$VHS" ] && command -v vhs >/dev/null 2>&1; then
    VHS="$(command -v vhs)"
fi
if [ ! -x "$VHS" ]; then
    echo "vhs not found (https://github.com/charmbracelet/vhs), skipping demo"
    exit 0
fi

go build -o ./chat-demo-bin ./_examples/chat
trap 'rm -f ./chat-demo-bin' EXIT

"$VHS" demo.tape
ls -lh demo.gif 2>/dev/null || true
