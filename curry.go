package graphics

import . "github.com/purescript-native/go-runtime"

func curry2(f func(Any, Any) Any) Any {
	return func(a Any) Any { return func(b Any) Any { return f(a, b) } }
}
func curry3(f func(Any, Any, Any) Any) Any {
	return func(a Any) Any { return func(b Any) Any { return func(c Any) Any { return f(a, b, c) } } }
}
