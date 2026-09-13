//go:build darwin && !js

package graphics

import (
	metal "github.com/i-am-the-slime/purescript-native-graphics/metaldarwin"
	. "github.com/purescript-native/go-runtime"
)

func metalFloats(value Any) []float32 {
	values := value.([]Any)
	out := make([]float32, len(values))
	for i, v := range values {
		out[i] = float32(number(v))
	}
	return out
}
func init() {
	f := Foreign("Native.Graphics.Metal.Primitives")
	f["beginFrame"] = func() Any { return metal.FrameBegin() }
	f["endFrame"] = func() Any { metal.FrameEnd(); return nil }
	f["backingScale"] = func() Any { return float64(metal.BackingScale()) }
	f["onClose"] = func(v Any) Any { return func() Any { metal.SetCleanupFunc(func() { Run(v) }); return nil } }
	f["newTarget"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			return metal.NewTarget(integer(d["width"]), integer(d["height"]), d["multisample"].(bool))
		}
	}
	f["releaseTarget"] = func(v Any) Any { return func() Any { metal.ReleaseTarget(v.(*metal.Target)); return nil } }
	f["newBuffer"] = func(v Any) Any { return func() Any { return metal.NewBuffer(integer(v)) } }
	f["releaseBuffer"] = func(v Any) Any { return func() Any { metal.ReleaseBuffer(v.(*metal.Buffer)); return nil } }
	f["beginPass"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			metal.BeginPass(d["target"].(*metal.Target), d["clear"].(bool), metalFloats(d["color"]))
			return nil
		}
	}
	f["endPass"] = func() Any { metal.EndPass(); return nil }
	f["compose"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			raw := d["sources"].([]Any)
			targets := make([]*metal.Target, len(raw))
			for i, value := range raw {
				targets[i] = value.(*metal.Target)
			}
			values := d["ops"].([]Any)
			ops := make([]int32, len(values))
			for i, value := range values {
				ops[i] = int32(integer(value))
			}
			metal.Compose(targets, ops, d["destination"].(*metal.Target), d["drawable"].(bool))
			return nil
		}
	}
	f["filter"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			metal.Filter(d["source"].(*metal.Target), d["destination"].(*metal.Target), metalFloats(d["uniform"]))
			return nil
		}
	}
	f["blit"] = func(v Any) Any { return func() Any { metal.Blit(v.(*metal.Target)); return nil } }
	f["stencil"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			metal.Stencil(integer(d["state"]), integer(d["depth"]), integer(d["reference"]))
			return nil
		}
	}
	f["draw"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			metal.Draw(d["buffer"].(*metal.Buffer), integer(d["offset"]), metalFloats(d["vertices"]), integer(d["pipeline"]), integer(d["stride"]), metalFloats(d["uniform"]))
			return nil
		}
	}
}
