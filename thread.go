//go:build !js

package graphics

import "runtime"

func init() { runtime.LockOSThread() }
