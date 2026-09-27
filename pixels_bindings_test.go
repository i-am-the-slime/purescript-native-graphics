package graphics

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"math/rand/v2"
	"testing"
)

// These references retain the byte-at-a-time evaluation order, including when
// a source or stencil shares storage with the destination.
func referenceCompositePixels(dst, src, mask []byte, invertMask, invert bool) {
	for pixel := range len(dst) / 4 {
		i := pixel * 4
		coverage := 255
		if mask != nil {
			coverage = int(mask[i+3])
			if invertMask {
				coverage = 255 - coverage
			}
		}
		alpha := int(math.Floor(float64(int(src[i+3])*coverage) / 255))
		if alpha != 0 {
			for channel := range 3 {
				d := float64(dst[i+channel])
				var value float64
				if invert {
					value = math.Floor((d*float64(255-alpha) + (255-d)*float64(alpha)) / 255)
				} else {
					s := float64(src[i+channel])
					value = math.Min(255, math.Floor(s*float64(coverage)/255)+math.Floor(d*float64(255-alpha)/255))
				}
				dst[i+channel] = byte(value)
			}
			dst[i+3] = byte(math.Min(255, float64(alpha)+math.Floor(float64(dst[i+3])*float64(255-alpha)/255)))
		}
	}
}

func referenceMaskPixels(dst, mask []byte) {
	if len(dst) != len(mask) {
		return
	}
	for pixel := range len(dst) / 4 {
		i := pixel * 4
		alpha := float64(mask[i+3])
		for channel := range 4 {
			dst[i+channel] = byte(math.Floor(float64(dst[i+channel]) * alpha / 255))
		}
	}
}

func referenceOpacityPixels(dst []byte, alpha float64) {
	for i, channel := range dst {
		// Data.Int.floor = unsafeClamp <<< Number.floor, followed by the
		// native writeByte's integer conversion and uint8 narrowing.
		x := math.Floor(float64(channel) * alpha)
		var n int
		switch {
		case math.IsNaN(x), math.IsInf(x, 0):
			n = 0
		case x >= float64(math.MaxInt):
			n = math.MaxInt
		case x <= float64(math.MinInt):
			n = math.MinInt
		default:
			n = int(x)
		}
		dst[i] = uint8(int(float64(n)))
	}
}

func assertPixelBytes(t *testing.T, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("byte %d: got %d, want %d", i, got[i], want[i])
			}
		}
		t.Fatalf("byte lengths: got %d, want %d", len(got), len(want))
	}
}

func TestCompositePixelsDifferential(t *testing.T) {
	// Every source-alpha/stencil-alpha pair, with arbitrary non-premultiplied
	// RGB and destination alpha. Zero effective alpha must leave RGB untouched.
	const pixels = 256 * 256
	rng := rand.NewChaCha8([32]byte{42})
	dst, src, mask := make([]byte, pixels*4), make([]byte, pixels*4), make([]byte, pixels*4)
	rng.Read(dst)
	rng.Read(src)
	rng.Read(mask)
	for p := range pixels {
		src[p*4+3] = byte(p / 256)
		mask[p*4+3] = byte(p % 256)
	}
	for _, masked := range []bool{false, true} {
		for _, invertMask := range []bool{false, true} {
			for _, invert := range []bool{false, true} {
				t.Run(fmt.Sprintf("mask=%t/invertMask=%t/invert=%t", masked, invertMask, invert), func(t *testing.T) {
					got, want := bytes.Clone(dst), bytes.Clone(dst)
					var stencil *image.RGBA
					var referenceStencil []byte
					if masked {
						stencil = &image.RGBA{Pix: mask}
						referenceStencil = mask
					}
					referenceCompositePixels(want, src, referenceStencil, invertMask, invert)
					compositePixels(&image.RGBA{Pix: got}, &image.RGBA{Pix: src}, stencil, invertMask, invert)
					assertPixelBytes(t, got, want)
				})
			}
		}
	}
}

