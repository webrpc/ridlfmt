# RIDLFMT

RIDLFMT is a tool for formatting files written in the RIDL format, used by webrpc.

It uses similiar API as `gofmt`

```
ridlfmt -h
usage: ridlfmt [flags] [path...]

    Directory paths format .ridl files recursively (also accepts ./...).

    -h              show help
    -s              sort errors by code
    -v, -version    show version
    -w              write result to (source) file instead of stdout
```

## Installation

You can install RIDLFMT using `go install`:

```bash
go install github.com/webrpc/ridlfmt@latest
```

## Formatting files and directories

Like `gofmt`, RIDLFMT prints formatted output by default. Use `-w` to update
files in place. Directory arguments recursively select `.ridl` files, skipping
files whose names start with a dot, like `gofmt`. Explicit
file arguments are formatted regardless of their extension. `./...` and
`directory/...` are also supported as recursive directory shortcuts.

```bash
ridlfmt schema.ridl     # Print a formatted file
ridlfmt .               # Print all .ridl files recursively
ridlfmt -w ./...        # Format all .ridl files recursively in place
ridlfmt -w -s schemas   # Format a directory and sort errors by code
```

With no paths, RIDLFMT reads from stdin. Explicit paths take precedence over stdin.

## REST routes

A method can declare zero or more REST routes with `+ VERB /path` lines.
Routes use absolute paths, and `basepath` at the top level sets the shared prefix.

```ridl
basepath = /v1

service Intents
  - CommitIntent(CommitIntentRequest) => (CommitIntentResponse)
      + POST /intents/{intentId}
      + POST /Intents/CommitIntent     # backward-compatible with pre-REST API route.
```

RIDLFMT normalizes the route to `    + POST /intents/{intentId}` and keeps the routes in their written order.

## Setting in IDE

### VSCode

Install these extensions:

1. [RIDL syntax](https://marketplace.visualstudio.com/items?itemName=XanderAppWorks.vscode-webrpc-ridl-syntax)
   - Needed for recognition of `.ridl` filetype and as a bonus it provides syntax highlighting.
2. [Custom Local Formatters](https://marketplace.visualstudio.com/items?itemName=jkillian.custom-local-formatters)

If the extensions can't be found, install them manually: [Stack overflow: How to install VS code extension manually?](https://stackoverflow.com/questions/42017617/how-to-install-vs-code-extension-manually)

Add this to `settings.json`

```json
"customLocalFormatters.formatters": [
    {
        "command": "ridlfmt -s",
        "languages": ["ridl"]
    }
]
```

Flag `-s` is for sorting errors.

Now you should be able to format `.ridl`, right click and `Format Document`
If you want to format on save, use this settings, but it is global

```json
"editor.formatOnSave": true,

```

NOTE: If it doesn't work, check the logs (`Developer: Show logs...` -> `Extension Host`), if you see this error: `/bin/sh: line 1: ridlfmt: command not found` then `ridlfmt` is not seen by `/bin/sh`, you can copy the binary there with this command:

```bash
sudo cp $(go env GOPATH)/bin/ridlfmt /usr/local/bin/
```

### Neovim using [null-ls/none-ls](https://github.com/nvimtools/none-ls.nvim)

Define `.ridl` filetype, so Neovim would know about it (without this the `.ridl` files wouldn't be detected)

```lua
vim.cmd('autocmd BufRead,BufNewFile *.ridl set filetype=ridl')
```

Define custom source and register it same as builtins formatters:

```lua
local ridl_formatter = {
    name = "ridlfmt",
    filetypes = { "ridl" },
    method = null_ls.methods.FORMATTING,
    generator = null_ls.formatter({
        command = "ridlfmt",
        args = { "-s" },
        to_stdin = true,
        from_stderr = true,
    }),
}
```
