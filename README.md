# TODO-tui

Simple TODO app for terminal. Perfect for your scratchpad. 
TODO-tui uses `TODO-tui.md` files in the current folder for saving the lists.

## Installation

1. Clone this repo
2. Install go: https://go.dev/doc/install
3. `go build .`
4. run `-/install.sh` (Will install it in `~/.local/bin/`)

## Features

#### Config file

TODO-tui supports config files in this location: `~/.config/todo-tui/todo-tui.conf`.
The `install.sh`-file creates it by default.

- Adds support for remote files (With sync functionality)
- History-file support. (Saves the removed items to a local history file optionally specified in the config file.)
- Change filename.

## WORK IN PROGRESS

This is a app in a very early state. 
Ideas for future functionality:
- Provide md-file as a flag? 
- Sorting
- Levels (tabs)
- Group by day
- etc
