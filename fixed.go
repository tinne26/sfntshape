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

func fixedToF64(value fixed.Int26_6) float64 {
	return float64(value)/64.0
}

// Rounds up in case of ties. Doesn't check
// for overflows, NaNs, infinites, etc.
func fixedFromFloat64(value float64) fixed.Int26_6 {
	fractApprox := Fract(value*64)
	fp64Approx := fixedToF64(fractApprox)
	if fp64Approx == value { return fractApprox }
	if fp64Approx > value {
		fractApprox -= 1
		fp64Approx = fixedToF64(fractApprox)
	}

	if value - fp64Approx >= 0.0078125 { fractApprox += 1 }
	return fractApprox
}
