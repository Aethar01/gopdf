# gopdf

MuPDF-backend PDF viewer written in Go with Lua configuration.

## Requirements

- [libmupdf](https://mupdf.com/)
- [SDL2](https://libsdl.org/)

## Installation

```bash
go build
```

## Usage

```bash
gopdf /path/to/file.pdf      # open file
gopdf --page 20 file.pdf     # start at page 20
gopdf --config custom.lua file.pdf
```

## Configuration

### Config File Locations (in order)

| Priority | Path |
|----------|------|
| 1 (highest) | `--config <path>` argument |
| 2 | `~/.config/gopdf/config.lua` |
| 3 | `$XDG_CONFIG_HOME/gopdf/config.lua` |
| 4 | `$XDG_CONFIG_DIRS/gopdf/config.lua` |
| 5 (lowest) | `/etc/xdg/gopdf/config.lua` |

Start from [`config.lua.example`](./config.lua.example).

### Options

```lua
gopdf.options.dual_page = true           -- boolean
gopdf.options.first_page_offset = true  -- boolean
gopdf.options.render_mode = "continuous" -- "continuous" or "single"
gopdf.options.mouse_text_select = true  -- boolean
gopdf.options.alt_colors = true         -- boolean
gopdf.options.page_gap_vertical = 10    -- integer (px)
gopdf.options.page_gap_horizontal = 10  -- integer (px)

-- Colors (0-255)
gopdf.options.foreground = { 255, 255, 255 }
gopdf.options.background = { 20, 20, 20 }
gopdf.options.alt_foreground = { 0, 0, 0 }
gopdf.options.alt_background = { 235, 235, 235 }
gopdf.options.highlight_foreground = { 0, 0, 0 }
gopdf.options.highlight_background = { 255, 224, 102 }
```

### Keybindings

```lua
-- Keyboard
gopdf.bind("j", gopdf.scroll_down)
gopdf.bind("J", gopdf.next_page)
gopdf.bind_mouse("wheel_down", gopdf.scroll_down)
gopdf.bind_mouse("<C-wheel_up>", gopdf.zoom_in)

-- With custom callbacks
bind("h", function()
  gopdf.next_page()
  message("moved to page " .. gopdf.page())
end)
```

Unbind: `gopdf.unbind("j")`, `gopdf.unbind_mouse("wheel_down")`

### Lua API

**Viewer State**

| Function | Description |
|----------|-------------|
| `gopdf.page()` | Current page number |
| `gopdf.page_count()` | Total pages |
| `gopdf.goto_page(n)` | Jump to page n |
| `gopdf.mode()` | Current view mode |

**Display**

| Function | Description |
|----------|-------------|
| `gopdf.zoom()` / `gopdf.set_zoom(n)` | Get/set zoom level |
| `gopdf.fit_mode()` / `gopdf.set_fit_mode("width"\|"page")` | Fit mode |
| `gopdf.rotation()` / `gopdf.set_rotation(deg)` | Rotation (0/90/180/270) |
| `gopdf.fullscreen()` / `gopdf.set_fullscreen(bool)` | Fullscreen |
| `gopdf.status_bar_visible()` / `gopdf.set_status_bar_visible(bool)` | Status bar |

**Search**

| Function | Description |
|----------|-------------|
| `gopdf.search(query[, backward])` | Search document |
| `gopdf.search_query()` | Current search term |
| `gopdf.search_match_index()` | Current match (1-indexed) |
| `gopdf.search_match_count()` | Total matches |

**Actions**

```lua
gopdf.scroll_down()   gopdf.scroll_up()
gopdf.next_page()     gopdf.prev_page()
gopdf.toggle_dual_page()
gopdf.toggle_first_page_offset()
gopdf.toggle_render_mode()
gopdf.toggle_alt_colors()
gopdf.fit_width()     gopdf.fit_page()
gopdf.reload_config() gopdf.quit()
```

**Utilities**

```lua
message("text")              -- Show status message
command(":fit width")        -- Execute command
gopdf.clear_pending_keys()   -- Clear queued keys
```

**Cache**

```lua
gopdf.cache.limit()          -- Current cache limit
gopdf.cache.set_limit(n)     -- Set limit (MB)
gopdf.cache.clear()          -- Clear cache
```

**Document Metadata** (available during config load)

| Property | Description |
|----------|-------------|
| `gopdf.document.name` | Filename |
| `gopdf.document.path` | Full path |
| `gopdf.document.extension` | File extension |
| `gopdf.document.page_count` | Total pages |
| `gopdf.document.size_bytes` | File size |
| `gopdf.document.exists` | File exists |

## Default Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll down / up |
| `h` / `l` | Scroll left / right |
| `J` / `K` | Next / previous page |
| `gg` / `G` | First / last page |
| `Ng` | Jump to page N |
| `Nj` | Scroll N steps |
| `NJ` | Jump N pages/spreads |
| `m` | Toggle continuous/single page |
| `d` | Toggle dual-page mode |
| `co` | Toggle first-page offset |
| `tb` | Toggle alternate colors |
| `s` | Toggle status bar |
| `+` / `-` / `0` | Zoom in / out / reset |
| `w` / `z` | Fit width / fit page |
| `r` / `R` | Rotate clockwise / counter-clockwise |
| `g` | Page prompt |
| `/` / `?` | Search forward / backward |
| `n` / `N` | Next / previous match |
| `:` | Command prompt |
| `f` | Fullscreen |
| `q` | Quit |
| Mouse wheel | Scroll |
| `Ctrl` + wheel | Zoom |
| Left-drag | Text selection (copies on release) |

## Commands

| Command | Description |
|---------|-------------|
| `:page N` | Jump to page N |
| `:search <text>` | Search document |
| `:N` | Jump to page N |
| `:fit width` | Fit to width |
| `:fit page` | Fit to page |
| `:mode continuous` | Continuous scroll |
| `:mode single` | Single page mode |
| `:colors normal` | Normal colors |
| `:colors alt` | Alternate colors |
| `:set <option>!` | Toggle boolean option |
| `:reload-config` | Reload config file |
| `:quit` | Exit |

## License

gopdf is licensed under [AGPL](./LICENSE)

Links against [MuPDF](https://mupdf.com/), which is licensed under AGPL unless you have a separate commercial license.
