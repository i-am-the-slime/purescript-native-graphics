package graphics

import (
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/font"
	"image"
	"image/color"
	"math"
)

const ptPerMm = 72.0 / 25.4

func applyAlphaMask(img, mask *image.RGBA) {
	if len(img.Pix) != len(mask.Pix) {
		return
	}
	for i := 0; i < len(img.Pix); i += 4 {
		alpha := uint16(mask.Pix[i+3])
		img.Pix[i] = uint8(uint16(img.Pix[i]) * alpha / 255)
		img.Pix[i+1] = uint8(uint16(img.Pix[i+1]) * alpha / 255)
		img.Pix[i+2] = uint8(uint16(img.Pix[i+2]) * alpha / 255)
		img.Pix[i+3] = uint8(uint16(img.Pix[i+3]) * alpha / 255)
	}
}

func viewScale(m canvas.Matrix) float64 {
	return math.Sqrt(math.Abs(m[0][0]*m[1][1] - m[0][1]*m[1][0]))
}

func drawCirclePattern(ctx *canvas.Context, vx, vy, vw, vh float64, bg, dot color.NRGBA, tile, dotR, ox, oy float64) {
	ctx.SetStrokeColor(color.Transparent)
	ctx.SetFillColor(bg)
	ctx.DrawPath(vx, vy, canvas.Rectangle(vw, vh))
	cell := ctx.View().Mul(canvas.Identity.Translate(ox, oy)).Mul(canvas.Identity.Scale(tile, tile))
	if pitch := math.Hypot(cell[0][0], cell[1][0]); pitch < 2.0 && pitch > 0.0 {
		s := 2.0 / pitch
		cell = cell.Mul(canvas.Identity.Scale(s, s))
	}
	rUnit := dotR / tile
	pat := canvas.NewHatchPattern(dot, 0, cell, unitCellCircle(rUnit))
	ctx.SetFillPattern(pat)
	ctx.DrawPath(vx, vy, canvas.Rectangle(vw, vh))
}

func unitCellCircle(rUnit float64) canvas.Hatcher {
	dot := canvas.Circle(rUnit)
	return func(x0, y0, x1, y1 float64) *canvas.Path {
		p := &canvas.Path{}
		iStart := int(math.Floor(x0))
		iEnd := int(math.Ceil(x1))
		jStart := int(math.Floor(y0))
		jEnd := int(math.Ceil(y1))
		for j := jStart; j <= jEnd; j++ {
			for i := iStart; i <= iEnd; i++ {
				p = p.Append(dot.Copy().Translate(float64(i)+0.5, float64(j)+0.5))
			}
		}
		return p
	}
}
func joinFromTag(t int) canvas.Joiner {
	switch t {
	case 0:
		return canvas.RoundJoin
	case 1:
		return canvas.BevelJoin
	default:
		return canvas.MiterJoin
	}
}

func capFromTag(t int) canvas.Capper {
	switch t {
	case 1:
		return canvas.RoundCap
	case 2:
		return canvas.SquareCap
	default:
		return canvas.ButtCap
	}
}

func alignFromTag(t int) canvas.TextAlign {
	switch t {
	case 0:
		return canvas.Left
	case 2:
		return canvas.Right
	default:
		return canvas.Center
	}
}
func buildPath(buf []float64) *canvas.Path {
	p := &canvas.Path{}
	n := len(buf)
	for i := 0; i < n; {
		op := buf[i]
		switch {
		case op == 1.0:
			p.MoveTo(buf[i+1], buf[i+2])
			i += 3
		case op == 2.0:
			p.LineTo(buf[i+1], buf[i+2])
			i += 3
		case op == 3.0:
			p.QuadTo(buf[i+1], buf[i+2], buf[i+3], buf[i+4])
			i += 5
		case op == 4.0:
			p.CubeTo(buf[i+1], buf[i+2], buf[i+3], buf[i+4], buf[i+5], buf[i+6])
			i += 7
		case op == 5.0:
			p.Close()
			i++
		default:
			return p
		}
	}
	return p
}
func clamp255(v uint32) uint8 {
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func drawTextCanvas(ctx *canvas.Context, x, y float64, content, family string, size float64, c color.NRGBA, align, baseline int, boldOffset float64) {
	if content == "" {
		return
	}
	registered := lookupFont(family)
	face := registered.canvas.Face(size*ptPerMm, c, canvas.FontRegular, canvas.FontNormal, font.NoHinting)
	offset := size * boldOffset
	width := face.TextWidth(content) + offset
	if align == 2 {
		x -= width
	} else if align != 0 {
		x -= width / 2
	}
	metrics := face.Metrics()
	switch baseline {
	case 1:
		y += metrics.CapHeight / 2
	case 2:
		y += metrics.Ascent
	case 3:
		y -= metrics.Descent
	}
	line := canvas.NewTextLine(face, content, canvas.Left)
	ctx.DrawText(x, y, line)
	if offset != 0 {
		ctx.DrawText(x+offset, y, line)
	}
}
func drawLineLatticeCanvas(ctx *canvas.Context, w, h float64, a []float64) {
	pitch := a[12] * math.Min(w, h)
	major := pitch * a[13]
	if pitch <= 0 || major <= 0 {
		return
	}
	ox := math.Mod(w*a[16], major)
	oy := math.Mod(h*a[17], major)
	ctx.Push()
	defer ctx.Pop()
	ctx.ResetView()
	ctx.SetFillColor(rgba(a[4:]))
	ctx.SetStrokeColor(color.Transparent)
	ctx.DrawPath(0, 0, canvas.Rectangle(w, h))
	ctx.SetFillColor(color.Transparent)
	ctx.SetStrokeColor(rgba(a[8:]))
	draw := func(spacing, width float64) {
		p := &canvas.Path{}
		for x := ox - math.Ceil(ox/spacing)*spacing; x <= w; x += spacing {
			p.MoveTo(x, 0)
			p.LineTo(x, h)
		}
		for y := oy - math.Ceil(oy/spacing)*spacing; y <= h; y += spacing {
			p.MoveTo(0, y)
			p.LineTo(w, y)
		}
		ctx.SetStrokeWidth(width * 2)
		ctx.DrawPath(0, 0, p)
	}
	draw(pitch, a[15])
	draw(major, a[14])
}
