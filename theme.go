package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// colorNameHint is the colour of help text under fields.
const colorNameHint fyne.ThemeColorName = "hint"

// blankIcon replaces the tick that valid entries show, which adds noise to every field.
var blankIcon = fyne.NewStaticResource("blank.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24"/>`))

// appTheme adjusts Fyne's default theme for legibility.
type appTheme struct {
	fyne.Theme
	// hideValid hides the tick on valid entries; it's used for the editing tabs only, as dialogs
	// use the same icon on their confirm buttons.
	hideValid bool
}

func newAppTheme(hideValid bool) fyne.Theme {
	return &appTheme{Theme: theme.DefaultTheme(), hideValid: hideValid}
}

func (t *appTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	dark := v != theme.VariantLight
	switch n {
	case colorNameHint:
		if dark {
			return color.NRGBA{0xa8, 0xa8, 0xb0, 0xff}
		}
		return color.NRGBA{0x55, 0x55, 0x5e, 0xff}
	case theme.ColorNameDisabled:
		// Synced and read-only values must stay readable.
		if dark {
			return color.NRGBA{0x8a, 0x8a, 0x92, 0xff}
		}
		return color.NRGBA{0x70, 0x70, 0x78, 0xff}
	}
	return t.Theme.Color(n, v)
}

func (t *appTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	if t.hideValid && n == theme.IconNameConfirm {
		return blankIcon
	}
	return t.Theme.Icon(n)
}

func (t *appTheme) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNameHeadingText {
		return 20 // card titles; the default 24 dominates the small forms
	}
	return t.Theme.Size(n)
}
