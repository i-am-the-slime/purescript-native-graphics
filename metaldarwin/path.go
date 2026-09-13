//go:build darwin && !js

package metaldarwin

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type ebitenVertex = ebiten.Vertex

// Generic path token opcodes:
//
//	1 move x y
//	2 line x y
//	3 quad cx cy x y
//	4 cube c1x c1y c2x c2y x y
//	5 close
const (
	PathOpMove  = 1
	PathOpLine  = 2
	PathOpQuad  = 3
	PathOpCube  = 4
	PathOpClose = 5
)

func (r *Renderer) buildVectorPath(tokens []float32) *vector.Path {
	p := &vector.Path{}
	m := r.top()
	n := len(tokens)
	for i := 0; i < n; {
		switch int(tokens[i]) {
		case PathOpMove:
			x, y := m.apply(tokens[i+1], tokens[i+2])
			p.MoveTo(x, y)
			i += 3
		case PathOpLine:
			x, y := m.apply(tokens[i+1], tokens[i+2])
			p.LineTo(x, y)
			i += 3
		case PathOpQuad:
			cx, cy := m.apply(tokens[i+1], tokens[i+2])
			x, y := m.apply(tokens[i+3], tokens[i+4])
			p.QuadTo(cx, cy, x, y)
			i += 5
		case PathOpCube:
			c1x, c1y := m.apply(tokens[i+1], tokens[i+2])
			c2x, c2y := m.apply(tokens[i+3], tokens[i+4])
			x, y := m.apply(tokens[i+5], tokens[i+6])
			p.CubicTo(c1x, c1y, c2x, c2y, x, y)
			i += 7
		case PathOpClose:
			p.Close()
			i++
		default:
			return p
		}
	}
	return p
}

func (r *Renderer) FillPath(tokens []float32, cr, cg, cb, ca float32) {
	p := r.buildVectorPath(tokens)
	verts, idx := p.AppendVerticesAndIndicesForFilling(nil, nil)
	r.emitIndexed(verts, idx, cr, cg, cb, ca)
}

func (r *Renderer) StrokePath(tokens []float32, width float32, joinTag, capTag int, cr, cg, cb, ca float32) {
	if width <= 0 {
		return
	}
	p := r.buildVectorPath(tokens)
	verts, idx := p.AppendVerticesAndIndicesForStroke(nil, nil, &vector.StrokeOptions{
		Width:    width,
		LineJoin: joinFromTag(joinTag),
		LineCap:  capFromTag(capTag),
	})
	r.emitIndexed(verts, idx, cr, cg, cb, ca)
}

func joinFromTag(t int) vector.LineJoin {
	switch t {
	case 0:
		return vector.LineJoinRound
	case 1:
		return vector.LineJoinBevel
	default:
		return vector.LineJoinMiter
	}
}

func capFromTag(t int) vector.LineCap {
	switch t {
	case 1:
		return vector.LineCapRound
	case 2:
		return vector.LineCapSquare
	default:
		return vector.LineCapButt
	}
}

func pathOpWidth(op int) int {
	switch op {
	case PathOpMove, PathOpLine:
		return 3
	case PathOpQuad:
		return 5
	case PathOpCube:
		return 7
	case PathOpClose:
		return 1
	default:
		return 0
	}
}

func splitPathContours(tokens []float32) [][]float32 {
	var contours [][]float32
	start := 0
	for i := 0; i < len(tokens); {
		if int(tokens[i]) == PathOpMove && i > start {
			contours = append(contours, tokens[start:i])
			start = i
		}
		width := pathOpWidth(int(tokens[i]))
		if width == 0 || i+width > len(tokens) {
			break
		}
		i += width
		if i == len(tokens) && start < i {
			contours = append(contours, tokens[start:i])
		}
	}
	return contours
}

func (r *Renderer) tessellateClip(tokens []float32) []float32 {
	p := r.buildVectorPath(tokens)
	verts, idx := p.AppendVerticesAndIndicesForFilling(nil, nil)
	out := make([]float32, 0, len(idx)*6)
	for _, i := range idx {
		v := verts[i]
		out = append(out, v.DstX, v.DstY, 0, 0, 0, 0)
	}
	return out
}

func (r *Renderer) buildClipDraws(tokens []float32, evenOdd bool) [][]float32 {
	if !evenOdd {
		return [][]float32{r.tessellateClip(tokens)}
	}
	contours := splitPathContours(tokens)
	draws := make([][]float32, 0, len(contours))
	for _, contour := range contours {
		if verts := r.tessellateClip(contour); len(verts) > 0 {
			draws = append(draws, verts)
		}
	}
	return draws
}

func (r *Renderer) emitIndexed(verts []ebitenVertex, idx []uint16, cr, cg, cb, ca float32) {
	if len(r.rrectBatch) > 0 {
		r.Flush()
	}
	a := ca * r.alpha()
	for _, i := range idx {
		v := verts[i]
		r.batch = append(r.batch, v.DstX, v.DstY, cr, cg, cb, a)
	}
}
