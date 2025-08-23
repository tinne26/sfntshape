package sfntshape

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// Provided for better compatibility with the etxt/fract.Unit type.
type Fract = fixed.Int26_6

// TODO: add some ArcTo method to draw quarter circles based on
//       cubic bézier curves? so we can (from 0, 0) ArcTo(0, 10, 10, 10)
//       instead of CubeTo(0, 5, 5, 10, 10, 10)
// TODO: add closing func? like rasterizer.ClosePath()?
// TODO: relative Quad and Cube?

// A helper type to assist the creation of shapes that can later be
// converted to [sfnt.Segments] and rasterized with [Rasterize](),
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
	segments   []sfnt.Segment
	scale      Fract
	invertY    bool // but rasterizers already invert coords, so this is negated
	hasLastDir bool
	lastDir    float64
}

// Creates a new Shape object.
func New() Shape {
	return Shape{
		rasterizer: vector.NewRasterizer(0, 0),
		segments:   make([]sfnt.Segment, 0, 8),
		invertY:    false,
		scale:      64,
	}
}

// Returns the current scaling factor.
func (shape *Shape) GetScale() Fract {
	return shape.scale
}

// Sets a scaling factor to be applied to the coordinates of
// subsequent [Shape.MoveTo](), [Shape.LineTo]() and similar
// commands.
func (shape *Shape) SetScale(scale float64) {
	shape.SetScaleFract(fixedFromFloat64(scale))
}

// Like [Shape.SetScale](), but expecting a Fract value
// instead of a float64.
func (shape *Shape) SetScaleFract(scale Fract) {
	shape.scale = scale
}

// FindFurthest finds the furthest point in the shape from the
// given position.
func (shape *Shape) FindFurthest(ox, oy Fract) (x, y Fract) {
	furthestDistSq := Fract(0)
	furthest := fixed.Point26_6{X: ox, Y: oy}
	for _, segment := range shape.segments {
		o := segment.Args[0]
		dx, dy := ox-o.X, oy-o.Y
		dist := dx.Mul(dx) + dy.Mul(dy)
		if dist > furthestDistSq {
			furthest = o
			furthestDistSq = dist
		}
	}
	return furthest.X, furthest.Y
}

// Returns whether [Shape.InvertY] is active or inactive.
func (shape *Shape) HasInvertY() bool { return shape.invertY }

// InvertY changes the positive Y axis from going up to going
// down when enabled.
//
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
func (shape *Shape) InvertY(active bool) { shape.invertY = active }

// Position returns the current position.
func (shape *Shape) Position() (x, y Fract) {
	if len(shape.segments) == 0 {
		return 0, 0
	}
	pt := getSegmentTarget(shape.segments[len(shape.segments)-1])
	if shape.invertY {
		pt.Y = -pt.Y
	}
	return pt.X, pt.Y
}

func getSegmentTarget(segment sfnt.Segment) fixed.Point26_6 {
	switch segment.Op {
	case sfnt.SegmentOpMoveTo, sfnt.SegmentOpLineTo:
		return segment.Args[0]
	case sfnt.SegmentOpQuadTo:
		return segment.Args[1]
	case sfnt.SegmentOpCubeTo:
		return segment.Args[2]
	default:
		panic(segment.Op)
	}
}

// Direction returns the direction at the end of the last segment,
// in radians. The directions match Golang's conventions:
//   - 0: x axis positive (right)
//   - math.Pi/2: y axis positive (up by default)
//   - math.Pi: x axis negative (left)
//   - 3*math.Pi/2: y axis negative (down by default)
func (shape *Shape) Direction() float64 {
	shape.ensureHasDir()
	return shape.lastDir
}

func (shape *Shape) ensureHasDir() {
	if !shape.hasLastDir {
		shape.lastDir = shape.computeLastDir()
	}
}

func (shape *Shape) computeLastDir() float64 {
	if len(shape.segments) == 0 {
		return 0.0
	}
	last := shape.segments[len(shape.segments)-1]
	to := getSegmentTarget(last)
	from := fixed.Point26_6{}
	switch last.Op {
	case sfnt.SegmentOpMoveTo, sfnt.SegmentOpLineTo:
		if len(shape.segments) > 1 {
			from = shape.segments[len(shape.segments)-2].Args[0]
		}
	case sfnt.SegmentOpQuadTo:
		from = last.Args[0]
	case sfnt.SegmentOpCubeTo:
		from = last.Args[1]
	default:
		panic(last.Op)
	}
	dx, dy := to.X-from.X, to.Y-from.Y
	if shape.invertY {
		dy = -dy
	}
	return math.Atan2(float64(dy)/64.0, float64(dx)/64.0)
}

