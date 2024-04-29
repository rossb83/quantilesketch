# quantilesketch

Quantile sketch is datastructure that provides
* small in-memory footprint for ingested data streams
* fast merges of multiple sketches
* error for percentile computation guaranteed to be below a user-defined threshold
* fast percentile computation

Running the example in `main.go` creates a `100` quantile sketches with an error below `0.001` each which ingest `1000` values `10` times from a normal distribution. The sketches are then all merged together to produce the output below.
```
p50 -> -0.00017495121686582989
p90 -> 1.280178578089975
p95 -> 1.6372199718286418
p99 -> 2.323326002985621
```

A sketch can record values and compute percentiles. Sketches themselves can be merged.
```
type Sketch interface {
	RecordValue(value float64, count float64) error
	GetValueAtQuantile(quantile float64) (float64, error)
}
```

## mapper
Mapper is responsible for mapping values to indexes that are used in the storage and reverse-mapping indexes back to values to return data to the users. There are 3 flavors of mappers: `logarithmic`, `linear`, `cubic`. Currently only `logarithmic` is implemented first as that is what DataDog uses as their default, but I plan to implement the other two for experimentation.
```
type Mapper interface {
	Index(value float64) int
	Value(index int) float64
	MinValue() float64
	MaxValue() float64
}
```

## storage
Storage is responsible for storing the indexes and their corresponding counts such that data can be merged and percentiles can be computed. There are 5 flavors of storages: `sparse`, `dense`, `collapsing lowest dense`, `collapsing highest dense`, and `bucketed pagination`. Currently only `bucketed pagination` is implemented first as that is what DataDog uses as their default, but I plan to implement the other 4 for experimentation.
```
type Storage interface {
	Size() float64
	KeyAtRank(rank float64) (int, error)
	RecordValue(index int, count float64)
}
```
