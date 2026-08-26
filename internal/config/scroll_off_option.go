package config

func init() {
	configOptions["scroll_off"] = intOption(
		"Minimum number of rows kept visible above and below the selected item in UI menus, like Vim's scrolloff.",
		func(c *Config) int { return c.ScrollOff },
		func(c *Config, v int) { c.ScrollOff = max(0, v) },
	)
}
