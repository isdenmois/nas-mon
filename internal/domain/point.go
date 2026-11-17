package domain

// Point is one aggregated bucket ready for storage. TS is a unix second aligned
// to the bucket's Resolution.
type Point struct {
	Kind Kind
	Name string
	TS   int64
	Avg  float64
	Min  float64
	Max  float64
	N    int64
}
