//go:build darwin && !js

package graphics

import (
	"github.com/i-am-the-slime/purescript-native-graphics/metaldarwin"
	. "github.com/purescript-native/go-runtime"
)

func init() {
	text := Foreign("Native.Graphics.Metal.Text")
	text["resourceVersion"] = func() Any { return metaldarwin.TextResourceVersion() }
	text["readResources"] = func() Any {
		resources := metaldarwin.ReadTextResources()
		fonts := make([]Any, len(resources.Fonts))
		for i, font := range resources.Fonts {
			fonts[i] = Dict{"atlasVariant": font.AtlasVariant, "unicodeKeys": font.UnicodeKeys}
		}
		return Dict{"atlasJSON": resources.AtlasJSON, "width": resources.Width, "height": resources.Height, "fonts": fonts}
	}
	text["shapeText"] = curry3(func(fontID, content, size Any) Any {
		return func() Any {
			shaped := metaldarwin.ShapeText(integer(fontID), content.(string), number(size))
			glyphs := make([]Any, len(shaped.Glyphs))
			for i, glyph := range shaped.Glyphs {
				glyphs[i] = Dict{"glyphID": glyph.GlyphID, "sourceIndex": glyph.SourceIndex,
					"xOffset": glyph.XOffset, "yOffset": glyph.YOffset, "advance": glyph.Advance}
			}
			return Dict{"glyphs": glyphs, "width": shaped.Width, "capHeight": shaped.CapHeight,
				"ascent": shaped.Ascent, "descent": shaped.Descent, "backingScale": shaped.BackingScale}
		}
	})
}
