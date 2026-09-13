package graphics

import "math"

type affine struct{ a, b, c, d, tx, ty float64 }

func identityAffine() affine { return affine{a: 1, d: 1} }
func (m affine) apply(x, y float64) (float64, float64) {
	return m.a*x + m.b*y + m.tx, m.c*x + m.d*y + m.ty
}
func viewAffine(w, h, x, y, vw, vh float64) affine {
	scale := math.Min(w/vw, h/vh)
	return affine{a: scale, d: scale, tx: (w-vw*scale)/2 - x*scale, ty: (h-vh*scale)/2 - y*scale}
}
