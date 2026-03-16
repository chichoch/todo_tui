#!/bin/sh
set -e

BIN_NAME="todo-tui"
LOCAL_APP_DIR="$HOME/.todo-tui"
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

if [ "$MODE" = "system" ]; then
  if [ -L "$SYSTEM_BIN_DIR/$BIN_NAME" ] || [ -f "$SYSTEM_BIN_DIR/$BIN_NAME" ]; then
    sudo rm "$SYSTEM_BIN_DIR/$BIN_NAME"
    echo "Removed $SYSTEM_BIN_DIR/$BIN_NAME"
  else
    echo "Binary not found at $SYSTEM_BIN_DIR/$BIN_NAME"
  fi

  echo "Done."
  exit 0
fi

if [ -L "$LOCAL_BIN_DIR/$BIN_NAME" ] || [ -f "$LOCAL_BIN_DIR/$BIN_NAME" ]; then
  rm "$LOCAL_BIN_DIR/$BIN_NAME"
  echo "Removed $LOCAL_BIN_DIR/$BIN_NAME"
else
  echo "Binary not found at $LOCAL_BIN_DIR/$BIN_NAME"
fi

if [ -d "$LOCAL_APP_DIR" ]; then
  rm -rf "$LOCAL_APP_DIR"
  echo "Removed $LOCAL_APP_DIR"
fi

echo "Done."
