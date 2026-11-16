package palette

import "testing"

func TestRGB(t *testing.T) {
	tests := []struct {
		name    string
		r, g, b uint8
	}{
		{"ink", 0x14, 0x21, 0x3D},
		{"surface", 0xFF, 0xFF, 0xFF},
		{"success", 0x15, 0x80, 0x3D},
	}
	for _, tt := range tests {
		r, g, b, err := RGB(tt.name)
		if err != nil || r != tt.r || g != tt.g || b != tt.b {
			t.Errorf("RGB(%q) = %d,%d,%d, %v; want %d,%d,%d", tt.name, r, g, b, err, tt.r, tt.g, tt.b)
		}
	}
	if _, _, _, err := RGB("nope"); err == nil {
		t.Error("RGB of an unknown colour should fail")
	}
}

func TestEveryColourParses(t *testing.T) {
	for name := range hex {
		if _, _, _, err := RGB(name); err != nil {
			t.Error(err)
		}
	}
}
