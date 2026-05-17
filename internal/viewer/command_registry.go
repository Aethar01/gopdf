package viewer

import "gopdf/internal/actions"

func isCountableAction(action string) bool {
	return actions.IsCountable(action)
}
