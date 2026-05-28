package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// AppTheme implements fyne.Theme.
type AppTheme struct{}

var _ fyne.Theme = (*AppTheme)(nil)

// -- Palette --
// Restrained desktop utility palette: neutral surfaces, blue action color, clear contrast.
var colBackground = color.NRGBA{R: 17, G: 19, B: 23, A: 255}
var colSurface = color.NRGBA{R: 31, G: 34, B: 39, A: 255}
var colSurfaceAlt = color.NRGBA{R: 42, G: 46, B: 52, A: 255}
var colPrimary = color.NRGBA{R: 72, G: 139, B: 219, A: 255}
var colPrimarySoft = color.NRGBA{R: 72, G: 139, B: 219, A: 120}
var colAccent = color.NRGBA{R: 235, G: 170, B: 70, A: 255}
var colText = color.NRGBA{R: 235, G: 237, B: 241, A: 255}
var colPlacehold = color.NRGBA{R: 151, G: 158, B: 169, A: 255}
var colBorder = color.NRGBA{R: 83, G: 89, B: 99, A: 255}

func (m AppTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colBackground
	case theme.ColorNameButton, theme.ColorNameInputBackground:
		return colSurface
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return colPrimary
	case theme.ColorNameFocus:
		return colAccent
	case theme.ColorNameForeground:
		return colText
	case theme.ColorNamePlaceHolder:
		return colPlacehold
	case theme.ColorNameScrollBar:
		return colBorder
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (m AppTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m AppTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m AppTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameText:
		return 14
	}
	return theme.DefaultTheme().Size(name)
}
