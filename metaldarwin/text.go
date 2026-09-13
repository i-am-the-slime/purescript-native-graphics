//go:build darwin && !js

package metaldarwin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"unicode/utf8"

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

type msdfMeta struct {
	Atlas struct {
		DistanceRange float32 `json:"distanceRange"`
		Size          float32 `json:"size"`
		Width         int     `json:"width"`
		Height        int     `json:"height"`
		YOrigin       string  `json:"yOrigin"`
	} `json:"atlas"`
	Variants []msdfVariant `json:"variants"`
}

type msdfVariant struct {
	Glyphs []msdfGlyph `json:"glyphs"`
}

type msdfBounds struct{ Left, Bottom, Right, Top float32 }

type msdfGlyph struct {
	Index       uint32      `json:"index"`
	Unicode     uint32      `json:"unicode"`
	PlaneBounds *msdfBounds `json:"planeBounds"`
	AtlasBounds *msdfBounds `json:"atlasBounds"`
}

type msdfEntry struct {
	px0, py0, px1, py1 float32
	u0, v0, u1, v1     float32
	hasGlyph           bool
}

type configuredFont struct {
	source      *text.GoTextFaceSource
	face        *font.Face
	glyphs      map[uint32]msdfEntry
	unicodeKeys bool
}

var configuration Config
var atlasMeta msdfMeta
var atlasPixels *image.RGBA
var configuredFonts []configuredFont

func Configure(config Config) error {
	var meta msdfMeta
	var pixels *image.RGBA
	fonts := make([]configuredFont, len(config.Fonts))
	if len(config.Fonts) > 0 {
		if err := json.Unmarshal(config.AtlasJSON, &meta); err != nil {
			return fmt.Errorf("MSDF metadata: %w", err)
		}
		if meta.Atlas.Width <= 0 || meta.Atlas.Height <= 0 || meta.Atlas.Size <= 0 || meta.Atlas.DistanceRange <= 0 {
			return fmt.Errorf("MSDF metadata requires positive dimensions, size and distance range")
		}
		img, err := png.Decode(bytes.NewReader(config.AtlasPNG))
		if err != nil {
			return fmt.Errorf("MSDF image: %w", err)
		}
		if img.Bounds().Dx() != meta.Atlas.Width || img.Bounds().Dy() != meta.Atlas.Height {
			return fmt.Errorf("MSDF image dimensions do not match metadata")
		}
		pixels = image.NewRGBA(image.Rect(0, 0, meta.Atlas.Width, meta.Atlas.Height))
		draw.Draw(pixels, pixels.Bounds(), img, img.Bounds().Min, draw.Src)
	}
	for i, spec := range config.Fonts {
		if spec.AtlasVariant < 0 || spec.AtlasVariant >= len(meta.Variants) {
			return fmt.Errorf("font %d: atlas variant %d is out of range", i, spec.AtlasVariant)
		}
		source, err := text.NewGoTextFaceSource(bytes.NewReader(spec.Data))
		if err != nil {
			return fmt.Errorf("font %d: %w", i, err)
		}
		face, ok := source.UnsafeInternal().(*font.Face)
		if !ok {
			return fmt.Errorf("font %d: unsupported shaping face", i)
		}
		variant := meta.Variants[spec.AtlasVariant]
		glyphs := make(map[uint32]msdfEntry, len(variant.Glyphs))
		aw, ah := float32(meta.Atlas.Width), float32(meta.Atlas.Height)
		for _, glyph := range variant.Glyphs {
			key := glyph.Index
			if spec.UnicodeKeys {
				key = glyph.Unicode
			}
			entry := msdfEntry{}
			if p, a := glyph.PlaneBounds, glyph.AtlasBounds; p != nil && a != nil {
				entry = msdfEntry{px0: p.Left, py0: -p.Top, px1: p.Right, py1: -p.Bottom,
					u0: a.Left / aw, v0: (ah - a.Top) / ah, u1: a.Right / aw, v1: (ah - a.Bottom) / ah, hasGlyph: true}
				if meta.Atlas.YOrigin == "top" {
					entry.py0, entry.py1 = p.Top, p.Bottom
					entry.v0, entry.v1 = a.Top/ah, a.Bottom/ah
				}
			}
			glyphs[key] = entry
		}
		fonts[i] = configuredFont{source: source, face: face, glyphs: glyphs, unicodeKeys: spec.UnicodeKeys}
	}
	configuration, atlasMeta, atlasPixels, configuredFonts = config, meta, pixels, fonts
	return nil
}

func uploadTextAtlas() {
	if atlasPixels != nil {
		atlasUpload(0, 0, atlasPixels.Bounds().Dx(), atlasPixels.Bounds().Dy(), atlasPixels.Pix)
	}
}

