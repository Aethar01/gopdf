package config

func init() {
	configOptions["copy_on_select"] = boolOption(
		"Copy selected text to the clipboard when the mouse selection is released.",
		func(c *Config) bool { return c.CopyOnSelect },
		func(c *Config, v bool) { c.CopyOnSelect = v },
	)
}
