package sfntshape

import "image"
import "image/color"

import "golang.org/x/image/font/sfnt"
import "golang.org/x/image/math/fixed"
import "golang.org/x/image/vector"

// Provided for better compatibility with the etxt/fract.Unit type.
type Fract = fixed.Int26_6

// TODO: add some ArcTo method to draw quarter circles based on
//       cubic bézier curves? so we can (from 0, 0) ArcTo(0, 10, 10, 10)
//       instead of CubeTo(0, 5, 5, 10, 10, 10)
// TODO: add closing func? like rasterizer.ClosePath()?

// A helper type to assist the creation of shapes that can later be
// converted to [sfnt.Segments] and rasterized with [etxt/mask.Rasterize](),
// or directly converted to an [*image.RGBA].
//
// Notice that the rasterization is a CPU process, so working with big shapes
// (based on their bounding rectangle) can be quite expensive.
//
// Despite what the names of the methods might lead you to believe,
// shapes are not created by "drawing lines", but rather by defining
// a set of boundaries that enclose an area. If you get unexpected
// results using shapes, come back to think about this.
//
// Shapes by themselves do not care about the direction you use to define
// the segments (clockwise/counter-clockwise), but rasterizers that use
// the segments most often do. For example, if you define two squares one
// inside the other, both in the same order (e.g: top-left to top-right,
// top-right to bottom right...) the rasterized result will be a single
// square. If you define them following opposite directions, instead,
// the result will be the difference between the two squares.
type Shape struct {
	rasterizer *vector.Rasterizer
	segments []sfnt.Segment
	scale Fract
	invertY bool // but rasterizers already invert coords, so this is negated
}

// Creates a new Shape object.
func New() Shape {
	return Shape {
		rasterizer: vector.NewRasterizer(0, 0),
		segments: make([]sfnt.Segment, 0, 8),
		invertY: false,
		scale: 64,
	}
}

// Returns the current scaling factor.
func (self *Shape) GetScale() Fract {
	return self.scale
}

// Sets a scaling factor to be applied to the coordinates of
// subsequent [Shape.MoveTo](), [Shape.LineTo]() and similar
// commands.
func (self *Shape) SetScale(scale float64) {
	self.SetScaleFract(fixedFromFloat64(scale))
}

// Like [Shape.SetScale](), but expecting a Fract value
// instead of a float64.
func (self *Shape) SetScaleFract(scale Fract) {
	self.scale = scale
}

// Returns whether [Shape.InvertY] is active or inactive.
func (self *Shape) HasInvertY() bool { return self.invertY }

// Let's say you want to draw a triangle pointing up, similar to an
// "A". By default, you would move to (0, 0) and then draw lines to
// (k, 2*k), (2*k, 0) and back to (0, 0).
//
// If you set InvertY to true, the previous shape will draw a triangle
// pointing down instead, similar to a "V". This is a convenient flag
// that makes it easier to work on different contexts (e.g., font glyphs
// are defined with the ascenders going into the negative y plane).
//
// InvertY can also be used creatively or to switch between clockwise and
// counter-clockwise directions when drawing symmetrical shapes that have
// their center at (0, 0).
func (self *Shape) InvertY(active bool) { self.invertY = active }

// Gets the shape information as [sfnt.Segments]. The underlying data
// is referenced both by the Shape and the sfnt.Segments, so be
// careful what you do with it.
func (self *Shape) Segments() sfnt.Segments {
	return sfnt.Segments(self.segments)
}

// Moves the current position to (x, y).
// See [vector.Rasterizer] operations and [sfnt.Segment].
func (self *Shape) MoveTo(x, y int) {
	self.MoveToFract(Fract(x << 6), Fract(y << 6))
}

// Like [Shape.MoveTo], but with fractional coordinates.
func (self *Shape) MoveToFract(x, y Fract) {
	if !self.invertY { y = -y }
	if self.scale != 64 {
		x = x.Mul(self.scale)
		y = y.Mul(self.scale)
	}
	self.segments = append(self.segments,
		sfnt.Segment {
			Op: sfnt.SegmentOpMoveTo,
			Args: [3]fixed.Point26_6 {
				fixed.Point26_6{x, y},
				fixed.Point26_6{},
				fixed.Point26_6{},
			},
		})
}

// Creates a straight boundary from the current position to (x, y).
// See [vector.Rasterizer] operations and [sfnt.Segment].
func (self *Shape) LineTo(x, y int) {
	self.LineToFract(Fract(x << 6), Fract(y << 6))
}

// Like [Shape.LineTo], but with fractional coordinates.
func (self *Shape) LineToFract(x, y Fract) {
	if !self.invertY { y = -y }
	if self.scale != 64 {
		x = x.Mul(self.scale)
		y = y.Mul(self.scale)
	}
	self.segments = append(self.segments,
		sfnt.Segment {
			Op: sfnt.SegmentOpLineTo,
			Args: [3]fixed.Point26_6 {
				fixed.Point26_6{x, y},
				fixed.Point26_6{},
				fixed.Point26_6{},
			},
		})
}

// Creates a quadratic Bézier curve (also known as a conic Bézier curve)
// to (x, y) with (ctrlX, ctrlY) as the control point.
// See [vector.Rasterizer] operations and [sfnt.Segment].
func (self *Shape) QuadTo(ctrlX, ctrlY, x, y int) {
	self.QuadToFract(
		Fract(ctrlX << 6), Fract(ctrlY << 6),
		Fract(x     << 6), Fract(y     << 6))
}

