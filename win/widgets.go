package main

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

type FancyProgressBar struct {
	widget.BaseWidget
	value         float64
	indeterminate bool
	shimmer       float32
	anim          *fyne.Animation
	animating     bool
}

func NewFancyProgressBar() *FancyProgressBar {
	bar := &FancyProgressBar{value: 0}
	bar.ExtendBaseWidget(bar)
	return bar
}

func (p *FancyProgressBar) SetValue(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.value = v
	p.ensureAnim()
	p.Refresh()
}

func (p *FancyProgressBar) SetIndeterminate(indeterminate bool) {
	if p.indeterminate == indeterminate {
		return
	}
	p.indeterminate = indeterminate
	p.ensureAnim()
	p.Refresh()
}

func (p *FancyProgressBar) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(colSurfaceAlt)
	bg.CornerRadius = 6
	fill := canvas.NewRectangle(colPrimary)
	fill.CornerRadius = 6
	shine := canvas.NewLinearGradient(color.NRGBA{R: 255, G: 255, B: 255, A: 0}, color.NRGBA{R: 255, G: 255, B: 255, A: 70}, 90)
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = colBorder
	border.StrokeWidth = 1
	border.CornerRadius = 6

	return &fancyProgressRenderer{
		bar:    p,
		bg:     bg,
		fill:   fill,
		shine:  shine,
		border: border,
		objects: []fyne.CanvasObject{
			bg,
			fill,
			shine,
			border,
		},
	}
}

func (p *FancyProgressBar) ensureAnim() {
	shouldRun := p.indeterminate || (p.value > 0 && p.value < 1)
	if p.anim == nil {
		p.anim = fyne.NewAnimation(1400*time.Millisecond, func(f float32) {
			p.shimmer = f
			p.Refresh()
		})
		p.anim.Curve = fyne.AnimationLinear
		p.anim.RepeatCount = fyne.AnimationRepeatForever
	}
	if shouldRun && !p.animating {
		p.animating = true
		p.anim.Start()
		return
	}
	if !shouldRun && p.animating {
		p.animating = false
		p.anim.Stop()
	}
}

type fancyProgressRenderer struct {
	bar     *FancyProgressBar
	bg      *canvas.Rectangle
	fill    *canvas.Rectangle
	shine   *canvas.LinearGradient
	border  *canvas.Rectangle
	objects []fyne.CanvasObject
}

func (r *fancyProgressRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.border.Resize(size)

	fillWidth := float32(size.Width) * float32(r.bar.value)
	if r.bar.indeterminate {
		fillWidth = size.Width
	}
	if fillWidth < 0 {
		fillWidth = 0
	}
	r.fill.Resize(fyne.NewSize(fillWidth, size.Height))

	if fillWidth <= 1 {
		r.shine.Hide()
		return
	}

	shimmerWidth := float32(size.Width) * 0.28
	if shimmerWidth > fillWidth && !r.bar.indeterminate {
		shimmerWidth = fillWidth
	}
	if shimmerWidth < 1 {
		r.shine.Hide()
		return
	}
	r.shine.Show()
	r.shine.Resize(fyne.NewSize(shimmerWidth, size.Height))

	var maxOffset float32
	if r.bar.indeterminate {
		maxOffset = size.Width - shimmerWidth
	} else {
		maxOffset = fillWidth - shimmerWidth
	}
	if maxOffset < 0 {
		maxOffset = 0
	}
	r.shine.Move(fyne.NewPos(maxOffset*r.bar.shimmer, 0))
}

func (r *fancyProgressRenderer) MinSize() fyne.Size {
	return fyne.NewSize(140, 16)
}

func (r *fancyProgressRenderer) Refresh() {
	if r.bar.indeterminate {
		r.fill.FillColor = colPrimarySoft
	} else {
		r.fill.FillColor = colPrimary
	}
	r.fill.Refresh()
	r.bg.FillColor = colSurfaceAlt
	r.bg.Refresh()
	r.border.StrokeColor = colBorder
	r.border.Refresh()
	r.Layout(r.bar.Size())
	canvas.Refresh(r.bar)
}

func (r *fancyProgressRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *fancyProgressRenderer) Destroy() {}
