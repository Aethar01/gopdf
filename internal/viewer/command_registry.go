package viewer

type commandSpec struct {
	countable bool
}

var builtinCommandSpecs = map[string]commandSpec{
	"next_page":        {countable: true},
	"prev_page":        {countable: true},
	"scroll_down":      {countable: true},
	"scroll_up":        {countable: true},
	"scroll_left":      {countable: true},
	"scroll_right":     {countable: true},
	"next_spread":      {countable: true},
	"prev_spread":      {countable: true},
	"zoom_in":          {countable: true},
	"zoom_out":         {countable: true},
	"search_next":      {countable: true},
	"search_prev":      {countable: true},
	"first_page":       {},
	"last_page":        {},
	"command_mode":     {},
	"search_prompt":    {},
	"goto_page_prompt": {},
}

func isCountableAction(action string) bool {
	return builtinCommandSpecs[action].countable
}
