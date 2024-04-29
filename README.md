# quantilesketch

Computing percentiles for datastreams, are problematic at scale. For users emitting this data, libraries like tally (and underlying metrics engines like m3) require users to define fixed buckets for for their data. This burden on users gives the metrics engine the ability to compute exact percentiles with a fairly cheap storage and query layer.
```
type Scope interface {
	Histogram(name string, buckets Buckets) Histogram
}
```
Just a brief example of why this is a difficult problem. Suppose you have a stream of data with values `{1, 2, 3, 4, 5, 6, 7, 8, 9, 11}` however this data is emitted across 3 different zones such that zone 1 emits `{1, 2, 3}`, zone 2 emits `{4, 5, 6, 7, 8}`, and zone 3 emits `{9, 11}`. To compute the `p50` (or the mean) of this raw data at would require all the data to sit together in-memory at query-time which simply does not scale for metrics systems.

A quantile sketch can help solve this with the caveat that the percentiles will no longer be 100% accurate. A quantile sketch aims to be small in-memory (especially compared to the size of the original data stream) to solve the storage problem, and also comes with a fast merge algorithm to solve the zonal partition problem. Users can emit and compute using the below API, without needing to define fixed buckets for their data.
```
type Sketch interface {
	RecordValue(value float64, count float64) error
	GetValueAtQuantile(quantile float64) (float64, error)
}
```

DataDog have done some work to build a variant that guarantees the accuracy for the `GetValueAtQuantile(...)` to be below a user-defined threshold, for example `0.001`. Much of what sits in this repository is a simplified version of what DataDog has built, only pulling out what is needed. This has gained a lot of industry wide recognition and is now also used in Apache Pinot.

There are two underlying datastructures for this variant: mapper and storage.

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


Running the example in `main.go` creates a `100` quantile sketches with an accuracy below `0.001` each which ingest `1000` values `10` times from a normal distribution. The sketches are then all merged together to produce the output below.
```
p50 -> -0.00017495121686582989
p90 -> 1.280178578089975
p95 -> 1.6372199718286418
p99 -> 2.323326002985621
```