// Gets the shape information as [sfnt.Segments]. The underlying data
// is referenced both by the Shape and the sfnt.Segments, so be
// careful what you do with it.
func (shape *Shape) Segments() sfnt.Segments {
	return sfnt.Segments(shape.segments)
}

// Turn changes the current direction by the given degrees.
// Positive values turn right, negative values turn left.
func (shape *Shape) Turn(degrees float64) {
	shape.ensureHasDir()
	if !shape.invertY {
		degrees = -degrees
	}
	shape.lastDir += degrees * (math.Pi / 180.0)
}

func (shape *Shape) TurnTo(degrees float64) {
	if !shape.invertY {
		degrees = -degrees
	}
	shape.lastDir = degrees * (math.Pi / 180.0)
	shape.hasLastDir = true
}

// Move moves d units in the current direction.
func (shape *Shape) Move(d int) {
	shape.MoveFract(Fract(d << 6))
}

// Move moves d units in the current direction.
func (shape *Shape) MoveFract(d Fract) {
	shape.ensureHasDir()
	x, y := shape.Position()
	x, y = advance(x, y, shape.lastDir, d)
	shape.MoveToFract(x, y)
}

func advance(x, y Fract, rads float64, d Fract) (Fract, Fract) {
	s, c := math.Sincos(rads)
	dF64 := (float64(d) / 64.0)
	return x + fixedFromFloat64(dF64*c), y + fixedFromFloat64(dF64*s)
}

// Moves the current position to (x, y).
// See [vector.Rasterizer] operations and [sfnt.Segment].
func (shape *Shape) MoveTo(x, y int) {
	shape.MoveToFract(Fract(x<<6), Fract(y<<6))
}

// Like [Shape.MoveTo], but with fractional coordinates.
func (shape *Shape) MoveToFract(x, y Fract) {
	if !shape.invertY {
		y = -y
	}
	if shape.scale != 64 {
		x = x.Mul(shape.scale)
		y = y.Mul(shape.scale)
	}
	shape.segments = append(shape.segments,
		sfnt.Segment{
			Op:   sfnt.SegmentOpMoveTo,
			Args: [3]fixed.Point26_6{{X: x, Y: y}, {}, {}},
		})
}

// Line creates a straight boundary from the current position
// to d units in the current direction.
func (shape *Shape) Line(d int) {
	shape.LineFract(Fract(d << 6))
}

// LineFract creates a straight boundary from the current position
// to d units in the current direction.
func (shape *Shape) LineFract(d Fract) {
	shape.ensureHasDir()
	x, y := shape.Position()
	x, y = advance(x, y, shape.lastDir, d)
	shape.LineToFract(x, y)
	shape.hasLastDir = true
}

// Creates a straight boundary from the current position to (x, y).
// See [vector.Rasterizer] operations and [sfnt.Segment].
func (shape *Shape) LineTo(x, y int) {
	shape.LineToFract(Fract(x<<6), Fract(y<<6))
}

// Like [Shape.LineTo], but with fractional coordinates.
func (shape *Shape) LineToFract(x, y Fract) {
	shape.hasLastDir = false
	if !shape.invertY {
		y = -y
	}
	if shape.scale != 64 {
		x = x.Mul(shape.scale)
		y = y.Mul(shape.scale)
	}
	shape.segments = append(shape.segments,
		sfnt.Segment{
			Op:   sfnt.SegmentOpLineTo,
			Args: [3]fixed.Point26_6{{X: x, Y: y}, {}, {}},
		})
}

// Creates a quadratic Bézier curve (also known as a conic Bézier curve)
// to (x, y) with (ctrlX, ctrlY) as the control point.
// See [vector.Rasterizer] operations and [sfnt.Segment].
func (shape *Shape) QuadTo(ctrlX, ctrlY, x, y int) {
	shape.QuadToFract(
		Fract(ctrlX<<6), Fract(ctrlY<<6),
		Fract(x<<6), Fract(y<<6))
}

// Like [Shape.QuadTo], but with fractional coordinates.
func (shape *Shape) QuadToFract(ctrlX, ctrlY, x, y Fract) {
	shape.hasLastDir = false
	if !shape.invertY {
		ctrlY, y = -ctrlY, -y
	}
	if shape.scale != 64 {
		ctrlX = ctrlX.Mul(shape.scale)
		ctrlY = ctrlY.Mul(shape.scale)
		x = x.Mul(shape.scale)
		y = y.Mul(shape.scale)
	}
	shape.segments = append(shape.segments,
		sfnt.Segment{
			Op: sfnt.SegmentOpQuadTo,
			Args: [3]fixed.Point26_6{
				{X: ctrlX, Y: ctrlY},
				{X: x, Y: y},
				{},
			},
		})
}

