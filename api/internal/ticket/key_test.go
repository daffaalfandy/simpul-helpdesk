package ticket

import "testing"

func TestKeys(t *testing.T) {
	if got := FormatKey("HD", 24871); got != "HD-24871" {
		t.Errorf("FormatKey = %q", got)
	}
	if got := NormalizeKey("  hd-24871 "); got != "HD-24871" {
		t.Errorf("NormalizeKey = %q", got)
	}
	valid := []string{"HD-1", "HD-24871", "A-9", "ABCDEFGHIJ-123", "X2-77"}
	invalid := []string{"", "HD", "HD-", "HD-0", "HD-012", "hd-1", "1HD-1", "ABCDEFGHIJK-1", "HD-1x", "HD 1", "HD--1"}
	for _, k := range valid {
		if !ValidKey(k) {
			t.Errorf("ValidKey(%q) = false", k)
		}
	}
	for _, k := range invalid {
		if ValidKey(k) {
			t.Errorf("ValidKey(%q) = true", k)
		}
	}
}
