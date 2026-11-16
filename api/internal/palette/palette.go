// Package palette mirrors web/src/styles/tokens.css for output that can't read CSS variables:
// notification emails and the SLA report's Excel file and PDF. Keys are the token names without
// the --color- or --chip- prefix.
package palette

import (
	"fmt"
	"strconv"
)

var hex = map[string]string{
	"bg":            "#F5F8FC",
	"surface":       "#FFFFFF",
	"tint":          "#E8EEF7",
	"border":        "#DFE7F2",
	"border-subtle": "#E6EDF6",
	"border-strong": "#CFD9E8",
	"ink":           "#14213D",
	"ink-2":         "#334155",
	"muted":         "#52607A",
	"primary":       "#1D4ED8",
	"on-primary":    "#FFFFFF",
	"success":       "#15803D",
	"danger":        "#DC2626",
	"warning-bg":    "#FDF0CF",
	"warning-fg":    "#6E4D00",
	"danger-fg":     "#9A3412",
	"success-fg":    "#23633A",
	"info-bg":       "#EEF4FF",
	"info-fg":       "#1E3A8A",
}

const (
	hexDigits   = 6
	hexBase     = 16
	channelBits = 8
	channelMask = 0xFF
)

// Hex returns a colour as "#RRGGBB".
func Hex(name string) (string, error) {
	c, ok := hex[name]
	if !ok {
		return "", fmt.Errorf("palette: no colour %q", name)
	}
	return c, nil
}

// RGB returns a colour's red, green and blue channels.
func RGB(name string) (r, g, b uint8, err error) {
	c, err := Hex(name)
	if err != nil {
		return 0, 0, 0, err
	}
	v, err := strconv.ParseUint(c[1:1+hexDigits], hexBase, 3*channelBits)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("palette: colour %q: %w", name, err)
	}
	return uint8(v >> (2 * channelBits) & channelMask), uint8(v >> channelBits & channelMask), uint8(v & channelMask), nil
}
