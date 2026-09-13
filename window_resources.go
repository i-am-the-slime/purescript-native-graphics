package graphics

import (
	"bytes"
	"fmt"
	. "github.com/purescript-native/go-runtime"
	"image"
	"image/png"
)

var windowIcons []image.Image

func configureResources(d Dict) error {
	data := d["iconPNG"].([]byte)
	if len(data) == 0 {
		windowIcons = nil
		return nil
	}
	icon, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	windowIcons = []image.Image{icon}
	return nil
}
func callbackError(failure any) error {
	if err, ok := failure.(error); ok {
		return err
	}
	return fmt.Errorf("native window callback: %v", failure)
}
