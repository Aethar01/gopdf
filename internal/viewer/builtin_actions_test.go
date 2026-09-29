package viewer

import (
	"testing"

	"gopdf/internal/actions"
)

func TestEveryRegisteredActionHasAHandler(t *testing.T) {
	registered := map[string]bool{}
	for _, name := range actions.Names() {
		registered[name] = true
		if builtinActions[name] == nil {
			t.Errorf("action %q has no handler", name)
		}
	}
	for name := range builtinActions {
		if !registered[name] {
			t.Errorf("handler %q is not in the actions registry", name)
		}
	}
}
