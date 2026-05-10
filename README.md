# gopdf

MuPDF-backed PDF viewer written in Go.

## Features

- `libmupdf` backend through a direct cgo wrapper
- Continuous vertical scrolling with visible-page-only rendering
- Optional single-page rendering mode
- Single-page and dual-page book view
- Toggleable first-page offset for cover-style spreads
- Vim-style default keybindings
- Lua config loaded from standard config locations
- Hideable bottom status bar with vim-like command prompts
- Page jump, zoom, rotation, fit-width, fit-page, fullscreen
- Async native text search with `/` and `?`, highlights, and `n` / `N` navigation

## Build

`libmupdf` must be installed and discoverable through `pkg-config`.

```bash
go build ./...
```

## Run

```bash
go run . /path/to/file.pdf
go run . --page 20 /path/to/file.pdf
go run . --config /path/to/config.lua /path/to/file.pdf
```

## Config Locations

The viewer checks these locations in order:

1. `--config <path>` if provided
2. `~/.config/gopdf/config.lua`
3. `$XDG_CONFIG_HOME/gopdf/config.lua`
4. Each `$XDG_CONFIG_DIRS/gopdf/config.lua`
5. `/etc/xdg/gopdf/config.lua`

Start from `config.lua.example`.

Config files use a `gopdf` Lua namespace.

```lua
gopdf.options.dual_page = true
gopdf.options.first_page_offset = true
gopdf.options.render_mode = "continuous"
gopdf.options.mouse_text_select = true
gopdf.options.foreground = { 0, 0, 0 }
gopdf.options.alt_background = { 20, 20, 20 }
gopdf.options.alt_foreground = { 235, 235, 235 }
gopdf.options.highlight_foreground = { 0, 0, 0 }
gopdf.options.highlight_background = { 255, 224, 102 }
gopdf.options.alt_colors = false
gopdf.options.page_gap_vertical = 0
gopdf.options.page_gap_horizontal = 0
gopdf.bind("j", gopdf.scroll_down())
gopdf.bind("J", gopdf.next_page())
gopdf.bind("m", gopdf.toggle_render_mode())
gopdf.bind("tb", gopdf.toggle_alt_colors())
gopdf.bind_mouse("wheel_down", gopdf.scroll_down())
gopdf.bind_mouse("<C-wheel_up>", gopdf.zoom_in())
gopdf.bind("co", gopdf.toggle_first_page_offset())
```

Available config helpers:

- `gopdf.options.<name> = value`
- `gopdf.bind(keys, gopdf.some_action())`
- `gopdf.bind_mouse(event, gopdf.some_action())`
- `gopdf.unbind(keys)`
- `gopdf.unbind_mouse(event)`

Action helpers currently include names like `gopdf.scroll_down()`, `gopdf.scroll_up()`, `gopdf.scroll_left()`, `gopdf.scroll_right()`, `gopdf.next_page()`, `gopdf.prev_page()`, `gopdf.toggle_dual_page()`, `gopdf.toggle_first_page_offset()`, `gopdf.fit_width()`, `gopdf.fit_page()`, `gopdf.quit()`, and the other built-in viewer actions.

## Default Keys

- `j` / `k`: scroll down / up
- `h` / `l`: scroll left / right
- `J` / `K`: next / previous page jump
- `10g`: jump to page 10
- `20j`: scroll down 20 steps
- `5J`: jump forward 5 pages or spreads
- `m`: toggle continuous / single-page render mode
- `tb`: toggle alternate color mode
- Mouse wheel: scroll
- `Ctrl` + mouse wheel: zoom
- Left-drag text selection copies to clipboard on release
- `gg` / `G`: first / last page
- `:`: command prompt
- `/` / `?`: forward / backward search prompt
- `n` / `N`: repeat search in same / opposite direction
- `d`: toggle dual-page mode
- `co`: toggle first-page offset
- `s`: toggle status bar
- `+` / `-` / `0`: zoom in / out / reset to 100%
- `w` / `z`: fit width / fit page
- `r` / `R`: rotate clockwise / counter-clockwise
- `g`: page prompt
- `f`: fullscreen
- `q`: quit

## Commands

- `:page 42`
- `:search needle`
- `:100`
- `:mode continuous`
- `:mode single`
- `:set render_mode!`
- `:colors normal`
- `:colors alt`
- `:set alt_colors!`
- `:set dual_page!`
- `:set first_page_offset!`
- `:set status_bar!`
- `:fit width`
- `:fit page`
- `:reload-config`
- `:quit`

## Notes

- This project links against MuPDF, which is licensed under AGPL unless you have a separate commercial license.
- Pages are rendered on demand from the visible rows instead of rendering the full document at once.
- `page_gap_vertical` controls spacing between rows and top/bottom margins.
- `page_gap_horizontal` controls spacing between pages in a spread and left/right margins.
- `background` / `foreground` control the normal viewer and status text colors.
- `alt_background` / `alt_foreground` control the alternate palette.
- `highlight_foreground` / `highlight_background` control mouse text highlighting.
- Alternate colors now recolor the rendered PDF pages as well as the viewer chrome.
>>>>>>> a810f56 (first)
