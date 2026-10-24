// Package artwork draws VoltPuff's original little battery-cloud mascot.
package artwork

import (
	"image"
	"image/color"
	"math"
)

type shape func(x, y float64) bool

func circle(cx, cy, r float64) shape {
	return func(x, y float64) bool { return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r }
}

func rounded(x0, y0, x1, y1, r float64) shape {
	return func(x, y float64) bool {
		cx, cy := max(x0+r, min(x, x1-r)), max(y0+r, min(y, y1-r))
		return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
	}
}

func line(x0, y0, x1, y1, width float64) shape {
	return func(x, y float64) bool {
		dx, dy := x1-x0, y1-y0
		t := max(0, min(1, ((x-x0)*dx+(y-y0)*dy)/(dx*dx+dy*dy)))
		return math.Hypot(x-x0-t*dx, y-y0-t*dy) <= width/2
	}
}

func puff(x, y float64) bool {
	return circle(128, 146, 60)(x, y) || circle(84, 151, 39)(x, y) ||
		circle(172, 149, 39)(x, y) || circle(104, 117, 40)(x, y) || circle(151, 119, 38)(x, y)
}

// Icon returns the application icon, or a monochrome menu bar template.
// Coordinates share a 256-unit canvas, so the mascot stays the same at any size.
func Icon(size int, template bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	paint := func(s shape, ink color.NRGBA) {
		for y := range size {
			for x := range size {
				if s((float64(x)+.5)*256/float64(size), (float64(y)+.5)*256/float64(size)) {
					img.SetNRGBA(x, y, ink)
				}
			}
		}
	}
	ink := color.NRGBA{R: 64, G: 47, B: 91, A: 255}
	if template {
		paint(puff, color.NRGBA{A: 255})
		ink = color.NRGBA{}
	} else {
		bg := rounded(8, 8, 248, 248, 54)
		for y := range size {
			for x := range size {
				px, py := (float64(x)+.5)*256/float64(size), (float64(y)+.5)*256/float64(size)
				if bg(px, py) {
					v := (px + py) / 512
					img.SetNRGBA(x, y, color.NRGBA{R: uint8(214 - 35*v), G: uint8(195 - 37*v), B: uint8(255 - 12*v), A: 255})
				}
			}
		}
		paint(func(x, y float64) bool { return puff(x, y-7) }, color.NRGBA{R: 147, G: 116, B: 207, A: 255})
		paint(puff, color.NRGBA{R: 255, G: 250, B: 232, A: 255})
		paint(circle(93, 161, 10), color.NRGBA{R: 249, G: 184, B: 181, A: 255})
		paint(circle(166, 161, 10), color.NRGBA{R: 249, G: 184, B: 181, A: 255})
		paint(circle(57, 65, 5), color.NRGBA{R: 255, G: 245, B: 207, A: 255})
		paint(circle(202, 87, 3), color.NRGBA{R: 255, G: 245, B: 207, A: 255})
	}
	paint(rounded(105, 140, 113, 152, 4), ink)
	paint(rounded(145, 140, 153, 152, 4), ink)
	paint(line(122, 159, 128, 164, 4.5), ink)
	paint(line(128, 164, 135, 159, 4.5), ink)
	bolt := color.NRGBA{R: 247, G: 194, B: 63, A: 255}
	if template {
		bolt = color.NRGBA{A: 255}
	}
	paint(line(139, 47, 126, 64, 9), bolt)
	paint(line(126, 64, 140, 64, 9), bolt)
	paint(line(140, 64, 128, 81, 9), bolt)
	return img
}

// Tray downsamples the template to a crisp antialiased 32px menu bar icon.
func Tray() *image.NRGBA {
	src, dst := Icon(128, true), image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			alpha := 0
			for dy := range 4 {
				for dx := range 4 {
					alpha += int(src.NRGBAAt(x*4+dx, y*4+dy).A)
				}
			}
			dst.SetNRGBA(x, y, color.NRGBA{A: uint8(alpha / 16)})
		}
	}
	return dst
}
