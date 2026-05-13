# TODO-tui

Simple TODO app for terminal. Perfect for your scratchpad. 
TODO-tui uses `TODO-tui.md` files in the current folder for saving the lists.

## Installation

1. Clone this repo
2. Install go: https://go.dev/doc/install
3. `go build .`
4. run `-/install.sh` (Will install it in `~/.local/bin/`)

## Usage

```sh
todo-tui                        # use the config-driven file (default)
todo-tui notes.md               # edit a specific file (ignores file-cmd-save / file-cmd-load)
todo-tui ~/notes/grocery.md     # `~/` is expanded
todo-tui -n                     # disable history-file appends for this session
todo-tui --no-history notes.md  # combine: ad-hoc file + no history
```

Flags must come before the positional file argument. Passing a file argument
forces a plain local edit on that file — `file-cmd-save` and `file-cmd-load`
from the config are ignored. History is still recorded (if configured) unless
`-n` / `--no-history` is also passed.

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
- Sorting
- Levels (tabs)
- Group by day
- etc
