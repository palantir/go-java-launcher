package launchlib

import "math"

type RAMPercent interface {
	RAMPercent(limit int) (float64, error)
}

type StaticRAMPercent struct {
	percent float64
}

func (s StaticRAMPercent) RAMPercent(_ int) (float64, error) {
	return s.percent, nil
}

const (
	lowerBound = 75
	upperBound = 95
	growthRate = 1
	midpoint   = 50
	sharpness  = 1
)

var scalingFunc = genlog(lowerBound, upperBound, growthRate, midpoint, sharpness)

type ScalingRAMPercent struct{}

func (s ScalingRAMPercent) RAMPercent(limit int) (float64, error) {
	return scalingFunc(float64(limit)), nil
}

func genlog(min float64, max float64, growthRate float64, midpoint float64, v float64) func(float64) float64 {
	return func(in float64) float64 {
		// https://en.wikipedia.org/wiki/Generalised_logistic_function#Definition
		return min + (max-min)/(math.Pow(1+math.Pow(math.E, -1*growthRate*(in-midpoint)), 1/v))
	}
}