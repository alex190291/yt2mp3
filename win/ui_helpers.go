package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

func NewPanel(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(colSurface)
	bg.CornerRadius = 6
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = colBorder
	border.StrokeWidth = 1
	border.CornerRadius = 6

	return container.NewStack(
		bg,
		border,
		container.NewPadded(content),
	)
}

func NewAppBackground() fyne.CanvasObject {
	return canvas.NewRectangle(colBackground)
}
