package graphics

import (
	"bytes"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/tdewolff/canvas"
	"sync"
)

type registeredFont struct {
	canvas *canvas.FontFamily
	source *text.GoTextFaceSource
}

var fontRegistry = struct {
	sync.RWMutex
	fonts map[string]*registeredFont
}{fonts: make(map[string]*registeredFont)}

func RegisterFont(name string, data []byte, features string) error {
	family := canvas.NewFontFamily(name)
	if err := family.LoadFont(data, 0, canvas.FontRegular); err != nil {
		return err
	}
	if features != "" {
		family.SetFeatures(features)
	}
	source, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		return err
	}
	fontRegistry.Lock()
	fontRegistry.fonts[name] = &registeredFont{family, source}
	fontRegistry.Unlock()
	return nil
}
func findFont(name string) *registeredFont {
	fontRegistry.RLock()
	defer fontRegistry.RUnlock()
	return fontRegistry.fonts[name]
}
func lookupFont(name string) *registeredFont {
	f := findFont(name)
	if f == nil {
		panic(fmt.Sprintf("native graphics: no registered face named %q", name))
	}
	return f
}
func PickFontSource(family string) *text.GoTextFaceSource { return lookupFont(family).source }

func measureFont(name string, size float64, content string) float64 {
	face := &text.GoTextFace{Source: PickFontSource(name), Size: size}
	width, _ := text.Measure(content, face, 0)
	return width
}
