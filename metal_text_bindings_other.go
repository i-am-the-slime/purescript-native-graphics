//go:build !darwin || js

package graphics

import . "github.com/purescript-native/go-runtime"

func init() {
	text := Foreign("Native.Graphics.Metal.Text")
	unsupported := func() Any { panic("Native.Graphics.Metal.Text requires the native Darwin Metal backend") }
	text["resourceVersion"] = unsupported
	text["readResources"] = unsupported
	text["shapeText"] = curry3(func(_, _, _ Any) Any { return unsupported })
}
