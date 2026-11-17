package domain

// Series is one measured quantity: a kind applied to a name (a host, a
// container, a sensor key, a disk). ID is assigned by the store; zero means the
// series has not been persisted yet.
type Series struct {
	ID   int64
	Kind Kind
	Name string
}

// Sample is one observation waiting to be bucketed. Samples live in memory and
// never touch the disk.
type Sample struct {
	Kind  Kind
	Name  string
	Value float64
}
