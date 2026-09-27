package graphics

import (
	"image"
	"math"

	. "github.com/purescript-native/go-runtime"
)

func init() {
	g := Foreign("Native.Graphics.Raster.Native")
	g["compositeImage"] = rasterEffect(func(d Dict) Any {
		compositePixels(d["dst"].(*image.RGBA), d["src"].(*image.RGBA), nil, false, d["invert"].(bool))
		return nil
	})
	g["compositeMaskedImage"] = rasterEffect(func(d Dict) Any {
		compositePixels(d["dst"].(*image.RGBA), d["src"].(*image.RGBA), d["mask"].(*image.RGBA), d["invertMask"].(bool), d["invert"].(bool))
		return nil
	})
	g["maskImage"] = rasterEffect(func(d Dict) Any {
		maskPixels(d["image"].(*image.RGBA), d["mask"].(*image.RGBA))
		return nil
	})
	g["opacityImage"] = rasterEffect(func(d Dict) Any {
		opacityPixels(d["image"].(*image.RGBA), number(d["alpha"]))
		return nil
	})
}

// Traverse flat storage, including row padding, just like the byte-level API.
// Composition and masking leave an incomplete trailing pixel untouched.
func compositePixels(dst, src, mask *image.RGBA, invertMask, invert bool) {
	for i, end := 0, len(dst.Pix)/4*4; i < end; i += 4 {
		coverage := 255
		if mask != nil {
			coverage = int(mask.Pix[i+3])
			if invertMask {
				coverage = 255 - coverage
			}
		}
		alpha := int(src.Pix[i+3]) * coverage / 255
		if alpha == 0 {
			continue
		}
		remaining := 255 - alpha
		for channel := range 3 {
			d := int(dst.Pix[i+channel])
			var value int
			if invert {
				value = (d*remaining + (255-d)*alpha) / 255
			} else {
				value = min(255, int(src.Pix[i+channel])*coverage/255+d*remaining/255)
			}
			dst.Pix[i+channel] = uint8(value)
		}
		dst.Pix[i+3] = uint8(min(255, alpha+int(dst.Pix[i+3])*remaining/255))
	}
}

func maskPixels(img, mask *image.RGBA) {
	if len(img.Pix) != len(mask.Pix) {
		return
	}
	for i, end := 0, len(img.Pix)/4*4; i < end; i += 4 {
		alpha := int(mask.Pix[i+3])
		for channel := range 4 {
			img.Pix[i+channel] = uint8(int(img.Pix[i+channel]) * alpha / 255)
		}
	}
}

func opacityPixels(img *image.RGBA, alpha float64) {
	const maxInt = int(^uint(0) >> 1)
	const minInt = -maxInt - 1
	for i, channel := range img.Pix {
		value := math.Floor(float64(channel) * alpha)
		// Data.Int.floor maps nonfinite results to zero and clamps to native
		// Int bounds, not byte bounds. writeByte then converts Int via Number
		// back to int before narrowing to uint8; retain that boundary behavior.
		if math.IsNaN(value) || math.IsInf(value, 0) {
			value = 0
		} else if value >= float64(maxInt) {
			value = float64(maxInt)
		} else if value <= float64(minInt) {
			value = float64(minInt)
		}
		img.Pix[i] = uint8(int(value))
	}
}
