#!/usr/bin/env bash
# claude-code-sync installer for Unix (macOS/Linux)
set -euo pipefail

REPO="felixisaac/claude-code-sync"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
SCRIPT_NAME="claude-code-sync"

echo "Installing claude-code-sync..."

# Check for age
if ! command -v age &>/dev/null; then
    echo ""
    echo "age is required but not installed. Install it first:"
    echo ""
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "  brew install age"
    elif command -v apt &>/dev/null; then
        echo "  sudo apt install age"
    elif command -v dnf &>/dev/null; then
        echo "  sudo dnf install age"
    elif command -v pacman &>/dev/null; then
        echo "  sudo pacman -S age"
    else
        echo "  See https://github.com/FiloSottile/age#installation"
    fi
    echo ""
    exit 1
fi

# Check for git
if ! command -v git &>/dev/null; then
    echo "git is required but not installed. Please install git first."
    exit 1
fi

# Create install directory
mkdir -p "$INSTALL_DIR"

# Download script
echo "Downloading from GitHub..."
curl -fsSL "https://raw.githubusercontent.com/$REPO/main/$SCRIPT_NAME" -o "$INSTALL_DIR/$SCRIPT_NAME"
chmod +x "$INSTALL_DIR/$SCRIPT_NAME"

# Add to PATH if needed
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo ""
    echo "Add $INSTALL_DIR to your PATH by adding this to your shell config:"
    echo ""
    echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
    echo ""
fi

echo ""
echo "Installed successfully to $INSTALL_DIR/$SCRIPT_NAME"
echo ""
echo "Get started:"
echo "  claude-code-sync init"
echo ""
echo "For help:"
echo "  claude-code-sync help"
