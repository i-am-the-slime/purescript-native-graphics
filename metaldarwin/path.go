//go:build darwin && !js

package metaldarwin

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// NativePath owns only the native vector-library resource. Coordinates and
// stroke width have already been transformed by PureScript.
type NativePath struct {
	path vector.Path
}

func NewPath() *NativePath { return &NativePath{} }

func (p *NativePath) MoveTo(x, y float32)         { p.path.MoveTo(x, y) }
func (p *NativePath) LineTo(x, y float32)         { p.path.LineTo(x, y) }
func (p *NativePath) QuadTo(cx, cy, x, y float32) { p.path.QuadTo(cx, cy, x, y) }
func (p *NativePath) CubicTo(c1x, c1y, c2x, c2y, x, y float32) {
	p.path.CubicTo(c1x, c1y, c2x, c2y, x, y)
}
func (p *NativePath) Close() { p.path.Close() }

func (p *NativePath) Fill() ([]ebiten.Vertex, []uint16) {
	return p.path.AppendVerticesAndIndicesForFilling(nil, nil)
}

func (p *NativePath) Stroke(width float32, joinTag, capTag int) ([]ebiten.Vertex, []uint16) {
	join := vector.LineJoinMiter
	switch joinTag {
	case 0:
		join = vector.LineJoinRound
	case 1:
		join = vector.LineJoinBevel
	}
	cap := vector.LineCapButt
	switch capTag {
	case 1:
		cap = vector.LineCapRound
	case 2:
		cap = vector.LineCapSquare
	}
	return p.path.AppendVerticesAndIndicesForStroke(nil, nil, &vector.StrokeOptions{
		Width:    width,
		LineJoin: join,
		LineCap:  cap,
	})
}
