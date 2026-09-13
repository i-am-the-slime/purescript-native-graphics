// Package drawing defines backend-independent native drawing primitives.
package drawing

type Command struct {
	Kind       int
	Args       []float64
	Path       []float64
	Text, Font string
	FontID     int
}
type Composite struct {
	Source, Mask int
	InvertMask   bool
	Blend        int
}
type Layer struct{ Global bool }
type Drawing struct {
	Commands  []Command
	Layers    []Layer
	Composite []Composite
	Clear     [4]float32
}