// Like [Shape.QuadTo], but with fractional coordinates.
func (self *Shape) QuadToFract(ctrlX, ctrlY, x, y Fract) {
	if !self.invertY { ctrlY, y = -ctrlY, -y }
	if self.scale != 64 {
		ctrlX = ctrlX.Mul(self.scale)
		ctrlY = ctrlY.Mul(self.scale)
		x = x.Mul(self.scale)
		y = y.Mul(self.scale)
	}
	self.segments = append(self.segments,
		sfnt.Segment {
			Op: sfnt.SegmentOpQuadTo,
			Args: [3]fixed.Point26_6 {
				fixed.Point26_6{ctrlX, ctrlY},
				fixed.Point26_6{    x,     y},
				fixed.Point26_6{},
			},
		})
}

// Creates a cubic Bézier curve to (x, y) with (cx1, cy1) and (cx2, cy2)
// as the control points.
// See [golang.org/x/image/vector.Rasterizer] operations and
// [golang.org/x/image/font/sfnt.Segment].
func (self *Shape) CubeTo(cx1, cy1, cx2, cy2, x, y int) {
	self.CubeToFract(
		Fract(cx1 << 6), Fract(cy1 << 6),
		Fract(cx2 << 6), Fract(cy2 << 6),
		Fract(x   << 6), Fract(y   << 6))
}

// Like [Shape.CubeTo], but with fractional coordinates.
func (self *Shape) CubeToFract(cx1, cy1, cx2, cy2, x, y Fract) {
	if !self.invertY { cy1, cy2, y = -cy1, -cy2, -y }
	if self.scale != 64 {
		cx1 = cx1.Mul(self.scale)
		cx2 = cx2.Mul(self.scale)
		cy1 = cy1.Mul(self.scale)
		cy2 = cy2.Mul(self.scale)
		x = x.Mul(self.scale)
		y = y.Mul(self.scale)
	}
	self.segments = append(self.segments,
		sfnt.Segment {
			Op: sfnt.SegmentOpCubeTo,
			Args: [3]fixed.Point26_6 {
				fixed.Point26_6{cx1, cy1},
				fixed.Point26_6{cx2, cy2},
				fixed.Point26_6{  x,   y},
			},
		})
}

// Resets the shape segments. Be careful to not be holding the segments
// from [Shape.Segments]() when calling this (they may be overriden soon).
func (self *Shape) Reset() { self.segments = self.segments[0 : 0] }

// A helper method to rasterize the current shape into an [*image.Alpha].
func (self *Shape) Rasterize() (*image.Alpha, error) {
	return self.RasterizeFract(0, 0)
}

// A helper method to rasterize the current shape displaced by the given
// fractional offset into an [*image.Alpha].
func (self *Shape) RasterizeFract(offsetX, offsetY Fract) (*image.Alpha, error) {
	segments := self.Segments()
	if len(segments) == 0 { return nil, nil }
	return Rasterize(segments, self.rasterizer, offsetX, offsetY)
}

// A helper method to rasterize the current shape with the given
// colors. You could then export the result to a png file, e.g.:
//   file, _ := os.Create("my_ugly_shape.png")
//   _ = png.Encode(file, shape.Paint(color.White, color.Black))
//   // ...maybe even checking errors and closing the file ;)
func (self *Shape) Paint(drawColor, backColor color.Color) *image.RGBA {
	segments := self.Segments()
	if len(segments) == 0 { return nil }
	mask, err := Rasterize(segments, self.rasterizer, 0, 0)
	if err != nil { panic(err) } // default rasterizer doesn't return errors
	rgba := image.NewRGBA(mask.Rect)

	r, g, b, a := drawColor.RGBA()
	nrgba := color.NRGBA64 { R: uint16(r), G: uint16(g), B: uint16(b), A: 0 }
	for y := mask.Rect.Min.Y; y < mask.Rect.Max.Y; y++ {
		for x := mask.Rect.Min.X; x < mask.Rect.Max.X; x++ {
			nrgba.A = uint16((a*uint32(mask.AlphaAt(x, y).A))/255)
			rgba.Set(x, y, mixColors(nrgba, backColor))
		}
	}
	return rgba
}

// Helper method for [Shape.Paint](). The same as mixOverFunc
// on the generic version of etxt (see etxt/ebiten_no.go).
func mixColors(draw color.Color, back color.Color) color.Color {
	dr, dg, db, da := draw.RGBA()
	if da == 0xFFFF { return draw }
	if da == 0      { return back }
	br, bg, bb, ba := back.RGBA()
	if ba == 0      { return draw }
	return color.RGBA64 {
		R: uint16N((dr*0xFFFF + br*(0xFFFF - da))/0xFFFF),
		G: uint16N((dg*0xFFFF + bg*(0xFFFF - da))/0xFFFF),
		B: uint16N((db*0xFFFF + bb*(0xFFFF - da))/0xFFFF),
		A: uint16N((da*0xFFFF + ba*(0xFFFF - da))/0xFFFF),
	}
}

// clamping from uint32 to uint16 values
func uint16N(value uint32) uint16 {
	if value > 65535 { return 65535 }
	return uint16(value)
}
