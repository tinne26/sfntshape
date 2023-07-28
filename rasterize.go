package sfntshape

import "image"
import "image/draw"

import "golang.org/x/image/font/sfnt"
import "golang.org/x/image/math/fixed"
import "golang.org/x/image/vector"

// Rasterize an outline into a single-channel image.
func Rasterize(outline sfnt.Segments, rasterizer *vector.Rasterizer, originX, originY Fract) (*image.Alpha, error) {
	// return nil if the outline don't include lines or curves
	for _, segment := range outline {
		if segment.Op == sfnt.SegmentOpMoveTo { continue }
		return etxtLikeRasterize(outline, rasterizer, originX, originY)
	}
	return nil, nil // nothing to draw
}

// Code adapted from etxt's mask.DefaultRasterizer.
func etxtLikeRasterize(outline sfnt.Segments, rasterizer *vector.Rasterizer, originX, originY Fract) (*image.Alpha, error) {
	// get outline bounds
	bounds := outline.Bounds()

	// prepare rasterizer
	width, height, normOffsetX, normOffsetY, rectOffset := figureOutBounds(bounds, originX, originY)
	rasterizer.Reset(width, height)
	rasterizer.DrawOp = draw.Src

	// allocate glyph mask
	mask := image.NewAlpha(rasterizer.Bounds())

	// process outline
	processOutline(rasterizer, outline, normOffsetX, normOffsetY)

	// since the source texture is a uniform (an image that returns the same
	// color for any coordinate), the value of the point at which we want to
	// start sampling the texture (the fourth parameter) is unimportant.
	rasterizer.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})

	// translate the mask to its final position
	mask.Rect = mask.Rect.Add(rectOffset)
	return mask, nil
}

// (copied/adapted from etxt v0.0.9 mask/helper_funcs.go)
// 
// Given the glyph bounds and an origin position indicating the subpixel
// positioning (only lowest bits will be taken into account), it returns
// the bounding integer width and heights, the normalization offset to be
// applied to keep the coordinates in the positive plane, and the final
// offset to be applied on the final mask to align its bounds to the glyph
// origin. This is used in Rasterize() functions.
func figureOutBounds(bounds fixed.Rectangle26_6, originX, originY Fract) (int, int, Fract, Fract, image.Point) {
	floorMinX := fixedFloor(bounds.Min.X)
	floorMinY := fixedFloor(bounds.Min.Y)
	var maskCorrection image.Point
	maskCorrection.X = fixedToIntFloor(floorMinX)
	maskCorrection.Y = fixedToIntFloor(floorMinY)

	normOffsetX := -floorMinX + fixedFract(originX)
	normOffsetY := -floorMinY + fixedFract(originY)
	width  := (bounds.Max.X + normOffsetX).Ceil()
	height := (bounds.Max.Y + normOffsetY).Ceil()
	return width, height, normOffsetX, normOffsetY, maskCorrection
}

// (copied/adapted from etxt v0.0.9 mask/rasterizer.go)
func processOutline(rasterizer *vector.Rasterizer, outline sfnt.Segments, offsetX, offsetY fixed.Int26_6) {
	for _, segment := range outline {
		switch segment.Op {
		case sfnt.SegmentOpMoveTo:
			rasterizer.MoveTo(
				fixedToF32(segment.Args[0].X + offsetX), fixedToF32(segment.Args[0].Y + offsetY),
			)
		case sfnt.SegmentOpLineTo:
			rasterizer.LineTo(
				fixedToF32(segment.Args[0].X + offsetX), fixedToF32(segment.Args[0].Y + offsetY),
			)
		case sfnt.SegmentOpQuadTo:
			rasterizer.QuadTo(
				fixedToF32(segment.Args[0].X + offsetX), fixedToF32(segment.Args[0].Y + offsetY),
				fixedToF32(segment.Args[1].X + offsetX), fixedToF32(segment.Args[1].Y + offsetY),
			)
		case sfnt.SegmentOpCubeTo:
			rasterizer.CubeTo(
				fixedToF32(segment.Args[0].X + offsetX), fixedToF32(segment.Args[0].Y + offsetY),
				fixedToF32(segment.Args[1].X + offsetX), fixedToF32(segment.Args[1].Y + offsetY),
				fixedToF32(segment.Args[2].X + offsetX), fixedToF32(segment.Args[2].Y + offsetY),
			)
		default:
			panic("unexpected segment.Op case")
		}
	}
}
