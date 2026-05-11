-- All supported config options with their current defaults.

gopdf.options.status_bar_visible = true
gopdf.options.mouse_text_select = true
gopdf.options.natural_scroll = false
gopdf.options.alt_colors = false
gopdf.options.outline_initial_depth = 1
gopdf.options.outline_width_percent = 70
gopdf.options.outline_height_percent = 80

gopdf.options.render_mode = "continuous"
gopdf.options.dual_page = false
gopdf.options.first_page_offset = true
gopdf.options.fit_mode = "page"

gopdf.options.page_gap = 0
gopdf.options.spread_gap = 0
gopdf.options.page_gap_vertical = 0
gopdf.options.page_gap_horizontal = 0
gopdf.options.status_bar_height = 28
gopdf.options.ui_font_size = 14
gopdf.options.ui_font_path = ""
gopdf.options.sequence_timeout_ms = 700

-- Status bar content templates
gopdf.status_bar.left = "{message}"
gopdf.status_bar.right = "{page}/{total} {mode} fit={fit} rot={rot} {zoom}"

gopdf.options.background = { 255, 255, 255 }
gopdf.options.page_background = { 255, 255, 255 }
gopdf.options.foreground = { 17, 17, 17 }
gopdf.options.status_bar_color = { 17, 17, 17 }
gopdf.options.alt_background = { 17, 17, 17 }
gopdf.options.alt_page_background = { 17, 17, 17 }
gopdf.options.alt_foreground = { 255, 255, 255 }
gopdf.options.alt_status_bar_color = { 17, 17, 17 }
gopdf.options.highlight_foreground = { 0, 0, 0 }
gopdf.options.highlight_background = { 255, 224, 102 }

-- Default key bindings.

gopdf.bind("j", gopdf.scroll_down)
gopdf.bind("k", gopdf.scroll_up)
gopdf.bind("h", gopdf.scroll_left)
gopdf.bind("l", gopdf.scroll_right)
gopdf.bind("J", gopdf.next_page)
gopdf.bind("K", gopdf.prev_page)
gopdf.bind(" ", gopdf.next_page)
gopdf.bind("<PgDn>", gopdf.next_page)
gopdf.bind("<PgUp>", gopdf.prev_page)
gopdf.bind("gg", gopdf.first_page)
gopdf.bind("G", gopdf.last_page)
gopdf.bind(":", gopdf.command_mode)
gopdf.bind("/", gopdf.search_prompt)
gopdf.bind("?", gopdf.search_prompt_backward)
gopdf.bind("n", gopdf.search_next)
gopdf.bind("N", gopdf.search_prev)
gopdf.bind("d", gopdf.toggle_dual_page)
gopdf.bind("m", gopdf.toggle_render_mode)
gopdf.bind("tb", gopdf.toggle_alt_colors)
gopdf.bind("co", gopdf.toggle_first_page_offset)
gopdf.bind("s", gopdf.toggle_status_bar)
gopdf.bind("f", gopdf.toggle_fullscreen)
gopdf.bind("o", gopdf.outline)
gopdf.bind("<CR>", gopdf.confirm)
gopdf.bind("+", gopdf.zoom_in)
gopdf.bind("=", gopdf.zoom_in)
gopdf.bind("-", gopdf.zoom_out)
gopdf.bind("0", gopdf.reset_zoom)
gopdf.bind("w", gopdf.fit_width)
gopdf.bind("z", gopdf.fit_page)
gopdf.bind("r", gopdf.rotate_cw)
gopdf.bind("R", gopdf.rotate_ccw)
gopdf.bind("g", gopdf.goto_page_prompt)
gopdf.bind("q", gopdf.quit)
gopdf.bind("<Esc>", gopdf.close)
gopdf.bind("<C-i>", gopdf.jump_forward)
gopdf.bind("<C-o>", gopdf.jump_backward)

-- Default mouse bindings.

gopdf.bind_mouse("wheel_up", gopdf.scroll_up)
gopdf.bind_mouse("wheel_down", gopdf.scroll_down)
gopdf.bind_mouse("wheel_left", gopdf.scroll_left)
gopdf.bind_mouse("wheel_right", gopdf.scroll_right)
gopdf.bind_mouse("<C-wheel_up>", gopdf.zoom_in)
gopdf.bind_mouse("<C-wheel_down>", gopdf.zoom_out)
gopdf.bind_mouse("middle_down", gopdf.pan)
