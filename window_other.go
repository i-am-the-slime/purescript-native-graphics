//go:build !darwin || js

package graphics

import (
	"fmt"
	. "github.com/purescript-native/go-runtime"
)

func deviceScaleFactor() float64 { return 1 }
func configureWindow(Dict) error { return nil }
func runMetal(windowOptions, func(Dict) windowOutput, metalRenderer) error {
	return fmt.Errorf("native graphics: Metal requires Darwin")
}
