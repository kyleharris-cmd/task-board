#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="$REPO_ROOT/bin"
LOCAL_BIN_DIR="${HOME}/.local/bin"
TARGET="$LOCAL_BIN_DIR/tb"
ZSHRC="${HOME}/.zshrc"

mkdir -p "$BIN_DIR"
mkdir -p "$LOCAL_BIN_DIR"

echo "Building taskboard binary..."
go build -o "$BIN_DIR/taskboard" "$REPO_ROOT/cmd/taskboard"

echo "Installing tb -> $TARGET"
ln -sf "$BIN_DIR/taskboard" "$TARGET"

if ! echo "$PATH" | tr ':' '\n' | grep -Fx "$LOCAL_BIN_DIR" >/dev/null 2>&1; then
  echo "Adding $LOCAL_BIN_DIR to PATH in $ZSHRC"
  {
    echo ""
    echo "# taskboard installer"
    echo "export PATH=\"$LOCAL_BIN_DIR:\$PATH\""
  } >> "$ZSHRC"
  echo "PATH updated in $ZSHRC"
else
  echo "$LOCAL_BIN_DIR already in PATH"
fi

echo "Install complete."
echo "Run: tb --help"
echo "If your shell does not pick up PATH yet, run: source $ZSHRC"
