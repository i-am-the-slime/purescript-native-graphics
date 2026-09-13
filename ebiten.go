package graphics

// Device-language kernels only. PureScript selects, specializes and caches shaders.

const ebitenCompositeSource = `
//kage:unit pixels
package main
var Masked float
var InvertMask float
var Blend float
func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	base := imageSrc0UnsafeAt(srcPos)
	fg := imageSrc1UnsafeAt(srcPos)
	mask := imageSrc2UnsafeAt(srcPos).a
	mask = mix(mask, 1.0-mask, InvertMask)
	fg *= mix(1.0, mask, Masked)
	if Blend == 1.0 {
		return vec4(mix(base.rgb, vec3(1.0)-base.rgb, fg.a), fg.a+base.a*(1.0-fg.a))
	}
	return fg + base*(1.0-fg.a)
}
`

const ebitenClipSource = `
//kage:unit pixels
package main
var Alpha float
func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	return imageSrc0UnsafeAt(srcPos)*imageSrc1UnsafeAt(srcPos).a*Alpha
}
`

const ebitenLatticeSource = `
//kage:unit pixels
package main
var TilePx float
var MajorEvery float
var MajorHalf float
var MinorHalf float
var Origin vec2
var BgColor vec4
var InkColor vec4
// Derivatives are evaluated before fract, keeping coverage continuous as a
// line crosses device pixels and the minor and major lattices in phase.
func lineCoverage(c vec2, halfPx float) float {
	fw := fwidth(c)
	d := abs(fract(c+vec2(0.5))-vec2(0.5))
	dpx := d/fw
	covX := clamp(halfPx-dpx.x+0.5, 0.0, 1.0)
	covY := clamp(halfPx-dpx.y+0.5, 0.0, 1.0)
	return max(covX, covY)
}
func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	uv := dstPos.xy-Origin
	minor := lineCoverage(uv/TilePx, MinorHalf)
	major := lineCoverage(uv/(TilePx*MajorEvery), MajorHalf)
	return mix(BgColor, InkColor, max(minor, major))
}
`

const ebitenCircleSource = `
//kage:unit pixels
package main
var Scale vec2
var Translation vec2
var Origin vec2
var Tile float
var Radius float
var BgColor vec4
var InkColor vec4
func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	world := (dstPos.xy-Translation)/Scale-Origin
	cell := (fract(world/Tile+vec2(0.5))-vec2(0.5))*Tile
	distance := length(cell)-Radius
	pixel := max(length(fwidth(world)), 0.000001)
	coverage := clamp(0.5-distance/pixel, 0.0, 1.0)
	return mix(BgColor, InkColor, coverage)
}
`

const ebitenBlurSource = `
//kage:unit pixels
package main
var Sigma float
var Direction vec2
func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	sum := vec4(0.0)
	weight := 0.0
	for i := -%d; i <= %d; i++ {
		x := float(i)
		w := exp(-0.5*x*x/(Sigma*Sigma))
		sum += imageSrc0At(srcPos+Direction*x)*w
		weight += w
	}
	return sum/weight
}
`
