package mapper

import (
	"fmt"
	"math"
)

const (
	minNormalFloat64 = 2.2250738585072014e-308 //2^(-1022)
	expOverflow      = 7.094361393031e+02      // The value at which math.Exp overflows
)

type LogarithmicMapper struct {
	relativeAccuracy float64
	gamma            float64
	multiplier       float64
	minValue         float64
	maxValue         float64
}

func NewLogarithmicMapper(relativeAccuracy float64) (*LogarithmicMapper, error) {
	if relativeAccuracy <= 0 || relativeAccuracy >= 1 {
		return nil, fmt.Errorf("relative accuracy must be between 0 and 1")
	}
	gamma := (1 + relativeAccuracy) / (1 - relativeAccuracy)
	multiplier := 1 / math.Log(gamma)
	return &LogarithmicMapper{
		relativeAccuracy: relativeAccuracy,
		gamma:            gamma,
		multiplier:       multiplier,
		minValue: max(
			math.Exp(math.MinInt32/multiplier+1),
			minNormalFloat64*gamma,
		),
		maxValue: min(
			math.Exp(math.MaxInt32/multiplier-1),
			math.Exp(expOverflow)/(2*gamma)*(gamma+1),
		),
	}, nil
}

func IsMergeable(left *LogarithmicMapper, right *LogarithmicMapper) bool {
	if left.relativeAccuracy == right.relativeAccuracy {
		return true
	}
	return false
}

func (m *LogarithmicMapper) MinValue() float64 {
	return m.minValue
}

func (m *LogarithmicMapper) MaxValue() float64 {
	return m.maxValue
}

func (m *LogarithmicMapper) Index(value float64) int {
	index := math.Log(value) * m.multiplier
	if index >= 0 {
		return int(index)
	}
	return int(index) - 1
}

func (m *LogarithmicMapper) Value(index int) float64 {
	lowerBound := math.Exp(float64(index) / m.multiplier)
	relativeAccuracy := 1 - 2/(1+m.gamma)
	return lowerBound * (1 + relativeAccuracy)
}