// Creates a cubic Bézier curve to (x, y) with (cx1, cy1) and (cx2, cy2)
// as the control points.
// See [golang.org/x/image/vector.Rasterizer] operations and
// [golang.org/x/image/font/sfnt.Segment].
func (shape *Shape) CubeTo(cx1, cy1, cx2, cy2, x, y int) {
	shape.CubeToFract(
		Fract(cx1<<6), Fract(cy1<<6),
		Fract(cx2<<6), Fract(cy2<<6),
		Fract(x<<6), Fract(y<<6))
}

// Like [Shape.CubeTo], but with fractional coordinates.
func (shape *Shape) CubeToFract(cx1, cy1, cx2, cy2, x, y Fract) {
	shape.hasLastDir = false
	if !shape.invertY {
		cy1, cy2, y = -cy1, -cy2, -y
	}
	if shape.scale != 64 {
		cx1 = cx1.Mul(shape.scale)
		cx2 = cx2.Mul(shape.scale)
		cy1 = cy1.Mul(shape.scale)
		cy2 = cy2.Mul(shape.scale)
		x = x.Mul(shape.scale)
		y = y.Mul(shape.scale)
	}
	shape.segments = append(shape.segments,
		sfnt.Segment{
			Op: sfnt.SegmentOpCubeTo,
			Args: [3]fixed.Point26_6{
				{X: cx1, Y: cy1},
				{X: cx2, Y: cy2},
				{X: x, Y: y},
			},
		})
}

// Resets the shape segments. Be careful to not be holding the segments
// from [Shape.Segments]() when calling this (they may be overriden soon).
func (shape *Shape) Reset() {
	shape.segments = shape.segments[0:0]
	shape.hasLastDir = false
}

// A helper method to rasterize the current shape into an [*image.Alpha].
func (shape *Shape) Rasterize() (*image.Alpha, error) {
	return shape.RasterizeFract(0, 0)
}

// A helper method to rasterize the current shape displaced by the given
// fractional offset into an [*image.Alpha].
func (shape *Shape) RasterizeFract(offsetX, offsetY Fract) (*image.Alpha, error) {
	segments := shape.Segments()
	if len(segments) == 0 {
		return nil, nil
	}
	return Rasterize(segments, shape.rasterizer, offsetX, offsetY)
}

// A helper method to rasterize the current shape with the given
// colors. You could then export the result to a png file, e.g.:
//
//	file, _ := os.Create("my_ugly_shape.png")
//	_ = png.Encode(file, shape.Paint(color.White, color.Black))
//	// ...maybe even checking errors and closing the file ;)
func (shape *Shape) Paint(drawColor, backColor color.Color) *image.RGBA {
	segments := shape.Segments()
	if len(segments) == 0 {
		return nil
	}
	mask, err := Rasterize(segments, shape.rasterizer, 0, 0)
	if err != nil {
		panic(err)
	} // default rasterizer doesn't return errors
	rgba := image.NewRGBA(mask.Rect)

	r, g, b, a := drawColor.RGBA()
	nrgba := color.NRGBA64{R: uint16(r), G: uint16(g), B: uint16(b), A: 0}
	for y := mask.Rect.Min.Y; y < mask.Rect.Max.Y; y++ {
		for x := mask.Rect.Min.X; x < mask.Rect.Max.X; x++ {
			nrgba.A = uint16((a * uint32(mask.AlphaAt(x, y).A)) / 255)
			rgba.Set(x, y, mixColors(nrgba, backColor))
		}
	}
	return rgba
}

// Helper method for [Shape.Paint](). The same as mixOverFunc
// on the generic version of etxt (see etxt/ebiten_no.go).
func mixColors(draw color.Color, back color.Color) color.Color {
	dr, dg, db, da := draw.RGBA()
	if da == 0xFFFF {
		return draw
	}
	if da == 0 {
		return back
	}
	br, bg, bb, ba := back.RGBA()
	if ba == 0 {
		return draw
	}
	return color.RGBA64{
		R: uint16N((dr*0xFFFF + br*(0xFFFF-da)) / 0xFFFF),
		G: uint16N((dg*0xFFFF + bg*(0xFFFF-da)) / 0xFFFF),
		B: uint16N((db*0xFFFF + bb*(0xFFFF-da)) / 0xFFFF),
		A: uint16N((da*0xFFFF + ba*(0xFFFF-da)) / 0xFFFF),
	}
}

// clamping from uint32 to uint16 values
func uint16N(value uint32) uint16 {
	if value > 65535 {
		return 65535
	}
	return uint16(value)
}
