package graphics

import (
	"image"
	"math"
)

// blurRGBA gaussian-blurs `img` in place with standard deviation `sigma`
// pixels, approximated by three successive box blurs (the classic
// Kovesi/Kuckir construction — within ~3% of a true gaussian). The image is
// alpha-premultiplied (`image.RGBA`), so the four channels blur
// independently without fringing.
func blurRGBA(img *image.RGBA, sigma float64) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w == 0 || h == 0 || sigma <= 0 {
		return
	}
	src := make([]float32, 4*w*h)
	for y := 0; y < h; y++ {
		row := img.Pix[y*img.Stride : y*img.Stride+4*w]
		for k, v := range row {
			src[y*4*w+k] = float32(v)
		}
	}
	tmp := make([]float32, 4*w*h)
	for _, r := range boxRadiiForGauss(sigma) {
		boxBlurH(src, tmp, w, h, r)
		boxBlurV(tmp, src, w, h, r)
	}
	for y := 0; y < h; y++ {
		row := img.Pix[y*img.Stride : y*img.Stride+4*w]
		for k := range row {
			row[k] = uint8(src[y*4*w+k] + 0.5)
		}
	}
}

// boxRadiiForGauss picks three box-blur radii whose composition approximates
// a gaussian of the given sigma.
func boxRadiiForGauss(sigma float64) [3]int {
	n := 3.0
	wIdeal := math.Sqrt(12.0*sigma*sigma/n + 1.0)
	wl := int(math.Floor(wIdeal))
	if wl%2 == 0 {
		wl--
	}
	wu := wl + 2
	mIdeal := (12.0*sigma*sigma - n*float64(wl)*float64(wl) - 4.0*n*float64(wl) - 3.0*n) /
		(-4.0*float64(wl) - 4.0)
	m := int(math.Round(mIdeal))
	var radii [3]int
	for i := range radii {
		if i < m {
			radii[i] = (wl - 1) / 2
		} else {
			radii[i] = (wu - 1) / 2
		}
	}
	return radii
}

// boxBlurH box-blurs each row of `src` (4 interleaved channels) into `dst`
// with a window of `2r+1`, clamping at the edges via a running sum.
func boxBlurH(src, dst []float32, w, h, r int) {
	if r <= 0 {
		copy(dst, src)
		return
	}
	norm := float32(1.0) / float32(2*r+1)
	for y := 0; y < h; y++ {
		row := src[y*4*w : (y+1)*4*w]
		out := dst[y*4*w : (y+1)*4*w]
		var sum [4]float32
		for c := 0; c < 4; c++ {
			sum[c] = row[c] * float32(r+1)
		}
		for x := 1; x <= r; x++ {
			xi := min(x, w-1)
			for c := 0; c < 4; c++ {
				sum[c] += row[4*xi+c]
			}
		}
		for x := 0; x < w; x++ {
			for c := 0; c < 4; c++ {
				out[4*x+c] = sum[c] * norm
			}
			add := min(x+r+1, w-1)
			drop := max(x-r, 0)
			for c := 0; c < 4; c++ {
				sum[c] += row[4*add+c] - row[4*drop+c]
			}
		}
	}
}

// boxBlurV is boxBlurH along columns.
func boxBlurV(src, dst []float32, w, h, r int) {
	if r <= 0 {
		copy(dst, src)
		return
	}
	norm := float32(1.0) / float32(2*r+1)
	stride := 4 * w
	for x := 0; x < w; x++ {
		var sum [4]float32
		for c := 0; c < 4; c++ {
			sum[c] = src[4*x+c] * float32(r+1)
		}
		for y := 1; y <= r; y++ {
			yi := min(y, h-1)
			for c := 0; c < 4; c++ {
				sum[c] += src[yi*stride+4*x+c]
			}
		}
		for y := 0; y < h; y++ {
			for c := 0; c < 4; c++ {
				dst[y*stride+4*x+c] = sum[c] * norm
			}
			add := min(y+r+1, h-1)
			drop := max(y-r, 0)
			for c := 0; c < 4; c++ {
				sum[c] += src[add*stride+4*x+c] - src[drop*stride+4*x+c]
			}
		}
	}
}
