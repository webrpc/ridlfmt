# RIDLFMT

RIDLFMT is a tool for formatting files written in the RIDL format, used by webrpc.

It uses similiar API as `gofmt`

```
ridlfmt -h
usage: ridlfmt [flags] [path...]

    -h    show help
    -s    sort errors by code
    -w    write result to (source) file instead of stdout
```

## Installation

You can install RIDLFMT using `go install`:

```bash
go install github.com/webrpc/ridlfmt@latest
```

## Setting in IDE

### Example for Neovim using [null-ls/none-ls](https://github.com/nvimtools/none-ls.nvim)

Define `.ridl` filetype, so Neovim would know about it (without this the `.ridl` files wouldn't be detected)

```lua
vim.cmd('autocmd BufRead,BufNewFile *.ridl set filetype=ridl')
```

Define custom source and register it same as builtins:

```lua
local ridl_forrmater = {
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
