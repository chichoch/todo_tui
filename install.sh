#!/bin/sh
set -e

BIN_NAME="todo-tui"
LOCAL_APP_DIR="$HOME/.todo-tui/bin"
LOCAL_BIN_DIR="$HOME/.local/bin"
SYSTEM_BIN_DIR="/usr/bin"

usage() {
    echo "Usage: $0 [--local|--system]"
}

MODE="local"
if [ "$#" -gt 1 ]; then
    usage
    exit 1
fi

if [ "$#" -eq 1 ]; then
    case "$1" in
        --local|local)
            MODE="local"
            ;;
        --system|system)
            MODE="system"
            ;;
        *)
            usage
            exit 1
            ;;
    esac
fi

TMP_BIN="$(mktemp "${TMPDIR:-/tmp}/${BIN_NAME}.XXXXXX")"
trap 'rm -f "$TMP_BIN"' EXIT INT TERM

echo "Building $BIN_NAME..."
go build -o "$TMP_BIN" .

if [ "$MODE" = "system" ]; then
    echo "Installing binary to $SYSTEM_BIN_DIR/$BIN_NAME (sudo may prompt for your password)..."
    sudo install -m 0755 "$TMP_BIN" "$SYSTEM_BIN_DIR/$BIN_NAME"
    echo "Done."
    exit 0
fi

CONFIG_DIR="$HOME/.config/todo-tui"
CONFIG_FILE="$CONFIG_DIR/todo-tui.conf"

echo "Installing binary to $LOCAL_APP_DIR/$BIN_NAME..."
mkdir -p "$LOCAL_APP_DIR" "$LOCAL_BIN_DIR"
install -m 0755 "$TMP_BIN" "$LOCAL_APP_DIR/$BIN_NAME"
ln -sf "$LOCAL_APP_DIR/$BIN_NAME" "$LOCAL_BIN_DIR/$BIN_NAME"

if [ ! -f "$CONFIG_FILE" ]; then
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    mkdir -p "$CONFIG_DIR"
    cp "$SCRIPT_DIR/todo-tui.conf" "$CONFIG_FILE"
    echo "Installed default config to $CONFIG_FILE"
else
    echo "Config already exists at $CONFIG_FILE, skipping."
fi

case ":$PATH:" in
    *":$LOCAL_BIN_DIR:"*) ;;
    *)
        echo "Note: $LOCAL_BIN_DIR is not in your PATH. Add it to your shell's config."
        ;;
esac

echo "Done."
