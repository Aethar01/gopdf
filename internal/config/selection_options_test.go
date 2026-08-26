package config

import "testing"

func TestCopyOnSelectDefaultsTrue(t *testing.T) {
	if !Default().CopyOnSelect {
		t.Fatal("expected copy_on_select to default to true")
	}
}

func TestCopyOnSelectOptionCanBeDisabled(t *testing.T) {
	cfg := Default()
	desc, ok := configOptions["copy_on_select"]
	if !ok {
		t.Fatal("expected copy_on_select option to be registered")
	}
	if err := desc.applyText(&cfg, "false"); err != nil {
		t.Fatal(err)
	}
	if cfg.CopyOnSelect {
		t.Fatal("expected copy_on_select=false to update config")
	}
}
