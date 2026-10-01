# gopdf

A minimal, keyboard-driven document viewer backed by MuPDF and configured with Lua.

gopdf provides Vim-style navigation, continuous and single-page layouts, dual-page spreads, text search and selection, outlines, links, persistent sessions and marks, themes, commands, and scriptable keybindings without a permanent toolbar.

## Quick Start

```bash
gopdf file.pdf
```

If no file is provided, gopdf reopens the most recently viewed document.

Useful defaults:

| Key | Action |
|---|---|
| `j` / `k` | Scroll down / up |
| `J` / `K` | Next / previous page |
| `/` / `?` | Search forward / backward |
| `n` / `N` | Next / previous match |
| `o` | Open the document outline |
| `O` | Page overview grid |
| `F5` | Presentation mode |
| `H` | Highlight the selection (pick a colour) |
| `u` / `U` | Undo / redo edits |
| `x` | Delete the annotation under the pointer (right-click for more) |
| `F` | Follow a link by typing its hint |
| `gr` | Open recent files |
| `:` | Open the command prompt |
| `F1` / `g?` | Help: commands, keys, search flags and options |
| `q` | Quit |

## Installation

Download a package for Linux, macOS, or Windows from the [latest release](https://github.com/Aethar01/gopdf/releases/latest).

<details open>
<summary>Linux</summary>

Run the AppImage directly:

```bash
chmod +x gopdf-*-linux-x86_64.AppImage
./gopdf-*-linux-x86_64.AppImage file.pdf
```

Arch-based systems can install [gopdf-git from the AUR](https://aur.archlinux.org/packages/gopdf-git):

```bash
yay -S gopdf-git
```

</details>

<details>
<summary>macOS</summary>

Install the release matching Intel or Apple silicon, or use Homebrew:

```bash
brew install Aethar01/gopdf/gopdf
```

</details>

<details>
<summary>Windows</summary>

The release provides an installer with optional PDF file association and a portable zip.

</details>

## Usage

```bash
gopdf /path/to/file.pdf      # open a document
gopdf --goto 20 file.pdf     # start on page 20
gopdf --config custom.lua file.pdf
gopdf --no-config file.pdf   # built-in defaults only
gopdf --no-plugins file.pdf  # skip plugin loading
gopdf -v                     # print version
gopdf -V                     # enable verbose logs
```

Use `F1`, `g?` or `:help` to browse commands, key bindings, search flags and options; choosing a row runs it or fills in the prompt. `:keybinds` edits key bindings.

### Single instance

Instances are per document. Each window is reachable by anything that opens the
same file.

```bash
gopdf --unique file.pdf                  # reuse the window showing file.pdf
gopdf --unique --goto 42 file.pdf        # ... and go to page 42
gopdf --unique --goto 42:100:250 f.pdf   # ... to a point on page 42
gopdf --goto 42 file.pdf                 # always a new window, opened at 42
gopdf --unique --command "reload-config" f.pdf   # run a command in that window
```

`X` and `Y` are points from the page's top-left corner. `--command` takes a
viewer command as typed after `:`, including commands registered by plugins,
and exits non-zero if the command is unrecognised.

With no window showing the document, `--unique` opens one and applies any
`--goto` and `--command` as it starts. A different document is a different
instance, so two files give two windows. The address follows the document: a
window that switches files becomes reachable under the new one.

Sockets live under `$XDG_RUNTIME_DIR/gopdf` on Linux, the per-user temporary
directory on macOS, and `%LOCALAPPDATA%\gopdf` on Windows. They are mode 0600
and named by a hash of the document path. A socket left by a crash is reclaimed
on the next start.

## Configuration

Configuration is optional and written in Lua. Start with [`config.example.lua`](./config.example.lua), or create a small file containing only the values and bindings you want to change. Reload it with `:reload-config`.

The first existing configuration file for the current platform is loaded:

| Platform | Location |
|---|---|
| Any | Path passed with `--config` |
| Linux | `~/.config/gopdf/config.lua` |
| Linux | `$XDG_CONFIG_HOME/gopdf/config.lua` |
| Linux | Each `$XDG_CONFIG_DIRS/gopdf/config.lua` |
| Linux | `/etc/xdg/gopdf/config.lua` |
| macOS | `~/Library/Application Support/gopdf/config.lua` |
| macOS | `~/.config/gopdf/config.lua` |
| Windows | `%APPDATA%\gopdf\config.lua` |

### Themes

Colours, the UI font and the shape of panels come from one table, `gopdf.theme`. Fields left out keep the values of the base theme, so a theme can be as small as you like:

```lua
gopdf.theme = {
  base = "birch",              -- moss (the default), birch or classic
  accent = "#335533",
  alt = { accent = "#88aa88" }, -- colours for alternate-color mode
  font = { family = "Iosevka", size = 14 },
  status_bar_style = "pill",   -- float the status bar over the page
}
```

A theme can live in its own file. `require` looks beside `config.lua` first, with dots naming directories, so `themes/forest.lua` is `require("themes.forest")`:

```lua
-- themes/forest.lua, next to config.lua
return { base = "moss", accent = "#2f5d3a", alt = { accent = "#8fbf8f" } }

-- config.lua
gopdf.theme = require("themes.forest")
```

Tweak a single field with `gopdf.theme.radius = 0` or, at runtime, `:set theme.radius=0`. With no `font.family` the UI uses the system's interface font. The [reference](https://aethar01.github.io/gopdf_docs/) lists every field.

To go further, each piece of the UI, such as a panel, a menu row, the status bar or a link hint, is an element with its own fill, border, shadows, padding and shape. A shape can be any path, written as SVG path data or drawn by a Lua function, and an element's `draw` function can draw its box however it likes:

```lua
gopdf.theme.elements = {
  panel = { radius = 14, shadow = "0 8 24 shadow" },
  row_selected = { fill = "accent", text = "panel" },
  status_left = { shape = "M0,0 H100%-8 L100%,50% L100%-8,100% H0 Z" },
  hint = {
    draw = function(canvas, box, state)
      canvas:default()
      canvas:stroke("M0,100% H100%", "accent", 2) -- underline each hint
    end,
  },
}
```

Interactive keybinding changes are stored in `autogen.lua`. It is loaded before `config.lua`, so explicit user configuration takes precedence.

Session data is stored in `session.sqlite` under the platform application-data directory:

| Platform | Location |
|---|---|
| Linux | `$XDG_DATA_HOME/gopdf` or `~/.local/share/gopdf` |
| macOS | `~/Library/Application Support/gopdf` |
| Windows | `%APPDATA%\gopdf` |

## Documentation

The [documentation site](https://aethar01.github.io/gopdf_docs/) covers:

- Configuration options and defaults
- Commands and search flags
- Lua functions and tables
- Bindable actions and default keys

The site provides documentation for the current `git` branch and an immutable snapshot for each tagged release. Reference content and the example configuration are generated from the same registrations used by the application:

```bash
go generate ./...
```

## Building

Requirements:

- Go 1.25+
- MuPDF 1.25.6+
- SDL3
- pkg-config/pkgconf
- A C compiler supported by CGO

```bash
go build
go test ./...
```

On Windows, install the dependencies from MSYS2 UCRT64:

```bash
pacman -S --needed mingw-w64-ucrt-x86_64-go mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-pkgconf mingw-w64-ucrt-x86_64-sdl3 mingw-w64-ucrt-x86_64-mupdf
go build -o gopdf.exe
```

## License

gopdf is licensed under the [AGPL](./LICENSE).

It links against [MuPDF](https://mupdf.com/), which is licensed under the AGPL unless you have a separate commercial license.
