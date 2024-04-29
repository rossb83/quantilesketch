package quantilesketch

import (
	"fmt"
	"math"
	"quantilesketch/mapper"
	"quantilesketch/storage"
)

type Mapper interface {
	Index(value float64) int
	Value(index int) float64
	MinValue() float64
	MaxValue() float64
}

type Storage interface {
	Size() float64
	KeyAtRank(rank float64) (int, error)
	RecordValue(index int, count float64)
}

type QuantileSketch struct {
	mapper         Mapper
	positiveValues Storage
	negativeValues Storage
	zeroCount      float64
}

func New(relativeAccuracy float64) (*QuantileSketch, error) {
	logarithmicMapper, err := mapper.NewLogarithmicMapper(relativeAccuracy)
	if err != nil {
		return nil, err
	}
	return &QuantileSketch{
		mapper:         logarithmicMapper,
		positiveValues: storage.NewBufferedPaginatedStorage(),
		negativeValues: storage.NewBufferedPaginatedStorage(),
	}, nil
}

func Merge(left *QuantileSketch, right *QuantileSketch) (*QuantileSketch, error) {
	if err := isMappersMergeable(left.mapper, right.mapper); err != nil {
		return nil, err
	}
	positiveStorage, err := mergeStorages(left.positiveValues, right.positiveValues)
	if err != nil {
		return nil, err
	}
	negativeStorage, err := mergeStorages(left.negativeValues, right.negativeValues)
	if err != nil {
		return nil, err
	}
	left.positiveValues = positiveStorage
	left.negativeValues = negativeStorage
	left.zeroCount += right.zeroCount
	return left, nil
}

func isMappersMergeable(left Mapper, right Mapper) error {
	switch leftMapper := left.(type) {
	case *mapper.LogarithmicMapper:
		rightMapper, ok := right.(*mapper.LogarithmicMapper)
		if !ok {
			return fmt.Errorf("mapper types must match to be merged")
		}
		if !mapper.IsMergeable(leftMapper, rightMapper) {
			return fmt.Errorf("mappers are not mergeable")
		}
		return nil
	default:
		return fmt.Errorf("mapper type not supported")
	}
}

func mergeStorages(left Storage, right Storage) (Storage, error) {
	storage1, ok := left.(*storage.BufferedPaginatedStorage)
	if !ok {
		return nil, fmt.Errorf("storage type not supported")
	}
	storage2, ok := right.(*storage.BufferedPaginatedStorage)
	if !ok {
		return nil, fmt.Errorf("storage type not supported")
	}
	return storage.Merge(storage1, storage2)
}

func (s *QuantileSketch) RecordValue(value float64, count float64) error {
	if count < 0 {
		return fmt.Errorf("count must be non-negative")
	}
	if value > s.mapper.MaxValue() || value < -s.mapper.MaxValue() || math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("value is not within proper range")
	}
	if value > s.mapper.MinValue() {
		index := s.mapper.Index(value)
		s.positiveValues.RecordValue(index, count)
		return nil
	}
	if value < -s.mapper.MinValue() {
		s.negativeValues.RecordValue(s.mapper.Index(-value), count)
		return nil
	}
	s.zeroCount += count
	return nil
}

func (s *QuantileSketch) GetValueAtQuantile(quantile float64) (float64, error) {
	if quantile < 0 || quantile > 1 {
		return 0, fmt.Errorf("quantile must be between 0 and 1")
	}
	count := s.zeroCount + s.positiveValues.Size() + s.negativeValues.Size()
	if count == 0 {
		return 0, fmt.Errorf("empty sketch")
	}
	rank := quantile * (count - 1)
	negativeValuesCount := s.negativeValues.Size()
	if rank < negativeValuesCount {
		key, err := s.negativeValues.KeyAtRank(negativeValuesCount - 1 - rank)
		if err != nil {
			return 0, err
		}
		return -s.mapper.Value(key), nil
	}
	if rank < s.zeroCount+negativeValuesCount {
		return 0, nil
	}
	key, err := s.positiveValues.KeyAtRank(rank - s.zeroCount - negativeValuesCount)
	if err != nil {
		return 0, err
	}
	return s.mapper.Value(key), nil
}
