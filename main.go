package main

import (
	"fmt"
	"math/rand"
	"quantilesketch/quantilesketch"
)

type Sketch interface {
	RecordValue(value float64, count float64) error
	GetValueAtQuantile(quantile float64) (float64, error)
}

func CreateSketches(numSketches int, relativeAccuracy float64) ([]Sketch, error) {
	var sketches []Sketch
	for i := 0; i < numSketches; i++ {
		sketch, err := quantilesketch.New(relativeAccuracy)
		if err != nil {
			return nil, err
		}
		sketches = append(sketches, sketch)
	}
	return sketches, nil
}

func RecordValues(numValues int, countPerValue int, sketches []Sketch) ([]Sketch, error) {
	for _, sketch := range sketches {
		for i := 0; i < numValues; i++ {
			if err := sketch.RecordValue(rand.NormFloat64(), float64(countPerValue)); err != nil {
				return nil, err
			}
		}
	}
	return sketches, nil
}

func MergeSketches(sketches ...Sketch) (Sketch, error) {
	if len(sketches) == 0 {
		return nil, fmt.Errorf("must have at least 1 sketch")
	}
	merged := sketches[0]
	for _, right := range sketches[1:] {
		switch leftSketch := merged.(type) {
		case *quantilesketch.QuantileSketch:
			rightSketch, ok := right.(*quantilesketch.QuantileSketch)
			if !ok {
				return nil, fmt.Errorf("sketch types must match")
			}
			mergedSKetch, err := quantilesketch.Merge(leftSketch, rightSketch)
			if err != nil {
				return nil, err
			}
			merged = mergedSKetch
		default:
			return nil, fmt.Errorf("type not supported")
		}
	}
	return merged, nil
}

func GetValuesAtQuantiles(sketch Sketch, quantiles ...float64) ([]float64, error) {
	var values []float64
	for _, quantile := range quantiles {
		value, err := sketch.GetValueAtQuantile(quantile)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func main() {
	relativeAccuracy := 0.001
	numSketches := 100

	sketches, err := CreateSketches(numSketches, relativeAccuracy)
	if err != nil {
		panic(err)
	}

	numValues := 1000
	countPerValue := 10

	sketches, err = RecordValues(numValues, countPerValue, sketches)
	if err != nil {
		panic(err)
	}

	merged, err := MergeSketches(sketches...)
	if err != nil {
		panic(err)
	}

	quantiles := []float64{0.5, 0.9, 0.95, 0.99}

	values, err := GetValuesAtQuantiles(merged, quantiles...)
	if err != nil {
		panic(err)
	}
	fmt.Println(quantiles)
	fmt.Println(values)
}
