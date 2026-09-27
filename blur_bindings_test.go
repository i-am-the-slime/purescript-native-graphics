package graphics

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"math/rand/v2"
	"testing"

	. "github.com/purescript-native/go-runtime"
)

// This reference keeps PureScript Numbers in float64 and explicitly rounds
// only where the former Pixels.boxPass used float32 or wrote a FloatBuffer.
func oldBlurReference(pixels []byte, width, height int, radii []int) []byte {
	result := bytes.Clone(pixels)
	if width <= 0 || height <= 0 {
		return result
	}
	f32 := func(value float64) float64 { return float64(float32(value)) }
	length := width * height * 4
	src, tmp := make([]float64, length), make([]float64, length)
	for i := range length {
		src[i] = float64(pixels[i])
	}
	pass := func(input, output []float64, lines, length, lineStride, step, radius int) {
		if radius <= 0 {
			copy(output, input)
			return
		}
		norm := f32(1.0 / f32(float64(2*radius+1)))
		for line := range lines {
			for channel := range 4 {
				base := line*lineStride + channel
				total := f32(input[base] * f32(float64(radius+1)))
				for x := 1; x < radius+1; x++ {
					index := x
					if index >= length {
						index = length - 1
					}
					total = f32(total + input[base+index*step])
				}
				for x := range length {
					output[base+x*step] = f32(total * norm)
					addIndex, dropIndex := x+radius+1, x-radius
					if addIndex >= length {
						addIndex = length - 1
					}
					if dropIndex < 0 {
						dropIndex = 0
					}
					delta := f32(input[base+addIndex*step] - input[base+dropIndex*step])
					total = f32(total + delta)
				}
			}
		}
	}
	for _, radius := range radii {
		pass(src, tmp, height, width, width*4, 4, radius)
		pass(tmp, src, width, height, 4, width*4, radius)
	}
	for i, value := range src {
		result[i] = uint8(math.Floor(value + 0.5))
	}
	return result
}

func TestBlurRGBAMatchesOldArithmetic(t *testing.T) {
	sizes := [][2]int{{1, 1}, {1, 7}, {8, 1}, {2, 3}, {3, 2}, {7, 5}, {19, 11}}
	radiusSets := [][]int{nil, {0}, {-2}, {1, 1, 1}, {0, 1, 2}, {4, 5, 5}, {31, 33, 32}, {0, -1, 3, 1}}
	for _, size := range sizes {
		for _, radii := range radiusSets {
			for _, premultiplied := range []bool{false, true} {
				t.Run(fmt.Sprintf("%dx%d/radii=%v/premultiplied=%t", size[0], size[1], radii, premultiplied), func(t *testing.T) {
					rng := rand.New(rand.NewPCG(217, 91))
					for sample := range 16 {
						img := image.NewRGBA(image.Rect(0, 0, size[0], size[1]))
						for pixel := range len(img.Pix) / 4 {
							alpha := rng.IntN(256)
							if sample == 0 {
								alpha = 0
							} else if sample == 1 {
								alpha = 255
							}
							img.Pix[pixel*4+3] = byte(alpha)
							for channel := range 3 {
								limit := 256
								if premultiplied {
									limit = alpha + 1
								}
								img.Pix[pixel*4+channel] = byte(rng.IntN(limit))
							}
						}
						want := oldBlurReference(img.Pix, size[0], size[1], radii)
						blurRGBA(img, size[0], size[1], radii)
						for i, got := range img.Pix {
							if got != want[i] {
								t.Fatalf("sample %d pixel (%d,%d) channel %d: got %d, want %d", sample, (i/4)%size[0], (i/4)/size[0], i%4, got, want[i])
							}
						}
					}
				})
			}
		}
	}
}

func TestBlurRGBANonpositiveDimensionsLeaveImageUnchanged(t *testing.T) {
	for _, size := range [][2]int{{0, 2}, {2, 0}, {-1, 2}, {2, -1}, {-1, -1}} {
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		copy(img.Pix, []byte{8, 16, 32, 64, 20, 30, 40, 80, 90, 80, 70, 128, 1, 2, 3, 255})
		want := bytes.Clone(img.Pix)
		blurRGBA(img, size[0], size[1], []int{1, 2, 2})
		if !bytes.Equal(img.Pix, want) {
			t.Fatalf("dimensions %v changed pixels: got %v, want %v", size, img.Pix, want)
		}
	}
}

func TestBlurImageEffect(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	copy(img.Pix, []byte{255, 128, 32, 255, 0, 0, 0, 0, 20, 30, 40, 100, 3, 10, 60, 80, 128, 0, 0, 128, 10, 20, 30, 200})
	before := bytes.Clone(img.Pix)
	want := oldBlurReference(before, 3, 2, []int{1, 2, 2})
	effect := Foreign("Native.Graphics.Raster.Native")["blurImage"].(func(Any) Any)(Dict{
		"width": 3, "height": 2, "image": img, "radii": []Any{1, 2, 2},
	})
	if !bytes.Equal(img.Pix, before) {
		t.Fatal("constructing the Effect changed image pixels")
	}
	Run(effect)
	if !bytes.Equal(img.Pix, want) {
		t.Fatalf("running the blur Effect: got %v, want %v", img.Pix, want)
	}
}

func BenchmarkBlurRGBA(b *testing.B) {
	const width, height = 840, 640
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	seed := make([]byte, len(img.Pix))
	rng := rand.New(rand.NewPCG(217, 91))
	for pixel := range width * height {
		alpha := rng.IntN(256)
		seed[pixel*4+3] = byte(alpha)
		for channel := range 3 {
			seed[pixel*4+channel] = byte(rng.IntN(alpha + 1))
		}
	}
	radii := []int{5, 5, 6}
	b.ReportAllocs()
	b.SetBytes(int64(len(seed)))
	for b.Loop() {
		copy(img.Pix, seed)
		blurRGBA(img, width, height, radii)
	}
}
