//go:build darwin && !js

package metaldarwin

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/shaping"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/math/fixed"
)

// FontConfig binds a caller-owned shaping font to a variant of the packed atlas.
// UnicodeKeys selects Unicode rather than font glyph IDs as atlas lookup keys.
type FontConfig struct {
	Data         []byte
	AtlasVariant int
	UnicodeKeys  bool
}

// Config contains resources and native preferences, never application identities.
// Configure must run before Setup, on the same thread as rendering.
type Config struct {
	Fonts                            []FontConfig
	AtlasPNG, AtlasJSON, IconPNG     []byte
	QuitMenuTitle, QuitKeyEquivalent string
	SampleCount                      int
	Appearance                       Appearance
}

type Color struct{ Red, Green, Blue, Alpha float64 }
type Appearance struct {
	Name                                                                               string
	Background, Tint                                                                   Color
	Opaque, TitlebarTransparent, TitleHidden, FullSizeContentView, MovableByBackground bool
	Backdrop, Glass                                                                    bool
	CornerRadius                                                                       float64
	GlassStyle, Material, BlendingMode, State                                          int
}

type configuredFont struct {
	source *text.GoTextFaceSource
	face   *font.Face
}

type TextFontResource struct {
	AtlasVariant int
	UnicodeKeys  bool
}

type TextResourceData struct {
	AtlasJSON     string
	Width, Height int
	Fonts         []TextFontResource
}

var configuration Config
var atlasPixels *image.RGBA
var configuredFonts []configuredFont
var textResources TextResourceData
var textResourceVersion int

// Configure loads native font and image resources. Atlas interpretation belongs
// to Native.Graphics.Metal.Text and is deferred until its first text draw.
func Configure(config Config) error {
	var pixels *image.RGBA
	fonts := make([]configuredFont, len(config.Fonts))
	resources := TextResourceData{AtlasJSON: string(config.AtlasJSON), Fonts: make([]TextFontResource, len(config.Fonts))}
	if len(config.Fonts) > 0 {
		img, err := png.Decode(bytes.NewReader(config.AtlasPNG))
		if err != nil {
			return fmt.Errorf("MSDF image: %w", err)
		}
		resources.Width, resources.Height = img.Bounds().Dx(), img.Bounds().Dy()
		pixels = image.NewRGBA(image.Rect(0, 0, resources.Width, resources.Height))
		draw.Draw(pixels, pixels.Bounds(), img, img.Bounds().Min, draw.Src)
	}
	for i, spec := range config.Fonts {
		source, err := text.NewGoTextFaceSource(bytes.NewReader(spec.Data))
		if err != nil {
			return fmt.Errorf("font %d: %w", i, err)
		}
		face, ok := source.UnsafeInternal().(*font.Face)
		if !ok {
			return fmt.Errorf("font %d: unsupported shaping face", i)
		}
		fonts[i] = configuredFont{source: source, face: face}
		resources.Fonts[i] = TextFontResource{AtlasVariant: spec.AtlasVariant, UnicodeKeys: spec.UnicodeKeys}
	}
	configuration, atlasPixels, configuredFonts, textResources = config, pixels, fonts, resources
	textResourceVersion++
	return nil
}

func uploadTextAtlas() {
	if atlasPixels != nil {
		atlasUpload(0, 0, atlasPixels.Bounds().Dx(), atlasPixels.Bounds().Dy(), atlasPixels.Pix)
	}
}

func TextResourceVersion() int            { return textResourceVersion }
func ReadTextResources() TextResourceData { return textResources }

type TextGlyph struct {
	GlyphID, SourceIndex      int
	XOffset, YOffset, Advance float64
}

type ShapedText struct {
	Glyphs                        []TextGlyph
	Width, CapHeight              float64
	Ascent, Descent, BackingScale float64
}

// ShapeText exposes shaping and font metrics only: no atlas lookup, cursor
// accumulation, baseline placement, alignment, emboldening, or glyph geometry.
// Shaping directly avoids text.AppendGlyphs initializing an Ebiten/GLFW window.
func ShapeText(fontID int, content string, size float64) ShapedText {
	if fontID < 0 || fontID >= len(configuredFonts) || size <= 0 {
		return ShapedText{}
	}
	configured := &configuredFonts[fontID]
	face := &text.GoTextFace{Source: configured.source, Size: size}
	metrics := face.Metrics()
	width, _ := text.Measure(content, face, 0)
	result := ShapedText{Width: width, CapHeight: metrics.CapHeight,
		Ascent: metrics.HAscent, Descent: metrics.HDescent, BackingScale: float64(BackingScale())}
	runes := []rune(content)
	input := shaping.Input{Text: runes, RunStart: 0, RunEnd: len(runes), Direction: di.DirectionLTR,
		Face: configured.face, Size: fixed.Int26_6(math.Round(size * 64))}
	var segmenter shaping.Segmenter
	var shaper shaping.HarfbuzzShaper
	for _, segment := range segmenter.Split(input, fixedShapeFont{configured.face}) {
		out := shaper.Shape(segment)
		for _, glyph := range out.Glyphs {
			result.Glyphs = append(result.Glyphs, TextGlyph{GlyphID: int(glyph.GlyphID), SourceIndex: glyph.TextIndex(),
				XOffset: fixedToFloat(glyph.XOffset), YOffset: fixedToFloat(glyph.YOffset), Advance: fixedToFloat(glyph.Advance)})
		}
	}
	return result
}

type fixedShapeFont struct{ face *font.Face }

func (f fixedShapeFont) ResolveFace(rune) *font.Face { return f.face }
func fixedToFloat(value fixed.Int26_6) float64       { return float64(value) / 64 }
