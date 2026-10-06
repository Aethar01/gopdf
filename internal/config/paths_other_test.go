//go:build !linux

package config

import "testing"

// isolateSystemDir does nothing where there is no directory of plugins and
// themes installed for every user.
func isolateSystemDir(t *testing.T) { t.Helper() }
