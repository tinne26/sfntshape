package sfntshape

import "golang.org/x/image/math/fixed"

// Helper functions adapted from etxt/fract/unit.go

func fixedFloor(value fixed.Int26_6) fixed.Int26_6 {
	return value & ^0x3F
}

func fixedCeil(value fixed.Int26_6) fixed.Int26_6 {
	return fixedFloor(value + 0x3F)
}

func fixedFract(value fixed.Int26_6) fixed.Int26_6 {
	return value & 0x3F
}

func fixedToIntFloor(value fixed.Int26_6) int {
	return int(value) >> 6
}

func fixedToF32(value fixed.Int26_6) float32 {
	return float32(value)/64.0
}
