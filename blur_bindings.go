package graphics

import (
	"image"
	"math"

	. "github.com/purescript-native/go-runtime"
)

func init() {
	Foreign("Native.Graphics.Raster.Native")["blurImage"] = rasterEffect(func(d Dict) Any {
		values := d["radii"].([]Any)
		radii := make([]int, len(values))
		for i, value := range values {
			radii[i] = integer(value)
		}
		blurRGBA(d["image"].(*image.RGBA), integer(d["width"]), integer(d["height"]), radii)
		return nil
	})
}

// blurRGBA preserves the flat RGBA byte layout used by the raster pixel API.
// Radius planning belongs to PureScript; each supplied radius is one separable
// box blur, with float32 storage between the horizontal and vertical passes.
func blurRGBA(img *image.RGBA, width, height int, radii []int) {
	if width <= 0 || height <= 0 || len(radii) == 0 {
		return
	}
	length := width * height * 4
	pixels := img.Pix[:length]
	scratch := make([]float32, 2*length)
	src, tmp := scratch[:length], scratch[length:]
	for i, value := range pixels {
		src[i] = float32(value)
	}
	for _, radius := range radii {
		blurBoxPass(src, tmp, height, width, width*4, 4, radius)
		blurBoxPass(tmp, src, width, height, 4, width*4, radius)
	}
	for i, value := range src {
		// The old Number addition is float64, not another float32 boundary.
		pixels[i] = uint8(math.Floor(float64(value) + 0.5))
	}
}

func blurBoxPass(src, dst []float32, lines, length, lineStride, step, radius int) {
	if radius <= 0 {
		copy(dst, src)
		return
	}
	norm := float32(1.0 / float64(float32(2*radius+1)))
	for line := range lines {
		for channel := range 4 {
			base := line*lineStride + channel
			// Explicit conversions preserve the original rounding boundaries
			// and prevent Go from fusing a multiply into a later addition.
			total := float32(src[base] * float32(radius+1))
			for x := 1; x <= radius; x++ {
				value := src[base+min(x, length-1)*step]
				total = float32(total + value)
			}
			for x := range length {
				dst[base+x*step] = float32(total * norm)
				add := src[base+min(x+radius+1, length-1)*step]
				drop := src[base+max(x-radius, 0)*step]
				total = float32(total + float32(add-drop))
			}
		}
	}
}