func TestBulkPixelsFlatStorage(t *testing.T) {
	rng := rand.NewChaCha8([32]byte{79})
	for _, length := range []int{0, 1, 3, 4, 7, 31, 60} {
		t.Run(fmt.Sprint(length), func(t *testing.T) {
			initial, src, mask := make([]byte, length), make([]byte, length), make([]byte, length)
			rng.Read(initial)
			rng.Read(src)
			rng.Read(mask)
			got, want := bytes.Clone(initial), bytes.Clone(initial)
			// Rect/Stride deliberately describe less than Pix. Native byte APIs
			// process flat storage, not just visible pixels of a subimage.
			img := &image.RGBA{Pix: got, Stride: 20, Rect: image.Rect(0, 0, 1, 1)}
			compositePixels(img, &image.RGBA{Pix: src}, &image.RGBA{Pix: mask}, false, false)
			referenceCompositePixels(want, src, mask, false, false)
			assertPixelBytes(t, got, want)
			maskPixels(img, &image.RGBA{Pix: mask})
			referenceMaskPixels(want, mask)
			assertPixelBytes(t, got, want)
			opacityPixels(img, 0.37)
			referenceOpacityPixels(want, 0.37)
			assertPixelBytes(t, got, want)
		})
	}
}

func TestCompositePixelsAliasedStorage(t *testing.T) {
	initial := []byte{253, 127, 19, 201, 17, 99, 211, 83, 191, 171, 37, 149}
	for _, invertMask := range []bool{false, true} {
		for _, invert := range []bool{false, true} {
			got, want := bytes.Clone(initial), bytes.Clone(initial)
			// Shifted overlapping views also preserve sequential channel reads.
			compositePixels(&image.RGBA{Pix: got[1:9]}, &image.RGBA{Pix: got[:8]}, &image.RGBA{Pix: got[2:10]}, invertMask, invert)
			referenceCompositePixels(want[1:9], want[:8], want[2:10], invertMask, invert)
			assertPixelBytes(t, got, want)
		}
	}
}

func TestMaskPixelsDifferential(t *testing.T) {
	const pixels = 256 * 256
	got, mask := make([]byte, pixels*4), make([]byte, pixels*4)
	for p := range pixels {
		for c := range 4 {
			got[p*4+c] = byte(p / 256)
		}
		mask[p*4+3] = byte(p % 256)
	}
	want := bytes.Clone(got)
	maskPixels(&image.RGBA{Pix: got}, &image.RGBA{Pix: mask})
	referenceMaskPixels(want, mask)
	assertPixelBytes(t, got, want)

	for _, size := range []int{0, 3, 7, 9, 12} {
		got := []byte{255, 127, 99, 201, 33, 66, 88, 149}
		want := bytes.Clone(got)
		maskPixels(&image.RGBA{Pix: got}, &image.RGBA{Pix: make([]byte, size)})
		assertPixelBytes(t, got, want)
	}

	got = []byte{255, 127, 99, 201, 33, 66, 88, 149}
	want = bytes.Clone(got)
	maskPixels(&image.RGBA{Pix: got}, &image.RGBA{Pix: got})
	referenceMaskPixels(want, want)
	assertPixelBytes(t, got, want)
}

func TestOpacityPixelsDifferential(t *testing.T) {
	for _, alpha := range []float64{0, 1, 0.5, 0.37, 1.5, -0.37, 1e30, -1e30, math.NaN(), math.Inf(1), math.Inf(-1)} {
		t.Run(fmt.Sprint(alpha), func(t *testing.T) {
			got := make([]byte, 256)
			for i := range got {
				got[i] = byte(i)
			}
			want := bytes.Clone(got)
			opacityPixels(&image.RGBA{Pix: got}, alpha)
			referenceOpacityPixels(want, alpha)
			assertPixelBytes(t, got, want)
		})
	}
}

func BenchmarkBulkPixels734x414(b *testing.B) {
	for _, operation := range []string{"source-over", "masked-source-over", "inverted-mask-invert", "mask", "opacity"} {
		b.Run(operation, func(b *testing.B) {
			dst := image.NewRGBA(image.Rect(0, 0, 734, 414))
			src := image.NewRGBA(dst.Rect)
			mask := image.NewRGBA(dst.Rect)
			rng := rand.NewChaCha8([32]byte{99})
			rng.Read(dst.Pix)
			rng.Read(src.Pix)
			rng.Read(mask.Pix)
			b.ReportAllocs()
			b.SetBytes(int64(len(dst.Pix)))
			b.ResetTimer()
			for range b.N {
				switch operation {
				case "source-over":
					compositePixels(dst, src, nil, false, false)
				case "masked-source-over":
					compositePixels(dst, src, mask, false, false)
				case "inverted-mask-invert":
					compositePixels(dst, src, mask, true, true)
				case "mask":
					maskPixels(dst, mask)
				case "opacity":
					opacityPixels(dst, 0.73)
				}
			}
		})
	}
}