const (
	AlignStart  = 0
	AlignCenter = 1
	AlignEnd    = 2
)

// Size and synthetic emboldening are in view points; positioning is transformed.
// Baseline: 0 alphabetic, 1 cap-middle, 2 top, 3 bottom.
func (r *Renderer) DrawText(x, y float32, content string, size float32, alignTag, baselineTag, fontID int, boldOffset float32, cr, cg, cb, ca float32) {
	if content == "" || size <= 0 || fontID < 0 || fontID >= len(configuredFonts) {
		return
	}
	configured := &configuredFonts[fontID]
	pxSize := float64(size)
	face := &text.GoTextFace{Source: configured.source, Size: pxSize}
	metrics := face.Metrics()
	r.Flush()
	originX, originY := r.top().apply(x, y)
	baseline := float64(originY)
	switch baselineTag {
	case 1:
		baseline += metrics.CapHeight / 2
	case 2:
		baseline += metrics.HAscent
	case 3:
		baseline -= metrics.HDescent
	}
	scale := BackingScale()
	if scale <= 0 {
		scale = 1
	}
	screenPxRange := size * scale * atlasMeta.Atlas.DistanceRange / atlasMeta.Atlas.Size
	width, _ := text.Measure(content, face, 0)
	offset := pxSize * float64(boldOffset)
	width += offset
	startX := float64(originX)
	switch alignTag {
	case AlignCenter:
		startX -= width / 2
	case AlignEnd:
		startX -= width
	}
	verts := make([]float32, 0, utf8.RuneCountInString(content)*48)
	alpha := ca * r.alpha()
	appendRunVerts(&verts, content, configured, startX, baseline, pxSize, cr, cg, cb, alpha)
	if offset != 0 {
		appendRunVerts(&verts, content, configured, startX+offset, baseline, pxSize, cr, cg, cb, alpha)
	}
	drawMSDF(verts, screenPxRange)
}

// Shape directly: text.AppendGlyphs would rasterize into an Ebiten image and
// initialize GLFW even though this host uses only AppKit/Metal.
func appendRunVerts(verts *[]float32, content string, configured *configuredFont, runX, baseline, pxSize float64, cr, cg, cb, alpha float32) {
	if configured.face == nil {
		return
	}
	runes := []rune(content)
	input := shaping.Input{Text: runes, RunStart: 0, RunEnd: len(runes), Direction: di.DirectionLTR,
		Face: configured.face, Size: fixed.Int26_6(math.Round(pxSize * 64))}
	var segmenter shaping.Segmenter
	var shaper shaping.HarfbuzzShaper
	cursor := runX
	for _, segment := range segmenter.Split(input, fixedShapeFont{configured.face}) {
		out := shaper.Shape(segment)
		for _, glyph := range out.Glyphs {
			entry, ok := lookupShapedGlyph(configured, runes, glyph)
			if ok && entry.hasGlyph {
				gx := float32(cursor + fixedToFloat(glyph.XOffset))
				gy := float32(baseline - fixedToFloat(glyph.YOffset))
				x0, y0 := gx+entry.px0*float32(pxSize), gy+entry.py0*float32(pxSize)
				x1, y1 := gx+entry.px1*float32(pxSize), gy+entry.py1*float32(pxSize)
				*verts = append(*verts,
					x0, y0, entry.u0, entry.v0, cr, cg, cb, alpha,
					x1, y0, entry.u1, entry.v0, cr, cg, cb, alpha,
					x1, y1, entry.u1, entry.v1, cr, cg, cb, alpha,
					x0, y0, entry.u0, entry.v0, cr, cg, cb, alpha,
					x1, y1, entry.u1, entry.v1, cr, cg, cb, alpha,
					x0, y1, entry.u0, entry.v1, cr, cg, cb, alpha)
			}
			cursor += fixedToFloat(glyph.Advance)
		}
	}
}

type fixedShapeFont struct{ face *font.Face }

func (f fixedShapeFont) ResolveFace(rune) *font.Face { return f.face }
func fixedToFloat(value fixed.Int26_6) float64       { return float64(value) / 64 }

func lookupShapedGlyph(configured *configuredFont, runes []rune, glyph shaping.Glyph) (msdfEntry, bool) {
	key := uint32(glyph.GlyphID)
	if configured.unicodeKeys {
		index := glyph.TextIndex()
		if index < 0 || index >= len(runes) {
			return msdfEntry{}, false
		}
		key = uint32(runes[index])
	}
	entry, ok := configured.glyphs[key]
	return entry, ok
}
