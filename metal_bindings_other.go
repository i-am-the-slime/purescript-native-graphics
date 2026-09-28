//go:build !darwin && !js

package graphics

import . "github.com/purescript-native/go-runtime"

func init() {
	f := Foreign("Native.Graphics.Metal.Primitives")
	fail := func() Any { panic("Native.Graphics.Metal requires macOS and a Metal device") }
	for _, name := range []string{"beginFrame", "endFrame", "backingScale", "endPass"} {
		f[name] = fail
	}
	for _, name := range []string{"awaitSubmission", "releaseSubmission", "submissionTiming", "onClose", "newTarget", "releaseTarget", "newBuffer", "releaseBuffer", "beginPass", "compose", "filter", "blit", "stencil", "draw"} {
		f[name] = func(Any) Any { return fail }
	}
}
